# Policy7 — Roadmap

Status fitur policy7: yang **sudah diimplementasi** vs yang **masih backlog**.
Diperbarui 2026-06-26.

## ✅ Sudah diimplementasi

- **Consumer API `/v1`** — query parameter, effective resolution, two-limit validate,
  authorization-limit check, approval thresholds, operational hours, product access, rates,
  fees, regulatory check, contracts metadata.
- **Admin API `/admin/v1`** — CRUD parameter, history, bulk-import, DataTable query; CRUD
  kategori (data-driven `value_schema`).
- **Versioning + audit trail** — `version++` + `is_active`, `parameter_history` dengan
  `change_reason` / before-after.
- **Resolution / inheritance** — actor-context (`user→role→branch→global`) dan Option C
  (`BRANCH→BRANCH_TYPE→GLOBAL`) dengan `branch_scope` poller dari enterprise.
- **Two-limit pattern** — transaction + authorization limit dalam satu value.
- **Data-driven categories** — `value_schema` (JSON Schema) + `x-ui` (FE render) + `x-rules`
  (cross-field, divalidasi backend, 422). Category management UI di bos7-enterprise.
- **Workflow7 approval** — mutasi parameter & kategori lewat workflow (`policy-param-*-v1`)
  → callback `wf-*` (M2M + audit signature, lib7 v0.11.2 ActorEnvelope 7-field).
- **NATS events** — `policy7.params.created|updated|deleted` + cache-invalidation antar
  instance + health request-reply.
- **audit7 forwarding** — semua mutasi → audit7 (durable JetStream ingest `policy7`).
- **Security** — multi-tenant org scoping, JWT (JWKS) / X-Service-Key, RequireDelegatedOrM2M,
  token-exchange dari BFF.
- **DEF-generated schema** — rebaseline ke 4 tabel `20260615*` (UuidF/JsonbF, CHECK
  constraints, COALESCE-functional unique), seed demo/prod via golang-migrate.
- **Policy Management UI overhaul** (bos7-enterprise umbrella #556, CLOSED) — schema-driven
  page CRUD, versioning/rollback, bulk import, effective simulator.

## 🔭 Backlog / belum diimplementasi

### Retirement legacy compatibility paths
- `GET /v1/params/rates/:product` & `GET /v1/params/fees/:product` masih `compatibility-only`.
- **Blocker:** belum ada telemetry usage per-endpoint untuk keputusan retire yang aman.
  → tambahkan instrumentation pemakaian sebelum menghapus.

### Cross-stream dependency
- Canonical role identifier (`role_id` vs `role_code`) masih bergantung pada auth7.

### Follow-up Policy Management (non-blocking, di devroot #401)
- **#577** — SSE tracker race pada UI mutasi.
- **#587** — full-chain version history (riwayat lintas tahap workflow).
- **#588** — bulk-import error reporting per-row.

### Data / seed hygiene
- `validateCategoryContext` mewajibkan `product` untuk `transaction_limit`; seed saat ini
  meng-NULL-kan via SQL bypass → perlu rekonsiliasi owner.

### DEF / migration (low priority)
- Nama auto-index `org_id` berbeda dari deployment lama (`idx_parameters_org_id` vs
  `idx_parameters_org`) — kosmetik.
- Belum migrasi ke `FwRelation` (sengaja tanpa soft-delete `deleted_at/by` yang tak ada di
  produksi).

### Potensi v2
- gRPC untuk query low-latency.
- Conditional parameters (mis. limit berbeda saat hari libur).

> Konteks historis (Plan 07/12/13, planning issues) dipindah ke
> `_backup/policy7-cleanup-20260626/` di root devroot saat cleanup dokumentasi 2026-06-26.
