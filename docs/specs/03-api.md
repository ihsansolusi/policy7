# 03 — API (as-built)

Referensi endpoint **yang ada saat ini** (pasca-retirement Fase 2–4, 2026-06-26).
Untuk rasional pengelompokan + peta transisi lihat [06-api-grouping](06-api-grouping.md).

Semua endpoint internal-only. Error envelope seragam:
`{ code, message, http_status, retryable, details, trace_id }`.

Auth (lihat [05-security](05-security.md)):
- `/v1/*` & `/admin/v1/*`: `Auth` (bearer JWT auth7 **atau** `X-Service-Key`) +
  `RequireDelegatedOrM2M`.
- `/admin/v1/.../wf-*`: `RequireM2M` + `VerifyAuditSignatureFromEnv` (group middleware).
- `GET /health`: tanpa auth.

## Consumer API — `/v1` (Grup 2: inquiry generik)

| Method | Path | Fungsi |
|---|---|---|
| GET | `/params/:category/:name/effective` | resolve satu param (inheritance `user→role→branch→global`) |
| POST | `/params/resolve` | resolve banyak param sekaligus (batch) — `{context, keys[]}` |
| GET | `/params?category=&product=` | snapshot semua param aktif dalam kategori |
| POST | `/params/transaction_limit/validate` | decision helper two-limit (AUTO/REQUIRES/REJECTED) — semantik eksplisit |

> Endpoint hardcoded-per-kategori lama (`operational-hours`, `product-access`,
> `approval-thresholds`, `rates/:product`, `fees/:product`, `regulatory/:type`(+`/check`),
> `authorization_limit/check`, basic `GET /params/:category/:name`) dan facade
> `/contracts/*` **sudah dihapus** (Fase 2–4) — digantikan `resolve`/`snapshot` generik.

### Discovery — `/v1` (Grup 3)

Read-only metadata kategori + `value_schema` agar consumer (atau tooling) bisa menafsirkan
bentuk `value` secara generik. Handler sama dgn `/admin/v1/categories`, dimount read-only.

| Method | Path | Fungsi |
|---|---|---|
| GET | `/categories` · `/categories/:code` | metadata kategori + `value_schema` (+ `x-ui`/`x-rules`) |

## Admin API — `/admin/v1` (Grup 1: management)

**Parameters** — reads + bulk-import; mutasi lewat `wf-*` (bukan direct CRUD).

| Method | Path | Fungsi |
|---|---|---|
| GET | `/params` | list |
| GET | `/params/:id` | detail |
| GET | `/params/:id/history` | riwayat versi **penuh** — full chain by identity tuple, oldest→newest (#587) |
| POST | `/params/bulk-import` | import massal best-effort; balas `{summary, results:[{row,status,code,error\|id}]}` per-row (#588) |

**Categories** (Wave C — data-driven `value_schema`) — reads; mutasi lewat `categories/wf-*`.

| Method | Path |
|---|---|
| GET | `/categories` · `/categories/:code` |

> Direct (non-`wf`) CRUD untuk params & categories + `POST /params/query` **sudah dihapus**
> (Fase 3) — semua mutasi melalui approval `wf-*`.

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
