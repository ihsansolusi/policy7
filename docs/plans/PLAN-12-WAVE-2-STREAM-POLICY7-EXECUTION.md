# Plan 12 Wave 2 — Stream Policy7 Execution

> **Status**: Locked (Contract Definition)  
> **Date**: 2026-05-11  
> **Umbrella**: `core7-devroot#200`  
> **Wave**: `core7-devroot#203`  
> **Stream Epic**: `policy7#57`  
> **Child Issues**: `policy7#61`, `policy7#62`, `policy7#63`  
> **Boundary References**:
> - `docs/architecture/auth7-policy7-enterprise-boundary.md`
> - `docs/architecture/auth7-policy7-enterprise-change-control.md`
> - `docs/plans/integration/PLAN-12-WAVE-1-BACKEND-AUTHORITY-LOCK.md`

---

## 1. Scope Wave 2

W2 stream `policy7` dibatasi pada **contract definition**:

- policy category contract untuk enterprise policy screens
- caller context contract + validation rules
- policy error contract untuk facade consumers

Belum mencakup wiring implementasi.

---

## 2. Result per Child Issue

### `policy7#61` — Define policy category contract for enterprise screens

Status: **Done (Contract Lock)**

Hasil:
- Kontrak category untuk policy screens enterprise ditetapkan di spec API.
- Mapping screen group ke category code dikunci:
  - `transaction_limit`
  - `approval_threshold`
  - `operational_hours`
  - `product_access`
  - `rate`
  - `fee`
  - `regulatory_threshold`

Evidence:
- `docs/specs/02-api-detail.md` bagian **1.5 Policy Category Contract for Enterprise Screens**.

### `policy7#62` — Define caller context contract and validation rules

Status: **Done (Contract Lock)**

Hasil:
- Canonical fields terkunci: `org_id`, `branch_id` (conditional), `user_id` (admin mutation), `role_id/role_code` (role-scoped), `product` (product-scoped).
- Rule validasi context minimum terkunci untuk error handling konsisten.

Evidence:
- `docs/specs/02-api-detail.md` bagian **2.4 Caller Context Contract and Validation Rules (W2)**.

### `policy7#63` — Define policy error contract for facade consumers

Status: **Done (Contract Lock)**

Hasil:
- Error envelope untuk facade consumers dikunci (`code`, `message`, `http_status`, `retryable`, `details`, `trace_id`).
- Error code tambahan untuk kontrak integrasi ditetapkan:
  - `INVALID_CALLER_CONTEXT`
  - `TENANT_SCOPE_VIOLATION`
  - `INVALID_PARAMETER_SHAPE`
  - `CATEGORY_NOT_CONFIGURED`
  - `POLICY_BACKEND_UNAVAILABLE`

Evidence:
- `docs/specs/02-api-detail.md` bagian **6.1 Policy Error Contract for Facade Consumers (W2)**.

---

## 3. Contract Fields + Validation Summary

### Contract Fields

- Category contract fields: `category`, `name`, `applies_to`, `applies_to_id`, `product`, `value`, `value_type`, `unit`, `scope`.
- Caller context fields: `org_id`, `branch_id`, `user_id`, `role_id/role_code`, `product`.
- Error contract fields: `error.code`, `error.message`, `error.http_status`, `error.retryable`, `error.details`, `error.trace_id`.

### Validation Summary

1. `org_id` wajib untuk semua endpoint.
2. `branch_id` wajib untuk branch-scoped resolution.
3. `user_id` wajib untuk admin mutation.
4. Salah satu `role_id`/`role_code` wajib untuk role-scoped category.
5. `product` wajib untuk category product-scoped.
6. Tenant mismatch menghasilkan `TENANT_SCOPE_VIOLATION`.

---

## 4. Dependency ke Stream Lain

### Dependency ke `bos7-enterprise`

- Menjaga request payload/query agar sesuai category contract per screen.
- Menjamin propagation context fields minimum pada setiap request ke `policy7`.
- Menangani error envelope baru (`trace_id`, `retryable`, `details`) secara konsisten di UI.

### Dependency ke `auth7`

- Menetapkan canonical source untuk `role_id` vs `role_code` di claim context (untuk finalisasi role-scoped request shape).
- Menjaga pola konsumsi `policy7` tetap untuk ABAC input only, bukan ownership permission/policy lifecycle.

---

## 5. Short Update untuk `core7-devroot#203`

`W2 policy7 stream selesai di level contract definition (tanpa wiring). #61 category contract enterprise screens sudah terkunci; #62 caller context contract + validation rules sudah terkunci; #63 policy error contract untuk facade consumers sudah terkunci. Boundary ownership tetap: policy7 single source of truth policy/parameter. Dependency utama ke bos7-enterprise (context propagation + error handling adoption) dan auth7 (canonical role identifier untuk role-scoped context final).`
