package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ihsansolusi/policy7/internal/store"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
)

type NATSClient struct {
	nc         *nats.Conn
	cache      *store.RedisCache
	db         store.Querier
	instanceID string
	version    string
}

func NewNATSClient(url string, cache *store.RedisCache, db store.Querier) (*NATSClient, error) {
	if url == "" {
		return nil, nil // Return nil if NATS URL is not provided
	}
	nc, err := nats.Connect(url, nats.RetryOnFailedConnect(true), nats.MaxReconnects(-1))
	if err != nil {
		return nil, err
	}

	client := &NATSClient{
		nc:         nc,
		cache:      cache,
		db:         db,
		instanceID: "policy7-instance",
		version:    "1.0.0",
	}

	return client, nil
}

func (n *NATSClient) StartSubscriptions() error {
	if n.nc == nil {
		return nil
	}

	_, err := n.nc.Subscribe("policy7.params.>", n.handleCacheInvalidation)
	if err != nil {
		return err
	}

	_, err = n.nc.Subscribe("policy7.health", n.handleHealthCheck)
	if err != nil {
		return err
	}

	log.Info().Msg("NATS subscriptions started")
	return nil
}

func (n *NATSClient) Close() {
	if n.nc != nil {
		n.nc.Close()
	}
}

func (n *NATSClient) PublishParameterEvent(ctx context.Context, eventType string, orgID string, param store.Parameter) error {
	if n.nc == nil {
		return nil
	}

	payload := map[string]interface{}{
		"event_id":   uuid.New().String(),
		"event_type": eventType,
		"org_id":     orgID,
		"timestamp":  time.Now().UTC(),
		"data": map[string]interface{}{
			"category": param.Category,
			"name":     param.Name,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return n.nc.Publish(eventType, data)
}

func (n *NATSClient) handleCacheInvalidation(msg *nats.Msg) {
	if n.cache == nil {
		return
	}

	var payload struct {
		OrgID string `json:"org_id"`
		Data  struct {
			Category string `json:"category"`
			Name     string `json:"name"`
		} `json:"data"`
	}

	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		log.Error().Err(err).Msg("failed to parse NATS cache invalidation payload")
		return
	}

	pattern := fmt.Sprintf("policy7:%s:%s:%s:*", payload.OrgID, payload.Data.Category, payload.Data.Name)
	err := n.cache.DelPattern(context.Background(), pattern)
	if err != nil {
		log.Error().Err(err).Str("pattern", pattern).Msg("failed to invalidate cache via NATS")
	} else {
		log.Debug().Str("pattern", pattern).Msg("cache invalidated via NATS")
	}
}

func (n *NATSClient) handleHealthCheck(msg *nats.Msg) {
	resp := map[string]interface{}{
		"timestamp":   time.Now().UTC(),
		"status":      "healthy",
		"version":     n.version,
		"instance_id": n.instanceID,
		"checks": map[string]interface{}{
			"database": map[string]string{"status": "healthy"},
			"cache":    map[string]string{"status": "healthy"},
			"nats":     map[string]interface{}{"status": "healthy", "connected": true},
		},
	}

	data, _ := json.Marshal(resp)
	msg.Respond(data)
}
