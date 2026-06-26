# 03 — API (as-built)

Referensi endpoint **yang ada saat ini**. Untuk arah/kontrak target (pengelompokan
generik + apa yang di-deprecate) lihat [06-api-grouping](06-api-grouping.md). Kolom
**Status**: ✅ aktif dipakai · ⚠️ deprecation-candidate (di-track `trackUsage`, akan
di-collapse/retire) · lihat [ROADMAP](../ROADMAP.md).

Semua endpoint internal-only. Error envelope seragam:
`{ code, message, http_status, retryable, details, trace_id }`.

Auth (lihat [05-security](05-security.md)):
- `/v1/*` & `/admin/v1/*` (CRUD): `Auth` (bearer JWT auth7 **atau** `X-Service-Key`) +
  `RequireDelegatedOrM2M`.
- `/admin/v1/.../wf-*`: `RequireM2M` + `VerifyAuditSignatureFromEnv` (group middleware).
- `GET /health`: tanpa auth.

## Consumer API — `/v1`

| Method | Path | Fungsi | Status |
|---|---|---|---|
| GET | `/params/:category/:name/effective` | resolusi efektif (inheritance) | ✅ aktif (BFF simulator) |
| GET | `/params/:category/:name` | ambil parameter (versi aktif, tanpa resolusi) | ⚠️ → pakai `/effective` |
| POST | `/params/transaction_limit/validate` | two-limit decision (AUTO/REQUIRES/REJECTED) | ⚠️ decision-helper (lihat 06) |
| POST | `/params/authorization_limit/check` | cek kapasitas approver | ⚠️ tak ada caller |
| GET | `/params/approval-thresholds` | ambang approval | ⚠️ → generic resolve |
| GET | `/params/operational-hours` | jam operasional (rencana input ABAC auth7 — tak pernah dipakai) | ⚠️ → generic resolve |
| GET | `/params/product-access` | aturan akses produk (idem) | ⚠️ → generic resolve |
| GET | `/params/rates/:product` | bunga per produk | ⚠️ compatibility-only |
| GET | `/params/fees/:product` | biaya per produk | ⚠️ compatibility-only |
| GET | `/params/regulatory/:type` | ambang regulator (CTR/STR) | ⚠️ → generic resolve |
| POST | `/params/regulatory/:type/check` | cek apakah transaksi lewat ambang | ⚠️ tak ada caller |
| GET | `/contracts/categories` · `/contracts/caller-context` · `/contracts/errors` | metadata self-describing API (facade-era) | ⚠️ facade retired; BFF blokir |

> Semua baris ⚠️ adalah desain **hardcoded-per-kategori** yang tidak cocok dengan kategori
> data-driven. Di [06-api-grouping](06-api-grouping.md) digantikan oleh `resolve`
> (single/batch) + `snapshot` generik. `/v1/.../effective` adalah satu-satunya yang sudah
> sesuai pola target.

> **Usage telemetry.** Endpoint kandidat retire (rates/fees, basic get, boundary reads,
> regulatory, validate/check, `/contracts/*`, dan direct non-`wf` admin CRUD) dibungkus
> middleware `trackUsage` → counter `policy7_endpoint_usage_total{route, caller}` (`/metrics`),
> dengan `caller` = service pemanggil (M2M `client_id` / delegated `act.sub` / `system` /
> `user`). Dipakai untuk menentukan kapan aman meretire tiap route — lihat
> [ROADMAP](../ROADMAP.md). Selain itu lib7 mengekspos `http_requests_total{method, path,
> status}` untuk semua route.

## Admin API — `/admin/v1`

**Parameters**

| Method | Path | Fungsi | Status |
|---|---|---|---|
| GET | `/params` | list | ✅ aktif (BFF) |
| GET | `/params/:id` | detail | ✅ aktif (BFF) |
| GET | `/params/:id/history` | riwayat versi | ✅ aktif (BFF) |
| POST | `/params/bulk-import` | import massal (error per-row) | ✅ aktif (BFF) |
| POST | `/params` | create | ⚠️ direct CRUD → pakai `wf-create` |
| PUT | `/params/:id` | update (versioning) | ⚠️ direct CRUD → pakai `wf-update` |
| DELETE | `/params/:id` | soft delete | ⚠️ direct CRUD → pakai `wf-delete` |
| POST | `/params/query` | DataTable query (filter/scope) | ⚠️ tak ada caller |

**Categories** (Wave C — data-driven `value_schema`)

| Method | Path | Status |
|---|---|---|
| GET | `/categories` · `/categories/:code` | ✅ aktif (BFF + form dinamis) |
| POST | `/categories` | ⚠️ direct CRUD → pakai `categories/wf-create` |
| PUT | `/categories/:code` | ⚠️ direct CRUD → pakai `wf-update` |
| DELETE | `/categories/:code` | ⚠️ direct CRUD → pakai `wf-delete` |

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
