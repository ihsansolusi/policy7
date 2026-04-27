# Policy7 — Plan Overview

> **Status**: Brainstorming → Planning
> **Target**: v1.0 Foundation

---

## Plan 01 — Foundation
- [x] Scaffold Go project (cmd/, internal/, domain/)
- [x] PostgreSQL schema (parameters, parameter_history)
- [x] Basic REST API (GET /v1/params/*)
- [x] Docker compose (postgres + redis)
- [x] Unit tests

## Plan 02 — Admin API
- [x] CRUD parameters (/admin/v1/params)
- [x] Versioning logic
- [x] Audit trail (parameter_history)
- [x] Effective date scheduling

## Plan 03 — Parameter Categories
- [x] Transaction limits (employee & customer)
- [x] Approval thresholds
- [x] Operational hours
- [x] Product access rules

## Plan 04 — Rates & Fees
- [x] Interest rates per product/tenor
- [x] Fee & tarif per product/channel
- [x] Regulatory thresholds (CTR/STR)

## Plan 05 — Integration
- [ ] Auth7 OPA integration (OPA query policy7)
- [ ] Core7 enterprise integration
- [ ] Workflow7 integration
- [ ] notif7 integration (regulatory alerts)

## Plan 06 — Performance & Caching with NATS

### Redis Hot Cache
- [ ] Implement parameter cache in Redis
- [ ] TTL-based expiration (5 minutes for hot params, 1 hour for rates)
- [ ] Cache warming on startup for org's active parameters
- [ ] Cache-aside pattern implementation
- [ ] Singleflight untuk cache stampede prevention
- [ ] Fallback to DB on cache failure

### NATS Event Streaming
- [ ] Add lib7-service-go/nats dependency (or nats.go)
- [ ] Create NATS client wrapper dengan connection pooling
- [ ] Define NATS subjects:
  - `policy7.params.created` — new parameter created
  - `policy7.params.updated` — parameter updated  
  - `policy7.params.deleted` — parameter deleted
  - `policy7.params.>` — Wildcard untuk subscribe semua events
- [ ] Publish events on parameter changes (async, non-blocking)
- [ ] Subscribe to own events untuk multi-instance cache coordination
- [ ] Event payload: `{event_id, event_type, org_id, data, timestamp}`

### Health Check via NATS (Request-Reply)
- [ ] Implement `policy7.health` request-reply handler
- [ ] Response: `{status, version, cache_status, db_status, timestamp}`
- [ ] Include di readiness probe untuk k8s

### Cache Invalidation Strategy
- [ ] On receive NATS event, invalidate Redis cache by pattern
- [ ] Pattern: `policy7:{org_id}:{category}:{name}:*`
- [ ] Multi-instance: setiap instance subscribe dan invalidate sendiri
- [ ] Track cache hit/miss metrics

### JetStream Decision
- [ ] **v1.0: Core NATS only** (no JetStream persistence)
  - Events are fire-and-forget untuk cache invalidation
  - Audit trail via PostgreSQL (parameter_history table)
  - Parameters are idempotent (can refetch dari DB)
- [ ] **v1.1: Evaluate JetStream** jika perlu event replay/audit trail

### Load Testing
- [ ] Benchmark: 1000 req/s read, 100 req/s write
- [ ] Test cache hit rate target: > 95%
- [ ] Test NATS event latency: < 10ms publish
- [ ] Test failover: DB down, cache-only mode

---

*Diperbarui: 2026-04-24*