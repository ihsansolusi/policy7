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

### API usage / deprecation candidates

Review lintas-repo (2026-06-26) menemukan **~separuh surface tidak punya caller in-tree**.
Consumer runtime nyata hanya **bos7-enterprise BFF** (reads `/admin/v1` + `/v1/.../effective`
+ bulk-import) dan **workflow7** (mutasi via `wf-*`). auth7 / auth7-ui / core7-service-* /
Go SDK `pkg/client` **tidak** memanggil policy7 via HTTP.

✅ **Telemetry terpasang** (2026-06-26): counter `policy7_endpoint_usage_total{route, caller}`
di `/metrics` (`internal/api/usage_metrics.go`, middleware `trackUsage`) mencatat siapa
(M2M `client_id` / delegated `act.sub` / `system` / `user`) yang memukul tiap
deprecation-candidate. **Next:** amati di lingkungan nyata → hapus route+handler yang 0 caller.

Endpoint yang di-track (kandidat retire):

| Grup | Endpoint | Catatan |
|---|---|---|
| legacy | `GET /v1/params/rates/:product` · `/fees/:product` | compatibility-only |
| /v1 basic | `GET /v1/params/:category/:name` | tersuperseded `…/effective` |
| /v1 boundary | `GET /v1/params/operational-hours` · `/product-access` | dirancang utk ABAC auth7 — **auth7 tak pernah implement** (ABAC lokal Postgres) |
| /v1 boundary | `GET /v1/params/approval-thresholds` | tak ada caller |
| /v1 | `GET /v1/params/regulatory/:type` · `POST …/regulatory/:type/check` | tak ada caller (SDK wrap, 0 importer) |
| /v1 | `POST /v1/params/transaction_limit/validate` · `…/authorization_limit/check` | simulator pakai `/effective` |
| /v1 facade | `GET /v1/contracts/categories` · `/caller-context` · `/errors` | facade retired; BFF allowlist memblok `/v1/contracts/*` |
| admin direct | `POST /admin/v1/params` · `PUT/DELETE …/:id` · `POST …/query` | tersuperseded alur `wf-*` (approval) |
| admin direct | `POST /admin/v1/categories` · `PUT/DELETE …/:code` | tersuperseded alur category `wf-*` |

> **Tidak** di-track (aktif dipakai): `/admin/v1/params` GET·`:id`·`:id/history`, `bulk-import`,
> `/admin/v1/categories` GET·`:code`, semua `wf-*`, `/v1/params/:category/:name/effective`.

**Go SDK `pkg/client`** (membungkus 4 method: ValidateTransactionLimit, GetEffectiveParameter,
CheckRegulatoryThreshold, CheckAuthorizationLimit) punya **0 importer** di ekosistem →
kandidat hapus. Service yang dulu diharapkan memakainya tidak memanggil policy7 sama sekali.

### Rencana migrasi API → kontrak target (5 grup)

Mengikuti [docs/specs/06-api-grouping.md](specs/06-api-grouping.md). Bertahap, tiap fase
aman & independen. Telemetry `policy7_endpoint_usage_total` jadi safety-net saat memotong.

- **Fase 1 — Inquiry generik (additive, no breaking).** Tambah `POST /v1/params/resolve`
  (batch) + `GET /v1/params?category=…` (snapshot effective). Tidak menghapus apa pun.
  Update `pkg/client` (bila dipertahankan) → `Resolve` / `BatchResolve`. Output: konsumen
  punya satu cara generik untuk semua kategori (termasuk kategori baru buatan admin).
- **Fase 2 — Deprecate hardcoded `/v1`.** Tandai `operational-hours`, `product-access`,
  `approval-thresholds`, `rates/:product`, `fees/:product`, `regulatory/:type`,
  `GET /params/:category/:name` (basic), `authorization_limit/check`, `regulatory/:type/check`
  sebagai deprecated (header `Deprecation` + log WARN). Pantau `policy7_endpoint_usage_total`
  ~1–2 rilis. Pertahankan `…/effective` + `transaction_limit/validate` (decision helper).
- **Fase 3 — Retire direct admin CRUD.** Hapus `POST/PUT/DELETE /admin/v1/params` &
  `/categories` + `POST /params/query` setelah konfirmasi 0 caller (semua mutasi sudah lewat
  `wf-*`). Handler & route dihapus.
- **Fase 4 — Hapus `/v1` hardcoded + `/contracts/*` + handler mati.** Setelah Fase 2 dry,
  hapus route + handler (`GetRates`/`GetFees`/`GetOperationalHours`/…/`contract_handler.go`).
- **Fase 5 — Discovery + SDK.** Putuskan expose `value_schema` read di `/v1` (untuk consumer
  generik) atau cukup `/admin/v1`. Hapus `pkg/client` bila tetap 0 importer, atau align ke
  Grup 2.

Setiap fase: `make test` + build hijau, dan (Fase 3–4) verifikasi `policy7_endpoint_usage_total`
= 0 untuk route bersangkutan sebelum hapus.

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
