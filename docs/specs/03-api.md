# 03 — API

Semua endpoint internal-only. Error envelope seragam:
`{ code, message, http_status, retryable, details, trace_id }`.

Auth (lihat [05-security](05-security.md)):
- `/v1/*` & `/admin/v1/*` (CRUD): `Auth` (bearer JWT auth7 **atau** `X-Service-Key`) +
  `RequireDelegatedOrM2M`.
- `/admin/v1/.../wf-*`: `RequireM2M` + `VerifyAuditSignatureFromEnv` (group middleware).
- `GET /health`: tanpa auth.

## Consumer API — `/v1`

| Method | Path | Fungsi |
|---|---|---|
| GET | `/params/:category/:name` | ambil parameter (versi aktif) |
| GET | `/params/:category/:name/effective` | resolusi efektif (inheritance) |
| POST | `/params/transaction_limit/validate` | two-limit decision (AUTO/REQUIRES/REJECTED) |
| POST | `/params/authorization_limit/check` | cek kapasitas approver |
| GET | `/params/approval-thresholds` | ambang approval |
| GET | `/params/operational-hours` | jam operasional (input ABAC auth7) |
| GET | `/params/product-access` | aturan akses produk (input ABAC auth7) |
| GET | `/params/rates/:product` | bunga per produk |
| GET | `/params/fees/:product` | biaya per produk |
| GET | `/params/regulatory/:type` | ambang regulator (CTR/STR) |
| POST | `/params/regulatory/:type/check` | cek apakah transaksi lewat ambang |
| GET | `/contracts/categories` · `/contracts/caller-context` · `/contracts/errors` | metadata self-describing API |

## Admin API — `/admin/v1`

**Parameters**

| Method | Path | Fungsi |
|---|---|---|
| GET | `/params` | list |
| GET | `/params/:id` | detail |
| POST | `/params` | create |
| PUT | `/params/:id` | update (versioning) |
| DELETE | `/params/:id` | soft delete |
| POST | `/params/bulk-import` | import massal (error per-row) |
| POST | `/params/query` | DataTable query (filter/scope) |
| GET | `/params/:id/history` | riwayat versi |

**Categories** (Wave C — data-driven `value_schema`)

| Method | Path |
|---|---|
| GET | `/categories` · `/categories/:code` |
| POST | `/categories` |
| PUT | `/categories/:code` |
| DELETE | `/categories/:code` |

## Workflow callbacks — `/admin/v1/.../wf-*`

Dipanggil **workflow7** setelah approval. M2M + audit signature wajib.

| Method | Path |
|---|---|
| POST | `/params/wf-create` |
| PUT | `/params/:id/wf-update` |
| POST | `/params/:id/wf-delete` |
| POST | `/categories/wf-create` |
| PUT | `/categories/:code/wf-update` |
| POST | `/categories/:code/wf-delete` |

Alur mutasi end-to-end: UI (bos7-enterprise) → workflow7 (`policy-param-*-v1`) → callback
`wf-*` di sini → versioning + `parameter_history` (dengan `change_reason`) → forward audit7.
Lihat [04-integration](04-integration.md).
