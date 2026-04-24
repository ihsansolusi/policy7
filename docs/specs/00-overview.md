# Policy7 — Spec 00: Overview & Vision

> **Versi**: 1.0-draft | **Tanggal**: 2026-04-24 | **Fase**: Brainstorming

---

## 1. Latar Belakang

Core7 adalah ekosistem core banking yang membutuhkan **parameter bisnis terpusat** yang bisa diubah tanpa deploy aplikasi. Beberapa parameter yang perlu dikelola:

- **Transaction limit** — berapa maksimal transaksi per role/customer
- **Approval threshold** — berapa rupiah perlu approval
- **Operational hours** — jam operasional per role
- **Interest rates & fees** — bunga produk, biaya admin
- **Regulatory thresholds** — CTR/STR reporting limits
- **Product access rules** — role mana boleh akses produk mana

Tanpa service terpisah, parameter ini tersebar di masing-masing service (core7 enterprise, workflow7, dll) yang menyebabkan:
- Duplikasi data
- Inkonsistensi antar service
- Sulit audit & versioning
- Perlu deploy setiap kali ubah parameter

---

## 2. Visi Policy7

> **Policy7** adalah *centralized business policy & parameter service* untuk ekosistem Core7 yang menyediakan:
> parameter bisnis real-time, versioned, auditable, yang bisa dikonsumsi oleh semua service
> (auth7, core7 enterprise, workflow7, notif7) via REST API.

---

## 3. Scope Parameter

### 3.1 In Scope v1.0

| Kategori | Parameter | Contoh |
|---|---|---|
| **Transaction Limit** | Per-role max amount | Teller max Rp 10M per transaksi |
| **Transaction Limit** | Per-role daily limit | Teller max Rp 50M per hari |
| **Transaction Limit** | Per-customer limit | Nasabah regular max Rp 50M per hari |
| **Approval Threshold** | Per-role threshold | > Rp 50M perlu branch_manager |
| **Operational Hours** | Per-role jam kerja | Teller 08:00-16:00 WIB |
| **Product Access** | Per-role product filter | Teller hanya tabungan & deposito |
| **Interest Rates** | Per-tenor bunga | Deposito 12 bulan = 4.5% p.a. |
| **Fees & Tarif** | Per-product biaya | Transfer antar bank = Rp 6.500 |
| **Regulatory** | CTR/STR threshold | Lapor CTR jika transaksi > Rp 100M |

### 3.2 Out of Scope v1.0

- **ABAC boolean rules** — tetap di auth7 (OPA/Rego)
- **Role & permission definition** — tetap di auth7
- **Approval flow orchestration** — tetap di workflow7
- **Real-time scoring engine** — v2.0 (credit scoring, risk rating)

---

## 4. Arsitektur

```
┌─────────────────────────────────────────────────────────────┐
│                       Core7 Ecosystem                       │
│                                                             │
│  ┌──────────┐    ┌──────────────┐    ┌──────────────────┐  │
│  │  auth7   │    │ core7 enter- │    │    workflow7     │  │
│  │          │    │   prise      │    │                  │  │
│  │ "BOLEH-  │    │              │    │ "SIAPA approve?" │  │
│  │  KAH?"   │    │              │    │                  │  │
│  │ YES/NO   │    │              │    │                  │  │
│  └────┬─────┘    └──────┬───────┘    └────────┬─────────┘  │
│       │                 │                      │            │
│       │                 │  GET /v1/params/     │            │
│       │                 │  limits?role=teller  │            │
│       │                 │       │              │            │
│       │                 ▼       │              │            │
│       │          ┌──────────────▼──────────┐   │            │
│       │          │        policy7          │   │            │
│       │          │   "BOLEHKAH SEBERAPA?"  │   │            │
│       │          │   "BERAPA BATASNYA?"    │   │            │
│       │          │   "KAPAN? BERAPA?"      │   │            │
│       │          └──────────┬──────────────┘   │            │
│       │                     │                  │            │
│       │              ┌──────┴──────┐          │            │
│       │              ▼             ▼          │            │
│       │        ┌──────────┐  ┌──────────┐    │            │
│       │        │PostgreSQL│  │  Redis   │    │            │
│       │        │   16     │  │  (hot)   │    │            │
│       │        └──────────┘  └──────────┘    │            │
│       │                                      │            │
│       │   Auth7 OPA bisa query policy7       │            │
│       │   untuk jam operasional, threshold   │            │
│       │   saat ABAC evaluation               │            │
│       │◄─────────────────────────────────────┘            │
│       │                                                   │
└───────┼───────────────────────────────────────────────────┘
        │
   notif7 (consume regulatory thresholds untuk alert)
```

---

## 5. Hubungan dengan Auth7

### 5.1 Auth7 → Policy7 (OPA Query)

