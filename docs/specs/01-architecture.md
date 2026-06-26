# 01 — Architecture

## Layering (clean architecture)

```
cmd/server/main.go
   └─ wiring: config (env), pgx pool, redis, NATS, tracer, token maker, router
internal/api/          HTTP boundary (Gin)
   ├─ router.go            route table + middleware chain
   ├─ middleware/          auth (JWT / X-Service-Key), RequireDelegatedOrM2M,
   │                       RequireM2M, VerifyAuditSignatureFromEnv
   ├─ parameter_handler.go consumer /v1 handlers
   ├─ admin_handler.go     admin /admin/v1 param CRUD + wf-* callbacks
   ├─ category_handler.go  category CRUD + wf-* (category_wf.go)
   ├─ contract_handler.go  /v1/contracts/* (self-describing API metadata)
   ├─ audit_forward.go     forward mutasi → audit7
   └─ response.go          error envelope (code/message/http_status/retryable/trace_id)
internal/service/      business logic
   ├─ parameter.go        resolution engine (inheritance), query
   ├─ admin_parameter.go  create/update/delete + versioning + history
   ├─ nats.go             publish events + subscribe cache-invalidation/health
   └─ branchscope/poller.go  sinkron branch_scope dari enterprise
internal/store/        persistence
   ├─ query.sql.go, models.go, querier.go   sqlc-generated
   ├─ connection.go, db.go                   pgx pool
   ├─ branch_scope.go                        branch_scope queries
   └─ redis.go                               hot-cache
internal/domain/       entities + value_schema validator + errors
pkg/client/            Go SDK untuk konsumen
```

Konvensi Go (sama dengan service7-template): `const op = "pkg.Type.Method"`, error wrap
`fmt.Errorf("%s: %w", op, err)`, tracing via OpenTelemetry, semua query difilter `org_id`.

## Hybrid data store

| Lapis | Teknologi | Peran |
|---|---|---|
| Master | PostgreSQL 16 (pgx + sqlc) | sumber kebenaran; versioned parameters + history |
| Cache | Redis | hot params; key `policy7:{org}:{category}:{name}:{applies_to}:{applies_to_id}:{product}` |
| Events | NATS | publish perubahan + cache-invalidation antar-instance + health |

NATS subjects:

- Publish: `policy7.params.created` / `policy7.params.updated` / `policy7.params.deleted`
- Subscribe: `policy7.params.>` (invalidate cache di semua instance), `policy7.health` (request-reply)

Semua instance subscribe `policy7.params.>` sehingga cache konsisten di deployment
multi-instance. NATS opsional di dev (`NATS_URL` kosong → publish/subscribe no-op).

## Resolution engine (inheritance)

Dua mode fallback hidup di `service/parameter.go`:

1. **Actor-context fallback** (`GetParameterWithContext`): `user → role → branch → global`.
   Ambil parameter paling spesifik yang ada untuk `applies_to`.
2. **Option C** (`ResolveParameter`): `BRANCH → BRANCH_TYPE → GLOBAL`.
   - Tier 1: override per-branch (`applies_to=branch`, `applies_to_id=branchID`).
   - Tier 2: default per `branch_type` — `branch_type` diambil dari tabel `branch_scope`
     (proyeksi dari enterprise, lihat [04-integration](04-integration.md)).
   - Tier 3: nilai org-global (`applies_to=global`).

Selalu mengembalikan **versi aktif terbaru** (`is_active = true`). `effective_from` /
`effective_until` memungkinkan penjadwalan perubahan.

## Observability

zerolog terstruktur + OpenTelemetry tracing (`OTEL_EXPORTER_OTLP_ENDPOINT`) + Prometheus
metrics (`METRICS_PORT`). `GET /health` tanpa auth untuk liveness.
