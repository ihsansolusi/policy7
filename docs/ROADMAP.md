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

## 🔭 Backlog / belum diimplementasi

### API migration → kontrak target (5 grup) — Fase 1–4 SELESAI (2026-06-26)

Review lintas-repo (2026-06-26) menemukan **~separuh surface tidak punya caller in-tree**.
Consumer runtime nyata: **bos7-enterprise BFF** (reads `/admin/v1` + `/v1/.../effective` +
bulk-import), **workflow7** (mutasi via `wf-*`), dan **auth7** (#161 `dd7b5fb` —
`operational_hours` via generic `/v1/params/{category}/{name}/effective` + opacache + NATS,
**bukan** endpoint hardcoded). auth7-ui / core7-service-* / Go SDK `pkg/client` tidak memanggil
policy7 via HTTP. auth7 memvalidasi desain Grup 2 — consumer baru pun memilih endpoint generik.

Karena temuan ini design-justified (endpoint hardcoded-per-kategori struktural tak cocok dengan
kategori data-driven) + 0 in-tree caller, retirement dieksekusi langsung (Fase 2+4 digabung),
bukan via observasi telemetry. Mengikuti [docs/specs/06-api-grouping.md](specs/06-api-grouping.md).

- **Fase 1 — Inquiry generik (additive)** ✅: `POST /v1/params/resolve` (batch) +
  `GET /v1/params?category=…` (snapshot) di `internal/api/inquiry_handler.go` +
  `ParameterService.SnapshotByCategory`.
- **Fase 2 — Deprecate hardcoded `/v1`** ✅ (digabung ke Fase 4): transitional telemetry
  (`policy7_endpoint_usage_total` / `trackUsage`) sempat dipasang lalu **dihapus** bersama
  endpoint-nya.
- **Fase 3 — Retire direct admin CRUD** ✅: hapus `POST/PUT/DELETE /admin/v1/params` &
  `/categories` + `POST /params/query` (handler + route). Semua mutasi lewat `wf-*`. Validasi
  (`validateScopeContext` + service category/value_schema gate) tetap utuh di jalur `wf-*`;
  test validasi dipindah ke `WfCreate`.
- **Fase 4 — Hapus `/v1` hardcoded + `/contracts/*` + handler mati** ✅: dihapus
  `GetParameter`(basic)/`GetRates`/`GetFees`/`GetRegulatory`/`CheckRegulatory`/
  `CheckAuthorizationLimit`/`GetApprovalThresholds`/`GetOperationalHours`/`GetProductAccess` +
  `contract_handler.go` + `usage_metrics.go`. Pertahankan `…/effective` +
  `transaction_limit/validate` (decision helper).

**Surface akhir:** Grup 1 (`/admin/v1` reads + bulk-import + `wf-*`) · Grup 2
(`/effective`, `resolve`, snapshot, `transaction_limit/validate`) · Grup 3 (`/admin/v1/categories`
reads) · Grup 4 (NATS) · Grup 5 (`/health`, `/metrics`). Lihat [03-api](specs/03-api.md).

- **Fase 5 — Discovery + SDK** ✅ (2026-06-26): **`pkg/client` Go SDK dihapus** (4 method, 0
  importer; konsumen pakai REST `/v1` langsung — pola acuan auth7 `internal/policy7client`).
  Menghapusnya sekaligus menyelesaikan kegagalan test pre-existing `pkg/client`.
  **Discovery `value_schema` di `/v1`: ditunda (YAGNI)** — belum ada consumer non-admin yang
  butuh bentuk value (auth7 tahu shape-nya); tetap di `/admin/v1/categories`. Tambahkan saat
  consumer generik pertama muncul.

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
