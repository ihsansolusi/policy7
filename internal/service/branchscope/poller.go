package branchscope

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ihsansolusi/policy7/internal/store"
	"github.com/rs/zerolog"
)

// Config holds settings for the branch scope sync poller.
type Config struct {
	// SourceURL is the enterprise /v1/source-contracts/branch-scope endpoint.
	SourceURL     string
	ClientID      string
	ClientSecret  string
	TokenEndpoint string
	OrgID         uuid.UUID
	Interval      time.Duration // default 5m
	PerPage       int           // default 200
}

type branchScopeItem struct {
	BranchID       string `json:"branch_id"`
	OrgID          string `json:"org_id"`
	BranchType     string `json:"branch_type"`
	ParentBranchID string `json:"parent_branch_id,omitempty"`
	UpdatedAt      string `json:"updated_at"`
}

type pageResponse struct {
	Data []branchScopeItem `json:"data"`
	Meta pageMeta          `json:"meta"`
}

type pageMeta struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
	Total   int `json:"total"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// Poller syncs branch_scope rows from enterprise on a fixed interval.
type Poller struct {
	cfg         Config
	db          store.BranchScopeQuerier
	log         zerolog.Logger
	token       string
	tokenExpiry time.Time
	client      *http.Client
}

// NewPoller creates a Poller. Interval defaults to 5m, PerPage to 200.
func NewPoller(cfg Config, db store.BranchScopeQuerier, log zerolog.Logger) *Poller {
	if cfg.Interval == 0 {
		cfg.Interval = 5 * time.Minute
	}
	if cfg.PerPage == 0 {
		cfg.PerPage = 200
	}
	return &Poller{
		cfg:    cfg,
		db:     db,
		log:    log.With().Str("component", "branchscope_poller").Logger(),
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Start runs the poller loop; it syncs once immediately then on each interval tick.
// Blocks until ctx is cancelled.
func (p *Poller) Start(ctx context.Context) {
	p.log.Info().Str("source", p.cfg.SourceURL).Dur("interval", p.cfg.Interval).Msg("branchscope poller started")
	p.sync(ctx)
	ticker := time.NewTicker(p.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			p.log.Info().Msg("branchscope poller stopped")
			return
		case <-ticker.C:
			p.sync(ctx)
		}
	}
}

func (p *Poller) sync(ctx context.Context) {
	const op = "branchscope.Poller.sync"
	token, err := p.acquireToken(ctx)
	if err != nil {
		p.log.Error().Err(err).Msgf("%s: acquire token", op)
		return
	}
	page, upserted := 1, 0
	for {
		pg, err := p.fetchPage(ctx, token, page)
		if err != nil {
			p.log.Error().Err(err).Int("page", page).Msgf("%s: fetch", op)
			return
		}
		for _, item := range pg.Data {
			if err := p.upsertItem(ctx, item); err != nil {
				p.log.Error().Err(err).Str("branch_id", item.BranchID).Msgf("%s: upsert", op)
			} else {
				upserted++
			}
		}
		if len(pg.Data) == 0 || page*pg.Meta.PerPage >= pg.Meta.Total {
			break
		}
		page++
	}
	p.log.Info().Int("upserted", upserted).Msg("branchscope sync done")
}

func (p *Poller) acquireToken(ctx context.Context) (string, error) {
	if p.token != "" && time.Now().Before(p.tokenExpiry) {
		return p.token, nil
	}
	if p.cfg.TokenEndpoint == "" || p.cfg.ClientID == "" {
		return "", nil
	}
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {p.cfg.ClientID},
		"client_secret": {p.cfg.ClientSecret},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned %d", resp.StatusCode)
	}
	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", err
	}
	p.token = tr.AccessToken
	p.tokenExpiry = time.Now().Add(time.Duration(tr.ExpiresIn-30) * time.Second)
	return p.token, nil
}

func (p *Poller) fetchPage(ctx context.Context, token string, page int) (*pageResponse, error) {
	reqURL := fmt.Sprintf("%s?page=%d&per_page=%d", p.cfg.SourceURL, page, p.cfg.PerPage)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("source returned %d: %s", resp.StatusCode, body)
	}
	var pg pageResponse
	if err := json.NewDecoder(resp.Body).Decode(&pg); err != nil {
		return nil, err
	}
	return &pg, nil
}

func (p *Poller) upsertItem(ctx context.Context, item branchScopeItem) error {
	branchID, err := uuid.Parse(item.BranchID)
	if err != nil {
		return fmt.Errorf("parse branch_id %q: %w", item.BranchID, err)
	}
	updatedAt, err := time.Parse(time.RFC3339, item.UpdatedAt)
	if err != nil {
		updatedAt = time.Now()
	}
	arg := store.UpsertBranchScopeParams{
		BranchID:   branchID,
		OrgID:      item.OrgID,
		BranchType: item.BranchType,
		UpdatedAt:  updatedAt,
	}
	if item.ParentBranchID != "" {
		if parentID, err := uuid.Parse(item.ParentBranchID); err == nil {
			arg.ParentBranchID = &parentID
		}
	}
	return p.db.UpsertBranchScope(ctx, arg)
}
