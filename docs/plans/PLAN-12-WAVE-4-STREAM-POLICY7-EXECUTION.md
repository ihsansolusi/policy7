# Plan 12 Wave 4 — Stream Policy7 Execution

> **Status**: Compatibility + Conformance Locked  
> **Date**: 2026-05-12  
> **Repo**: `ihsansolusi/policy7`  
> **Parent Stream Epic**: `policy7#57`  
> **Wave**: Plan 12 W4  
> **Issues**: `#67`, `#68`, `#69`

---

## 1) Issue `#67` — Compatibility Surface Classification (enterprise facade usage)

### Compatibility Register

| Surface / Path | Consumer | Status | Owner | Notes |
|---|---|---|---|---|
| `GET /v1/contracts/categories` | `bos7-enterprise` facade | `active` | `policy7` | W3 runtime contract endpoint |
| `GET /v1/contracts/caller-context` | `bos7-enterprise`, `auth7` integration | `active` | `policy7` | Caller context reference source |
| `GET /v1/contracts/errors` | `bos7-enterprise` facade | `active` | `policy7` | Error code contract source |
| `GET /admin/v1/params*` | `bos7-enterprise` policy admin screens | `active` | `policy7` | Backend authority policy CRUD |
| `GET /v1/params/approval-thresholds` | enterprise facade | `facade` | `policy7` | Access ke category policy via public query |
| `GET /v1/params/operational-hours` | `auth7` ABAC input + enterprise | `facade` | `policy7` | ABAC parameterized input |
| `GET /v1/params/product-access` | `auth7` ABAC input + enterprise | `facade` | `policy7` | Product policy lookup |
| `GET /v1/params/rates/:product` | enterprise facade | `compatibility-only` | `policy7` | Legacy naming (`rates`) masih dipertahankan |
| `GET /v1/params/fees/:product` | enterprise facade | `compatibility-only` | `policy7` | Legacy naming (`fees`) masih dipertahankan |
| `GET /v1/params/regulatory/:type` + `/check` | enterprise + notif flow | `active` | `policy7` | Regulatory threshold source |
| `POST /v1/params/transaction_limit/validate` | enterprise transaction flow | `active` | `policy7` | Runtime validation source |
| `POST /v1/params/authorization_limit/check` | enterprise/workflow flow | `facade` | `policy7` | Approver limit lookup |

### Guard / Conformance Notes

- Seluruh surface di atas tetap consume/serve policy dari `policy7`; tidak ada ownership shift ke enterprise/auth7.
- `admin/v1` tetap authority mutation policy; UI enterprise bertindak sebagai facade consumer.

### Evidence (files)

- `internal/api/router.go`
- `internal/api/contract_handler.go`
- `internal/api/parameter_handler.go`
- `internal/api/admin_handler.go`
- `docs/plans/PLAN-12-WAVE-3-STREAM-POLICY7-EXECUTION.md`

### Blocker + Next Action

- Blocker: belum ada telemetry usage per endpoint untuk memastikan surface `compatibility-only` aman dipensiunkan.
- Next action (W5): tambah observability counter per legacy path lalu freeze tanggal cutover berbasis traffic.

---

## 2) Issue `#68` — Deprecation + Cutover Conditions for Legacy Parameter Paths

### Compatibility Register (legacy-focused)

| Legacy Surface / Path | Status | Deprecation Marker | Cutover Condition | Target |
|---|---|---|---|---|
| `GET /v1/params/rates/:product` | `compatibility-only` | Keep for legacy consumers | Semua consumer migrate ke canonical category `rate` contract metadata + equivalent lookup path | `retire-target` |
| `GET /v1/params/fees/:product` | `compatibility-only` | Keep for legacy consumers | Semua consumer migrate ke canonical category `fee` contract metadata + equivalent lookup path | `retire-target` |
| `GET /v1/params/:category/:name` dengan category non-canonical (`rates`,`fees`,`regulatory`) | `facade` | Allowed sementara | Consumer align ke canonical category set W2 contract (`rate`,`fee`,`regulatory_threshold`) | `facade->active canonical` |

### Guard / Conformance Notes

- Deprecation hanya pada path/surface compatibility, bukan pada ownership data.
- Selama cutover, source of truth tetap `policy7`; tidak ada fallback ownership ke modul lain.

### Evidence (files)

- `internal/api/parameter_handler.go` (legacy path masih dilayani runtime)
- `docs/specs/02-api-detail.md` (canonical category contract W2)
- `docs/plans/PLAN-12-WAVE-4-STREAM-POLICY7-EXECUTION.md` (register deprecation/cutover)

### Blocker + Next Action

- Blocker: canonical replacement path detail belum difinalisasi lintas S5 consumer routing.
- Next action (W5): lock migration matrix endpoint-by-endpoint dengan S5, lalu set phased retirement plan.

---

## 3) Issue `#69` — Verify ABAC Lookup remains `auth7 -> policy7` without boundary regression

### Compatibility Register (ABAC flow surfaces)

| ABAC Surface | Status | Expected Flow | Conformance |
|---|---|---|---|
| `GET /v1/params/operational-hours` | `active` | `auth7 -> policy7` | pass |
| `GET /v1/params/product-access` | `active` | `auth7 -> policy7` | pass |
| Contract notes (`/v1/contracts/caller-context`) | `active` | `auth7` consume context contract | pass |
| Integration guidance docs | `active` | ABAC input only, bukan permission ownership | pass |

### Guard / Conformance Notes

- `auth7` tetap pemilik permission/role/session; `policy7` hanya parameter input ABAC.
- Tidak ada endpoint/admin flow yang memindahkan permission ownership ke `policy7`.
- Tidak ada indikasi boundary regression dari implementasi W3 runtime contract/error/audit.

### Evidence (files)

- `docs/specs/04-integration.md` (guardrail `auth7` consume `policy7` untuk ABAC input)
- `docs/specs/00-overview.md` (ownership lock statement)
- `internal/api/router.go` + `internal/api/contract_handler.go` (contract endpoint availability)
- `internal/api/parameter_handler.go` (query surfaces ABAC parameters)

### Blocker + Next Action

- Blocker: belum ada automated conformance test lintas repo (`auth7` caller vs policy7 contract) di CI integration.
- Next action (W5): tambahkan cross-repo integration checklist/test case untuk ABAC lookup conformance.
