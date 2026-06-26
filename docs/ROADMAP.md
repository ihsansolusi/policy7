# Policy7 — Roadmap

Status fitur policy7: yang **sudah diimplementasi** vs yang **masih backlog**.
Diperbarui 2026-06-26.

## ✅ Sudah diimplementasi

- **Consumer API `/v1`** (Grup 2, generic) — effective resolution, batch resolve, category
  snapshot, two-limit validate (decision helper). *(Endpoint hardcoded-per-kategori lama sudah
  diretire — lihat migrasi API di bawah.)*
- **Admin API `/admin/v1`** (Grup 1) — parameter reads (list/detail/history) + bulk-import,
  kategori reads (data-driven `value_schema`); mutasi via `wf-*` approval. *(Direct CRUD sudah
  diretire.)*
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

## ✅ API migration → kontrak target (5 grup) — SELESAI (2026-06-26)

Review lintas-repo menemukan **~separuh surface tak punya caller in-tree**. Consumer runtime
nyata: **bos7-enterprise BFF** (reads `/admin/v1` + `/v1/.../effective` + bulk-import),
**workflow7** (mutasi via `wf-*`), dan **auth7** (#161 `dd7b5fb` — `operational_hours` via
generic `/v1/params/{category}/{name}/effective` + opacache + NATS, **bukan** endpoint
hardcoded). auth7-ui / core7-service-* / Go SDK tidak memanggil policy7 via HTTP. Karena
design-justified (endpoint hardcoded-per-kategori tak cocok dengan kategori data-driven) +
0 in-tree caller, retirement dieksekusi langsung. Detail: [06-api-grouping](specs/06-api-grouping.md).

- **Fase 1** ✅ inquiry generik (additive): `POST /v1/params/resolve` (batch) +
  `GET /v1/params?category=…` (snapshot) — `internal/api/inquiry_handler.go`.
- **Fase 2+4** ✅ hapus `/v1` hardcoded (basic get, operational-hours, product-access,
  approval-thresholds, rates, fees, regulatory(+check), authorization_limit/check) +
  `/contracts/*` (`contract_handler.go`) + telemetry transisional (`usage_metrics.go`).
- **Fase 3** ✅ hapus direct admin CRUD (`POST/PUT/DELETE /admin/v1/params` & `/categories`,
  `POST /params/query`). Mutasi hanya lewat `wf-*`; validasi tetap utuh (`validateScopeContext`
  + service gate); test validasi dipindah ke `WfCreate`.
- **Fase 5** ✅ hapus `pkg/client` Go SDK (0 importer; konsumen pakai REST `/v1` langsung,
  pola auth7 `internal/policy7client`).

**Surface akhir:** Grup 1 (`/admin/v1` reads + bulk-import + `wf-*`) · Grup 2 (`/effective`,
`resolve`, snapshot, `transaction_limit/validate`) · Grup 3 (`/admin/v1/categories` reads) ·
Grup 4 (NATS) · Grup 5 (`/health`, `/metrics`). Lihat [03-api](specs/03-api.md).

## ✅ Selesai belakangan (2026-06-26)
- **Grup 3 discovery** — `GET /v1/categories` (+ `/:code`) read-only `value_schema` untuk
  consumer/tooling generik (handler sama dgn `/admin/v1`, read-only). Tersedia di kedua plane.
- **#587 full-chain version history** (backend) — `GET /admin/v1/params/:id/history` balas
  rantai versi penuh: group semua row yang berbagi identity tuple `(org_id, category, name,
  applies_to, applies_to_id, product)`. Query `GetParameterHistoryByIdentity` (generated).
  Divalidasi vs DB nyata (EXPLAIN + 3-versi rollback: chain=3 vs per-id=1).
- **#588 bulk-import per-row errors** (backend) — `POST /admin/v1/params/bulk-import` best-effort
  per-row → `{summary, results:[{row,status,code,error|id}]}`; validasi per-row = jalur single/wf.
- **sqlc drift reconciled** — enum overrides di `sqlc.yaml`; codegen kini bersih/idempotent.

## 🔭 Backlog / belum diimplementasi

### UI surfacing (cross-repo — bos7-enterprise, bukan backend policy7)
- **#587** — UI `VersionHistory` konsumsi full-chain (#585).
- **#588** — tampilkan detail error per-row di surface bulk-import.

> #577 (SSE tracker race) = FE bos7-enterprise (`useWorkflowTracker`); devroot#401.

### Potensi v2
- gRPC untuk query low-latency.
- Conditional parameters (mis. limit berbeda saat hari libur).

---

## Keputusan desain (bukan backlog)
- **Tanpa soft-delete `FwRelation`** — sengaja tetap entitas `{}` polos; tak ada
  `deleted_at/by` di produksi.
- **`validateCategoryContext` product-rule untuk transaction_limit DIHAPUS** (#579) — validitas
  kini murni data-driven via `value_schema`; seed tak perlu bypass lagi.
- **Identifier role = `role_code` (string), bukan UUID** — konvergen dgn auth7 (klaim JWT
  `Roles` = kode via `GetRoleCodesByUser`; policy7 `applies_to_id` role = kode). Verified
  2026-06-26; bukan lagi dependency terbuka.
- **Index auto-name `idx_parameters_org_id`** — konsisten antara migration & DB (pasca
  rebaseline). Nama lama `idx_parameters_org` sudah tak relevan; nama generator dipertahankan
  (jangan rename — selaras DEF auto-naming). Bukan backlog.

> Konteks historis (Plan 07/12/13, planning issues) dipindah ke
> `_backup/policy7-cleanup-20260626/` di root devroot saat cleanup dokumentasi 2026-06-26.
