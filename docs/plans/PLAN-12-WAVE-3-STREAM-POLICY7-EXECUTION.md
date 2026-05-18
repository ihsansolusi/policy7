# Plan 12 Wave 3 — Stream Policy7 Execution

> **Status**: Runtime Implemented  
> **Date**: 2026-05-11  
> **Umbrella**: `core7-devroot#200`  
> **Wave**: `core7-devroot#204`  
> **Stream Epic**: `policy7#57`  
> **Child Issues**: `policy7#64`, `policy7#65`, `policy7#66`

---

## 1. Issue Status

- `policy7#64` — **Done (runtime endpoints implemented)**
- `policy7#65` — **Done (validation + error envelope implemented)**
- `policy7#66` — **Done (audit/event mapping implemented for admin updates)**

---

## 2. Runtime API / Error / Audit Evidence

### Runtime contract endpoints

- `GET /v1/contracts/categories`
- `GET /v1/contracts/caller-context`
- `GET /v1/contracts/errors`

Implementasi:
- `internal/api/contract_handler.go`
- route registration: `internal/api/router.go`

### Validation + error envelope for facade consumers

- Error envelope standar (`success=false`, `error.code`, `error.message`, `error.http_status`, `error.retryable`, `error.details`, `error.trace_id`) di:
  - `internal/api/response.go`
- Digunakan di public/admin handlers:
  - `internal/api/parameter_handler.go`
  - `internal/api/admin_handler.go`
- Context validation utama yang di-enforce:
  - `org_id` wajib
  - `user_id` wajib untuk admin mutation
  - role/product requirement untuk category tertentu
  - invalid payload shape -> `INVALID_PARAMETER_SHAPE`

### Audit/event mapping for admin policy updates

- Event publish tetap di service layer:
  - `policy7.params.created`
  - `policy7.params.updated`
  - `policy7.params.deleted`
  - file: `internal/service/admin_parameter.go` + `internal/service/nats.go`
- Admin mutation responses sekarang menyertakan mapping metadata audit/event:
  - create -> `history_change_type=create`, `event_type=policy7.params.created`
  - update -> `history_change_type=update`, `event_type=policy7.params.updated`

---

## 3. Dependency ke S5 dan S1

### S5 (`bos7-enterprise`)

- Adopsi error envelope baru (`error.code`, `details`, `trace_id`, `retryable`).
- Konsumsi endpoint kontrak `/v1/contracts/*` untuk sinkronisasi category/context/error handling.
- Pastikan UI validation mengikuti requirement context runtime (org/user/role/product).

### S1 (`auth7`)

- Sinkronisasi canonical role identity (`role_id` atau `role_code`) untuk request role-scoped ke policy7.
- Pertahankan pola konsumsi policy7 sebagai ABAC input only (tanpa ownership policy lifecycle).
