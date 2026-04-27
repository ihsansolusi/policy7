# Prompt: Policy7 — Verify & Update Spec for NATS Integration

**Context:** Policy7 sudah memiliki hybrid Redis+NATS design di Spec 01-architecture.md. Perlu diverifikasi bahwa implementation plans sesuai dengan hybrid messaging roadmap.

**Reference:**
- `docs/infra/HYBRID-MESSAGING-MODEL.md` — decision document
- `docs/infra/hybrid-messaging-plans/P-POL7-01-Verify-Hybrid-Model.md` — verification plan
- `docs/infra/hybrid-messaging-plans/P-LIB7-01-Lib7-Messaging-Go.md` — library plan
- `supported-apps/policy7/docs/specs/01-architecture.md` — current spec

---

## Tasks for Policy7 Session

### 1. Review Spec 01 Architecture (Hybrid Model Section)

Baca `docs/specs/01-architecture.md` bagian hybrid model:
- Cache layer dengan Redis ✅ (sudah ada)
- NATS untuk parameter change events ✅ (sudah ada)
- Cache invalidation via NATS ✅ (sudah ada)
- Health check via NATS request-reply ✅ (sudah ada)

Identifikasi:
- Apakah design sudah lengkap atau ada gap?
- Apakah NATS subjects sudah defined?
- Apakah JetStream needed untuk v1.0 atau cukup core NATS?

### 2. Review Plan 06 (Performance & Caching)

Baca `docs/plans/PLAN-OVERVIEW.md` → Plan 06:

```markdown
## Plan 06 — Performance & Caching
- [ ] Redis hot cache
- [ ] Parameter change pub/sub    ← ⚠️ Need NATS details
- [ ] Load testing
```

Identifikasi gaps antara Plan 06 dan hybrid model requirements.

### 3. Update Plan 06 with NATS Details

Tambahkan detail untuk NATS implementation:

```markdown
## Plan 06 — Performance & Caching

### Redis Hot Cache
- [ ] Implement parameter cache in Redis
- [ ] TTL-based expiration (e.g., 5 minutes for hot params)
- [ ] Cache warming on startup for org's active parameters

### NATS Event Streaming
- [ ] Add lib7-service-go/nats dependency
- [ ] Create NATS client
- [ ] Define event subjects:
      - `policy7.params.updated` — parameter changed
      - `policy7.params.created` — new parameter
      - `policy7.params.deleted` — parameter deleted
- [ ] Publish events on parameter changes (create/update/delete)
- [ ] Subscribe from other services (future: auth7, core7)

### Health Check via NATS
- [ ] Respond to `policy7.health` request-reply
- [ ] Include cache status, connection status in response

### Cache Invalidation
- [ ] On receiving `policy7.params.updated` event, invalidate local cache
- [ ] Use subject wildcard: `policy7.params.>` for subscribe
```

### 4. Update Spec 04 (Integration) with NATS Subjects

Tambahkan section jika belum ada:

```markdown
## NATS Integration

### Subjects

| Subject | Direction | Purpose |
|---------|-----------|---------|
| `policy7.params.created` | Publish | Notify all instances of new param |
| `policy7.params.updated` | Publish | Notify all instances of param change |
| `policy7.params.deleted` | Publish | Notify all instances of param deletion |
| `policy7.health` | Subscribe + Reply | Health check request-reply |

### JetStream Requirement

v1.0: Core NATS only (no JetStream persistence needed)
- Parameters are idempotent (can refetch from DB)
- Events are fire-and-forget for cache invalidation
- Audit trail is via PostgreSQL (parameter_history table)

Future (v1.1): Add JetStream if audit trail of events needed.
```

---

## Key Decisions to Confirm

1. **JetStream vs Core NATS:** Untuk v1.0, cukup core NATS atau perlu JetStream?
2. **Subject naming:** `policy7.params.updated` or `policy7.parameter.updated`?
3. **Cache strategy:** Sync invalidation (broadcast) atau async? Both?
4. **Multi-instance:** Jika 2 instances running, apakah keduanya perlu subscribe dan invalidate sendiri?

---

## Output

1. Confirmation bahwa Spec 01 hybrid model design sudah complete
2. Updated Plan 06 dengan NATS implementation details
3. Updated atau new Spec section untuk NATS subjects (if needed)
4. List decisions yang perlu dibuat + recommended choices
5. Any spec/plan changes → commit to policy7 repo
