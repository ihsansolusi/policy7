# Policy7 — Plan 07: Enterprise Admin UI Integration

> **Status**: Plan 13 W2 Locked  
> **Reference**: [`docs/architecture/auth7-policy7-enterprise-boundary.md`](../../../../docs/architecture/auth7-policy7-enterprise-boundary.md)

---

## Goal

Menyelaraskan policy7 dengan boundary baru Core7 sehingga `bos7-enterprise` menjadi admin UI utama untuk policy management, sementara `policy7` tetap menjadi backend authority dan source of truth untuk semua policy data.

---

## Scope

- Kontrak admin UI `bos7-enterprise -> policy7 /admin/v1/*`
- Kategori policy yang diekspos ke enterprise admin UI
- Context fields yang diharapkan dari caller
- Dokumentasi ownership policy7 vs auth7 vs core7-service-enterprise
- Lock route contract `bos7-enterprise -> policy7` agar non-overlap dengan IAM

---

## Work Items

### 7.1 Screen/API Contract
- Dokumentasikan workflow CRUD parameter dari `bos7-enterprise`
- Tegaskan bahwa semua mutasi tetap melalui `policy7 /admin/v1/*`

### 7.2 Supported Categories
- Transaction limits
- Approval thresholds
- Operational hours
- Product access
- Rates
- Fees
- Regulatory thresholds

### 7.3 Caller Context
- `org_id` wajib
- `branch_id` opsional sesuai kategori
- `user_id` untuk audit
- `role_id` atau role code sesuai policy resolution
- `product` untuk parameter product-specific

### 7.4 Ownership Guardrails
- Tidak ada ownership role/permission di policy7
- Tidak ada ownership user/session di policy7
- Tidak ada duplication of policy truth di `auth7` atau `core7-service-enterprise`

---

## Acceptance Criteria

- Spec policy7 menyebut admin API sudah ada dan tetap authoritative
- `bos7-enterprise` muncul sebagai primary admin UI consumer
- Kategori policy untuk admin UI enterprise terdokumentasi eksplisit
- Caller context minimum untuk integrasi terdokumentasi jelas

---

## Plan 13 W2 Conformance Lock (Issue #71)

### Capability audit untuk policy admin facade

Capability yang harus tetap di backend owner `policy7`:
- list/detail/create/update/delete/history/bulk-import policy parameters
- category policy management: transaction limit, approval threshold, operational hours, product access, rates, fees, regulatory threshold
- audit metadata pada mutation (`change_reason`)

Di luar capability ini:
- permission/role/session IAM ownership
- approval lifecycle orchestration end-to-end

### Route ownership mapping decision

- Semua route policy screens `bos7-enterprise` menuju endpoint `policy7 /admin/v1/*`.
- API owner = `policy7`, Data owner = `policy7`.
- `bos7-enterprise` hanya facade consumer, bukan backend authority policy.

### Caller context + facade error envelope lock

Caller context minimum:
- `org_id`
- `branch_id` (conditional)
- `role_id`/`role_code` (conditional)
- `effective_date` (conditional)
- `reason`/`change_reason` untuk mutation

Error envelope:
- `success=false`
- `error.code`
- `error.message`
- `error.http_status`
- `error.retryable`
- `error.details`
- `error.trace_id`

### Gap decision (ambiguity/overlap ke auth7)

Gap yang diidentifikasi:
- canonical role identifier (`role_id` vs `role_code`) lintas stream belum final sepenuhnya

Keputusan:
- untuk kontrak policy facade, field `role_id` atau `role_code` diterima secara conditional
- final canonical identifier menunggu sinkronisasi stream S1 (`auth7`) tanpa mengubah boundary ownership
