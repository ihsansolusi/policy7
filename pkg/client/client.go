package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type Client struct {
	baseURL    string
	serviceID  string
	apiKey     string
	httpClient *http.Client
}

// NewClient initializes a new Policy7 API client
func NewClient(baseURL, serviceID, apiKey string) *Client {
	return &Client{
		baseURL:    baseURL,
		serviceID:  serviceID,
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

func (c *Client) do(ctx context.Context, method, path, orgID string, body interface{}) (*http.Response, error) {
	var reqBody []byte
	if body != nil {
		reqBody, _ = json.Marshal(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Service-ID", c.serviceID)
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("X-Org-ID", orgID)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.httpClient.Do(req)
}

// ValidateTransactionLimit checks an amount against the effective transaction limit.
func (c *Client) ValidateTransactionLimit(ctx context.Context, orgID string, req ValidationRequest) (*ValidationResponse, error) {
	resp, err := c.do(ctx, http.MethodPost, "/v1/params/transaction_limit/validate", orgID, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var valResp ValidationResponse
	if err := json.NewDecoder(resp.Body).Decode(&valResp); err != nil {
		return nil, err
	}
	return &valResp, nil
}

// GetEffectiveParameter retrieves an effective parameter applying inheritance fallback.
func (c *Client) GetEffectiveParameter(ctx context.Context, orgID, category, name string) (*Parameter, error) {
	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/v1/params/%s/%s/effective", category, name), orgID, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var param Parameter
	if err := json.NewDecoder(resp.Body).Decode(&param); err != nil {
		return nil, err
	}
	return &param, nil
}

// CheckRegulatoryThreshold validates if an amount exceeds regulatory compliance limits (CTR/STR).
func (c *Client) CheckRegulatoryThreshold(ctx context.Context, orgID, regType string, req RegulatoryRequest) (*RegulatoryResponse, error) {
	resp, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/v1/params/regulatory/%s/check", regType), orgID, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var regResp RegulatoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
		return nil, err
	}
	return &regResp, nil
}

// CheckAuthorizationLimit validates if an approver role is authorized for the amount.
func (c *Client) CheckAuthorizationLimit(ctx context.Context, orgID string, req AuthorizationRequest) (*AuthorizationResponse, error) {
	resp, err := c.do(ctx, http.MethodPost, "/v1/params/authorization_limit/check", orgID, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var authResp AuthorizationResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return nil, err
	}
	return &authResp, nil
}