Auth7 OPA/Rego bisa query policy7 untuk parameter saat ABAC evaluation:

```rego
package authz

# Query policy7 untuk jam operasional
teller_hours = data.policy7.operational_hours["teller"]

allow {
  input.user.roles[_] == "teller"
  input.resource.type == "transaction"
  input.action == "create"
  time.now_ns() >= teller_hours.start_ns
  time.now_ns() <= teller_hours.end_ns
}
```

### 5.2 Policy7 TIDAK Menyimpan ABAC Rules

ABAC boolean rules (allow/deny) tetap di auth7.
Policy7 hanya menyimpan **data parameter** yang dipakai ABAC rules.

---

## 6. Teknologi Stack

| Komponen | Teknologi |
|---|---|
| Language | Go 1.22+ |
| Framework | Gin (REST) |
| Database | PostgreSQL 16 (pgx + sqlc) |
| Cache | Redis (optional, hot params) |
| Migrations | golang-migrate |
| Config | env-based |

---

## 7. API Surface

### 7.1 Parameter Query

```
GET /v1/params/limits?role=teller&product=transfer
GET /v1/params/limits?customer_type=regular&product=transfer
GET /v1/params/thresholds?role=supervisor&type=transfer
GET /v1/params/operational-hours?role=teller
GET /v1/params/rates?product=deposito&tenor=12m
GET /v1/params/fees?product=transfer&channel=ATM
GET /v1/params/regulatory?type=CTR&country=ID
```

### 7.2 Parameter CRUD (Admin)

```
GET    /admin/v1/params                  # list all params
GET    /admin/v1/params/:id             # get single param
POST   /admin/v1/params                 # create param
PUT    /admin/v1/params/:id             # update param
DELETE /admin/v1/params/:id             # soft delete
GET    /admin/v1/params/:id/history     # version history
```

### 7.3 Parameter Types

```json
{
  "id": "uuid",
  "org_id": "uuid",
  "category": "transaction_limit",
  "name": "teller_transfer_max",
  "applies_to": "role",
  "applies_to_id": "teller",
  "product": "transfer",
  "value": 10000000,
  "unit": "IDR",
  "scope": "per_transaction",
  "effective_from": "2026-01-01T00:00:00Z",
  "effective_until": null,
  "version": 1,
  "created_by": "admin-uuid",
  "created_at": "2026-04-24T00:00:00Z"
}
```

---

## 8. Data Model (High Level)

```sql
-- Parameters table (versioned)
CREATE TABLE parameters (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL,
    category        VARCHAR(50) NOT NULL,      -- 'transaction_limit', 'rate', 'fee', 'regulatory'
    name            VARCHAR(100) NOT NULL,     -- 'teller_transfer_max'
    applies_to      VARCHAR(50) NOT NULL,      -- 'role', 'customer_type', 'product', 'global'
    applies_to_id   VARCHAR(100),               -- 'teller', 'regular', 'deposito'
    product         VARCHAR(50),                -- 'transfer', 'deposito', null
    value           JSONB NOT NULL,             -- numeric, string, object, array
    unit            VARCHAR(20),                -- 'IDR', 'percent', 'hours'
    scope           VARCHAR(50),                -- 'per_transaction', 'per_day', 'per_month'
    effective_from  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    effective_until TIMESTAMPTZ,
    version         INTEGER NOT NULL DEFAULT 1,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_by      UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, category, name, applies_to, applies_to_id, product, version)
);

-- Parameter history (audit trail)
CREATE TABLE parameter_history (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parameter_id    UUID NOT NULL REFERENCES parameters(id),
    org_id          UUID NOT NULL,
    previous_value  JSONB,
    new_value       JSONB,
    changed_by      UUID,
    changed_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

## 9. Versioning & Audit

Setiap perubahan parameter membuat record baru dengan version++:

```
v1: teller_transfer_max = 5.000.000 (created 2026-01-01)
v2: teller_transfer_max = 10.000.000 (updated 2026-04-01)
v3: teller_transfer_max = 15.000.000 (updated 2026-07-01)
```

- Query default selalu ambil version terbaru yang active
- History tersedia untuk audit
- Effective date memungkinkan schedule parameter changes

---

## 10. Open Questions

1. **Apakah policy7 perlu gRPC untuk low-latency query?**
   → v1.0: REST only, v2.0: pertimbangkan gRPC

2. **Apakah perlu caching (Redis) untuk hot parameters?**
   → v1.0: PostgreSQL dengan proper indexing, v1.1: Redis cache dengan TTL

3. **Apakah parameter perlu conditional (if-then)?**
   → v2.0: conditional parameters (e.g. limit berbeda di hari libur)

4. **Apakah policy7 perlu realtime notification ke consumers?**
   → v1.1: Redis pub/sub untuk parameter changes

---

*Next: Spec 01 — Architecture & API Detail (future)*
