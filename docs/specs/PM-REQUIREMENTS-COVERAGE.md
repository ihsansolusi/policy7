# Policy7 — NATS Integration: PM Requirements Coverage

> **Date**: 2026-04-27  
> **Reference**: PROMPT-P-POL7-01 — Policy7 Verify & Update Spec for NATS Integration  
> **Status**: ✅ ALL REQUIREMENTS COVERED

---

## Executive Summary

Semua requirements dari PM Core7 (PROMPT-P-POL7-01) telah ditambahkan ke specs dan plans Policy7. Berikut coverage matrix:

| PM Requirement | Spec/Plan Location | Status |
|----------------|-------------------|--------|
| NATS subjects defined | Spec 04 Section 6.1 | ✅ Complete |
| Health check via NATS request-reply | Spec 04 Section 6.4 | ✅ Complete |
| JetStream vs Core NATS decision | Spec 04 Section 6.5 | ✅ Documented |
| Plan 06 NATS implementation details | PLAN-OVERVIEW.md Plan 06 | ✅ Updated |
| Cache invalidation via NATS | Spec 04 Section 6.3 | ✅ Already existed |
| Multi-instance coordination | Spec 04 Section 6.3 + 6.5 | ✅ Documented |

---

## Detailed Coverage

### 1. ✅ NATS Subjects Defined

**Location**: `specs/04-integration.md` Section 6.1

```markdown
| Event Topic | Publisher | Consumer | Purpose |
|-------------|-----------|----------|---------|
| `policy7.params.created` | policy7 | auth7, core7 | Cache invalidation |
| `policy7.params.updated` | policy7 | auth7, core7 | Cache invalidation |
| `policy7.params.deleted` | policy7 | auth7, core7 | Cache invalidation |
| `policy7.regulatory.threshold_exceeded` | core7 | notif7 | CTR/STR alerts |
| `policy7.transaction.requires_authorization` | core7 | workflow7 | Approval task |
| `policy7.transaction.auto_authorized` | core7 | audit | Audit logging |
```

**Key Design Decisions:**
- Subject naming: `policy7.{entity}.{action}` pattern
- Wildcard support: `policy7.params.>` untuk subscribe semua parameter events
- Consistent dengan NATS best practices

---

### 2. ✅ Health Check via NATS Request-Reply

**Location**: `specs/04-integration.md` Section 6.4

**Subject**: `policy7.health`

**Features Documented:**
- Request structure dengan `check_type: "full" | "ping"`
- Response structure dengan detailed health checks (DB, cache, NATS)
- Kubernetes readiness probe integration example
- Go implementation code

**Example Response:**
```json
{
  "request_id": "uuid",
  "status": "healthy",
  "version": "1.0.0",
  "checks": {
    "database": {"status": "healthy", "latency_ms": 5},
    "cache": {"status": "healthy", "hit_rate": 0.94},
    "nats": {"status": "healthy", "connected": true}
  }
}
```

---

### 3. ✅ JetStream Decision

**Location**: `specs/04-integration.md` Section 6.5

**Decision: Core NATS Only untuk v1.0**

| Aspect | v1.0 (Core NATS) | v1.1 (JetStream - Future) |
|--------|------------------|---------------------------|
| Persistence | Fire-and-forget | Durable streams |
| Use Case | Cache invalidation | Event audit trail |
| Complexity | Low | Medium |
| Resource Usage | Minimal | Higher (disk) |

**Rationale:**
1. Cache invalidation events adalah ephemeral
2. Parameters are idempotent (can refetch dari PostgreSQL)
3. Audit trail sudah covered oleh `parameter_history` table
4. Simplicity untuk faster development

**Migration Path Documented:**
- v1.0: Core NATS dengan `nc.Publish()` dan `nc.Subscribe()`
- v1.1: Can add JetStream stream "POLICY7_EVENTS" tanpa breaking changes

---

### 4. ✅ Plan 06 Updated dengan NATS Details

**Location**: `plans/PLAN-OVERVIEW.md` Plan 06

**Added Details:**

```markdown
## Plan 06 — Performance & Caching with NATS

### NATS Event Streaming
- [ ] Add lib7-service-go/nats dependency
- [ ] Create NATS client wrapper dengan connection pooling
- [ ] Define NATS subjects:
  - `policy7.params.created`
  - `policy7.params.updated`
  - `policy7.params.deleted`
- [ ] Publish events on parameter changes (async, non-blocking)
- [ ] Subscribe to own events untuk multi-instance cache coordination

### Health Check via NATS (Request-Reply)
- [ ] Implement `policy7.health` request-reply handler
- [ ] Response: `{status, version, cache_status, db_status, timestamp}`
- [ ] Include di readiness probe untuk k8s

### JetStream Decision
- [ ] **v1.0: Core NATS only** (no JetStream persistence)
- [ ] **v1.1: Evaluate JetStream** jika perlu event replay/audit trail
```

---

### 5. ✅ Cache Invalidation via NATS

**Location**: `specs/04-integration.md` Section 6.3

Already existed dan enhanced dengan:
- Event payload structure
- Cache key pattern building
- Multi-instance coordination explanation
- Error handling

---

### 6. ✅ Multi-Instance Coordination

**Location**: `specs/04-integration.md` Section 6.3 dan 6.5

**Strategy:**
- Setiap Policy7 instance subscribe ke `policy7.params.>`
- When event received, invalidate own Redis cache
- No master/slave — all instances equal
- Simple dan scalable

**Code Example:**
```go
// Setiap instance subscribe dan invalidate sendiri
nc.Subscribe("policy7.params.>", func(msg *nats.Msg) {
    invalidateCache(msg.Data)  // Invalidate local Redis
})
```

---

## Files Updated

| File | Changes |
|------|---------|
| `specs/04-integration.md` | Added Section 6.4 (Health Check), Section 6.5 (JetStream Decision) |
| `plans/PLAN-OVERVIEW.md` | Enhanced Plan 06 dengan NATS implementation details |
| `CLAUDE.md` | Updated status dan added PM requirements coverage table |

---

## Verification Checklist

- [x] NATS subjects menggunakan naming convention yang konsisten
- [x] Health check request-reply fully documented dengan code examples
- [x] JetStream decision explicit: Core NATS untuk v1.0
- [x] Plan 06 includes actionable implementation checklist
- [x] Multi-instance cache coordination strategy documented
- [x] All sections have code examples dalam Go
- [x] Kubernetes integration (readiness probe) included

---

## Conclusion

**All requirements dari PROMPT-P-POL7-01 telah fully covered.**

Specs dan plans sekarang memiliki:
1. ✅ Detailed NATS integration specification
2. ✅ Complete implementation plan untuk developers
3. ✅ Clear architectural decisions (Core NATS vs JetStream)
4. ✅ Production-ready health check mechanism
5. ✅ Scalable multi-instance coordination strategy

**Status: READY FOR IMPLEMENTATION** 🚀

---

*Coverage verification completed: 2026-04-27*
