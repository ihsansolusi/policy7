# AGENT.md — Policy7 Handoff

Ringkasan status untuk lanjut sesi berikutnya di repo `policy7`.

## Current State

- `policy7` tetap single source of truth untuk policy/parameter.
- Ownership permission/role/session tetap di `auth7`.
- Policy Management `bos7-enterprise` diarahkan ke `policy7 /admin/v1/*`.
- Error envelope facade sudah distandardkan dengan `code`, `message`, `http_status`, `retryable`, `details`, `trace_id`.

## Progress Summary

- Plan 12 W1-W5 selesai.
- Plan 13 W2 selesai.
- Issue terkait sudah ditutup: `#58-#71`.
- W5 conformance audit recommendation: `NOT READY` untuk retire legacy compatibility paths.

## Important Artifacts

- `docs/specs/02-api-detail.md`
- `docs/plans/PLAN-07-ENTERPRISE-ADMIN-UI-INTEGRATION.md`
- `docs/plans/PLAN-12-WAVE-1-STREAM-POLICY7-EXECUTION.md`
- `docs/plans/PLAN-12-WAVE-2-STREAM-POLICY7-EXECUTION.md`
- `docs/plans/PLAN-12-WAVE-3-STREAM-POLICY7-EXECUTION.md`
- `docs/plans/PLAN-12-WAVE-4-STREAM-POLICY7-EXECUTION.md`
- `docs/plans/PLAN-12-WAVE-5-STREAM-POLICY7-CONFORMANCE-AUDIT.md`
- Runtime files:
  - `internal/api/contract_handler.go`
  - `internal/api/response.go`
  - `internal/api/router.go`
  - `internal/api/admin_handler.go`
  - `internal/api/parameter_handler.go`

## Open Risks / Next Focus

- Legacy compatibility paths `GET /v1/params/rates/:product` dan `GET /v1/params/fees/:product` masih aktif sebagai `compatibility-only`.
- Telemetry usage endpoint-level belum ada untuk retirement decision yang aman.
- Canonical role identifier (`role_id` vs `role_code`) masih dependency lintas stream dengan `auth7`.

