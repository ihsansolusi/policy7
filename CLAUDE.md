# CLAUDE.md — Policy7

Panduan konteks untuk Claude AI saat bekerja di repository `policy7`.

---

## Identitas Proyek

- **Proyek**: Policy7 — Business policy & parameter service untuk ekosistem Core7
- **Fase saat ini**: ✅ **COMPLETED** — All implementation plans (01-06) successfully completed
- **Planning Status**: ✅ **COMPLETE** — All 5 specs finalized, 6 plans, 55 GitHub issues
- **Repo**: `github.com/ihsansolusi/policy7` (branch: `main`)
- **Submodule di**: `/home/galih/Works/projects/banks/core7-devroot/supported-apps/policy7`
- **GitHub Org**: `ihsansolusi`
- **Root Issue**: [#1 — Policy7 v1.0](https://github.com/ihsansolusi/policy7/issues/1)
- **Total GitHub Issues**: 55 (1 root + 6 plans + 48 implementation)
- **Project Board**: [Core7 v2026.1](https://github.com/orgs/ihsansolusi/projects/8)

---

## Tujuan Policy7

Policy7 adalah service terpisah yang menyimpan **semua parameter bisnis** yang bisa berubah tanpa deploy aplikasi. Ini meliputi:

- Transaction limits (employee & customer)
- Approval thresholds
- Operational hours
- Interest rates & fees
- Regulatory thresholds (CTR/STR)
- Product access rules
- Business rules/scoring thresholds

---

## Hubungan dengan Auth7 & Core7

```
auth7:      "BOLEHKAH user ini akses resource ini?" → YES/NO
policy7:    "BOLEHKAH seberapa? BERAPA batasnya?" → numeric/threshold
workflow7:  "SIAPA yang harus approve?" → approval flow
```

Auth7 menyediakan **role & permission**.
Policy7 menyediakan **limit & parameter** per role/customer/product.
Core7 services query **keduanya** untuk decision lengkap.

---

## Struktur Repo

```
policy7/
├── cmd/
│   └── server/
│       └── main.go              # Entry point
├── internal/
│   ├── api/                     # REST handlers (Gin)
│   ├── service/                 # Business logic
│   ├── store/                   # Database access (pgx + sqlc)
│   └── domain/                  # Entities, errors
├── docs/
│   ├── specs/                   # Specs (00-overview, dll)
│   └── plans/                   # Implementation plans
├── migrations/                  # golang-migrate
└── scripts/                     # DB operations
```

---

## Teknologi Stack

| Komponen | Teknologi |
|---|---|
| Language | Go 1.22+ |
| Framework | Gin (REST) |
| Database | PostgreSQL 16 (pgx + sqlc) |
| Cache | Redis (optional, untuk hot params) |
| Migrations | golang-migrate |

---

## Aturan Kode

- Setiap Go method: `const op = "package.Type.Method"`
- Error wrapping: `fmt.Errorf("%s: %w", op, err)`
- Multi-tenant: semua query wajib filter `org_id`
- No secrets in config files — hanya `"${ENV_VAR}"`

---

## Referensi

### Policy7 Specs
- `docs/specs/00-overview.md` — Vision & scope
- `docs/specs/01-architecture.md` — Clean architecture & hybrid model
- `docs/specs/02-api-detail.md` — API specification (Public + Admin)
- `docs/specs/02-api-detail-samples.md` — Banking use cases validation

### External References
- Auth7 Specs: `../auth7/docs/specs/` (di devroot)
- Auth7 Spec 04: Authorization (referensi policy7): `../auth7/docs/specs/04-authorization.md`
- Hybrid Model Doc: `../../../docs/infra/HYBRID-MESSAGING-MODEL.md`

## Specs Summary

| Spec | Contents | Status |
|------|----------|--------|
| **00-overview** | Vision, scope v1.0, parameter types, hubungan dengan auth7/workflow7 | ✅ 1.0-draft |
| **01-architecture** | Clean architecture, interfaces, caching strategy, **hybrid Redis+NATS model** | ✅ 0.2-draft |
| **02-api-detail** | Public API (7 endpoints), Admin API (7 endpoints), **two-limit pattern**, authorization limits | ✅ 0.2-draft |
| **02-api-detail-samples** | 10 banking cases: **teller two-limit**, supervisor auth, rates, fees, regulatory, versioning, bulk import | ✅ Done |
| **03-data-model** | **PostgreSQL schema (parameters, parameter_history), Redis key patterns, JSONB structures, migration strategy** | ✅ **0.1-draft** |
| **04-integration** | **Auth7 OPA integration, Core7 Enterprise, Workflow7 approval, Notif7 regulatory alerts, NATS events, Go client** | ✅ **0.1-draft** |

### Key API Patterns

**Two-Limit Pattern (Teller Workflow):**
```
Authorization Limit (auto-auth threshold)    Transaction Limit (max input)
           Rp 25jt                                 Rp 100jt
              ↓                                       ↓
    Amount ≤ 25jt? ✅ Auto              Amount ≤ 100jt? ✅ Can Input
    Amount > 25jt? ⏸️ Needs Auth        Amount > 100jt? ❌ Rejected

Simple Decision Flow:
• Amount ≤ Auth Limit     → AUTO_AUTHORIZED
• Auth < Amount ≤ Trans   → REQUIRES_AUTHORIZATION  
• Amount > Trans Limit    → REJECTED
```

**Authorization Limits:**
- `teller_transfer_max` — Transaction limit (Rp 100jt)
- `teller_authorization_limit` — Auto-auth threshold (Rp 25jt)
- `supervisor_auth_max` — Max supervisor bisa authorize (Rp 100jt)
- `branch_manager_auth_max` — Max BM bisa authorize (Rp 500jt)

---

## Integration Overview

Policy7 di-integrasikan dengan 4 services utama di ekosistem Core7:

| Service | Integration Pattern | Use Case |
|---------|---------------------|----------|
| **auth7** | OPA/Rego query policy7 via HTTP | ABAC rules dengan operational hours, product access |
| **core7-enterprise** | HTTP REST API | Transaction validation (two-limit pattern), rates & fees |
| **workflow7** | HTTP REST API | Approval thresholds, authorization limits untuk approvers |
| **notif7** | NATS pub/sub | Regulatory alerts (CTR/STR threshold exceeded) |

**Integration Architecture:**
```
┌─────────┐   ┌───────────────┐   ┌───────────────┐   ┌─────────┐
│  auth7  │   │ core7-enterp. │   │   workflow7   │   │ notif7  │
│  (OPA)  │   │  (validate)   │   │   (approval)  │   │ (alert) │
└────┬────┘   └───────┬───────┘   └───────┬───────┘   └────┬────┘
     │                │                   │                │
     └────────────────┴───────────────────┴────────────────┘
                              │
                    ┌─────────▼──────────┐
                    │      policy7       │
                    │ (centralized param)│
                    └─────────┬──────────┘
                              │
           ┌──────────────────┼──────────────────┐
           │                  │                  │
    ┌──────▼──────┐   ┌──────▼──────┐   ┌──────▼──────┐
    │ PostgreSQL  │   │    Redis    │   │    NATS     │
    │  (master)   │   │   (cache)   │   │  (events)   │
    └─────────────┘   └─────────────┘   └─────────────┘
```

**Go Client Library:**
```go
import "github.com/ihsansolusi/policy7/pkg/client"

policy7 := client.NewClient(baseURL, apiKey, serviceID)
validation, _ := policy7.ValidateTransaction(ctx, req)
```

---

## Data Model Overview

### Core Tables

| Table | Purpose | Key Columns |
|-------|---------|-------------|
| `parameters` | Master parameter data with versioning | `org_id`, `category`, `name`, `value` (JSONB), `version`, `is_active` |
| `parameter_history` | Audit trail for all changes | `parameter_id`, `previous_value`, `new_value`, `change_reason` |
| `parameter_categories` | Category metadata (optional) | `code`, `name`, `value_schema` (JSON) |

### Key Design Decisions

- **JSONB Value**: Field `value` menggunakan JSONB untuk fleksibilitas tipe data (number, string, object, array)
- **Versioning**: Setiap update membuat record baru dengan `version++`, record lama `is_active = FALSE`
- **Unique Constraint**: Hanya 1 versi aktif per kombinasi `(org_id, category, name, applies_to, applies_to_id, product)`
- **Soft Delete**: Menggunakan `is_active` + `effective_until` (bukan hard delete)

### Redis Cache Patterns

```
policy7:{org_id}:{category}:{name}:{applies_to}:{applies_to_id}:{product}
policy7:uuid-bjbs:transaction_limit:teller_transfer_max:role:teller:transfer
policy7:uuid-bjbs:rate:deposito_12m:product:deposito:null
```

### JSONB Value Examples

**Transaction Limit:**
```json
{
  "transaction_limit": 100000000,
  "authorization_limit": 25000000,
  "currency": "IDR",
  "scope": "per_transaction"
}
```

**Interest Rate:**
```json
{
  "rate": 4.5,
  "rate_unit": "percent_per_year",
  "calculation_method": "simple_interest",
  "tenor_months": 12
}
```

---

## Status & Timeline

### Current Status (2026-04-27)
| Spec | Status | Version |
|------|--------|---------|
| 00-overview.md | ✅ Done | 1.0-draft |
| 01-architecture.md | ✅ Approved | 0.2-draft |
| 02-api-detail.md | ✅ **Enhanced** | **0.3-draft** |
| 02-api-detail-samples.md | ✅ Done | Banking cases validation |
| 03-data-model.md | ✅ **Enhanced** | **0.2-draft** |
| 04-integration.md | ✅ **Enhanced** | **0.2-draft** |
| PLAN-OVERVIEW.md | ✅ **Enhanced** | Updated Plan 06 |

### Specs Complete ✅

**All 5 specs created, all 7 gaps fixed, NATS integration detailed per PM requirements.**

**Status: PRODUCTION-READY** 🚀

### Gap Fixes Applied
| Gap | Fix Location |
|-----|--------------|
| Rate Limiting | Spec 02 Section 7 (comprehensive) |
| Error Codes | Spec 02 Section 10 (40+ codes) |
| Pagination | Spec 02 Section 8 (complete spec) |
| Bulk Import | Spec 02 Section 9 (error handling) |
| Cache Strategy | Spec 03 Section 4.5 (invalidation, warming) |
| Backup & DR | Spec 03 Section 8 (RTO/RPO, recovery) |
| Inheritance | Spec 03 Section 5 (algorithm, caching) |

### PM Requirements (PROMPT-P-POL7-01) ✅ Covered
| Requirement | Location | Status |
|-------------|----------|--------|
| NATS subjects defined | Spec 04 Section 6.1 | ✅ `policy7.params.created/updated/deleted` |
| Health check via NATS | Spec 04 Section 6.4 | ✅ Request-reply with detailed response |
| JetStream decision | Spec 04 Section 6.5 | ✅ Core NATS v1.0, JetStream v1.1 |
| Plan 06 NATS details | PLAN-OVERVIEW.md Plan 06 | ✅ Implementation checklist |
| Multi-instance cache coordination | Spec 04 Section 6.3 | ✅ All instances subscribe & invalidate |

### Key Decisions Made
- ✅ Architecture: Clean architecture (same as service7-template)
- ✅ Interface-first: Mock auth for parallel development
- ✅ **Hybrid messaging: Redis (cache) + NATS (events)**
- ✅ Event streaming: NATS v1.0
- ✅ **Two-Limit Pattern: Transaction Limit + Authorization Limit**
- ✅ **Authorization Limits: Separate limits for approvers**
- ✅ Conditional parameters: Included in v1.0
- ✅ Parameter inheritance: Included in v1.0
- ✅ Rate limiting: nginx/API gateway

### GitHub Issues Status ✅ COMPLETE

**All 55 issues created with correct hierarchy:**

```
core7-devroot#35 (105 - Supported Apps)
└── policy7#1: Policy7 v1.0 — Business Policy & Parameter Service [ROOT EPIC]
    ├── policy7#2: Plan 01 — Foundation & Infrastructure [10 issues, 32 pts]
    ├── policy7#3: Plan 02 — Admin API [8 issues, 37 pts]
    ├── policy7#4: Plan 03 — Parameter Categories [8 issues, 32 pts]
    ├── policy7#5: Plan 04 — Rates & Fees [6 issues, 22 pts]
    ├── policy7#6: Plan 05 — Integration [8 issues, 39 pts]
    └── policy7#7: Plan 06 — Performance & Caching with NATS [8 issues, 32 pts]
```

| Metric | Value |
|--------|-------|
| **Total Issues** | 55 (#1-#55) |
| **Root Epic** | 1 (#1) |
| **Plan Groups** | 6 (#2-#7) |
| **Individual Issues** | 48 (#8-#55) |
| **Story Points** | 194 pts |
| **Project Board** | #8 (Core7 v2026.1) |

**Quick Links:**
- [Policy7 Issues](https://github.com/ihsansolusi/policy7/issues)
- [Root Issue #1](https://github.com/ihsansolusi/policy7/issues/1)
- [Project Board](https://github.com/orgs/ihsansolusi/projects/8)

### Development Approach
**PARALLEL with auth7** — Timeline 4-5 months
- Month 1-2: Planning & Foundation (Plan 01-02) — ✅ Planning complete, ready to start
- Month 3-4: Parallel core development (Plan 03-04)
- Month 5: Integration with auth7 (Plan 05)
- Month 6: Finalization & testing (Plan 06)

### Current Status: ✅ COMPLETED & HANDOVER READY

| Phase | Status | Notes |
|-------|--------|-------|
| **Specs** | ✅ Complete | All 5 specs, 7 gaps fixed |
| **GitHub Issues** | ✅ Complete | All 55 issues are Closed |
| **Plan 01** | ✅ Done | PostgreSQL, Redis, CI/CD, NATS initialized |
| **Plan 02** | ✅ Done | Admin API CRUD, Versioning, Audit Trails |
| **Plan 03** | ✅ Done | Inheritance logic, Validate Limits |
| **Plan 04** | ✅ Done | Rates, Fees, Regulatory checks |
| **Plan 05** | ✅ Done | Go Client SDK, S2S Auth, E2E Tests |
| **Plan 06** | ✅ Done | Cache Warming, Singleflight, NATS Events |

**Next Action:** Handover to Core7 Manager. Project Policy7 is ready for production integration.

---

*Last Updated: 2026-04-28 — All Plans Implemented, Ready for Handover* 🚀
