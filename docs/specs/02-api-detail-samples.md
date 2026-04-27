# Policy7 — Sample Banking Cases

> **Tujuan**: Validasi kecukupan Spec API Policy7 dengan real-world banking scenarios
> **Date**: 2026-04-27

---

## Case 1: Teller Transaction Limit vs Authorization Limit

### Scenario
Teller "Siti" di KC Bandung ingin memproses transfer untuk nasabah sebesar Rp 75.000.000. 

**Simplified Logic (2 Limits Only):**
1. **Transaction Limit** = Rp 100.000.000 (maksimum nilai transaksi yang bisa diinput teller)
2. **Authorization Limit** = Rp 25.000.000 (batas auto otorisasi)

**Decision Flow:**
```
Amount ≤ Authorization Limit      → AUTO AUTHORIZED (langsung efektif)
Authorization < Amount ≤ Trans    → CAN INPUT, NEEDS AUTHORIZATION
Amount > Transaction Limit        → REJECTED (tidak bisa input)
```

**Contoh dengan Rp 75 jt:**
- Rp 75 jt ≤ Rp 25 jt? ❌ → Tidak auto authorized
- Rp 75 jt ≤ Rp 100 jt? ✅ → Bisa input, butuh otorisasi

### API Flow

**Step 1: Get teller limits (single call)**

```http
GET /v1/params/transaction_limit/teller_transfer_max?applies_to=role&applies_to_id=teller&product=transfer
Authorization: Bearer <siti_token>
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "uuid-1",
    "category": "transaction_limit",
    "name": "teller_transfer_max",
    "applies_to": "role",
    "applies_to_id": "teller",
    "product": "transfer",
    "transaction_limit": 100000000,
    "authorization_limit": 25000000,
    "unit": "IDR",
    "scope": "per_transaction"
  }
}
```

**Step 2: Validate transaction**

```http
POST /v1/params/transaction_limit/validate
Authorization: Bearer <siti_token>

{
  "amount": 75000000,
  "role": "teller",
  "product": "transfer"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "amount": 75000000,
    "decision": "REQUIRES_AUTHORIZATION",
    "can_input": true,
    "auto_authorized": false,
    "transaction_limit": {
      "max": 100000000,
      "remaining": 25000000
    },
    "authorization_limit": {
      "max": 25000000
    },
    "reason": "Amount exceeds authorization limit",
    "next_step": "Request supervisor authorization"
  }
}
```

### Business Outcome
⚠️ **Transaksi bisa diinput (Rp 75 jt ≤ Rp 100 jt), tapi butuh otorisasi** (karena Rp 75 jt > Rp 25 jt)

**Workflow:**
1. ✅ Siti bisa input transaksi Rp 75 jt
2. ⏸️ Transaksi status = "PENDING_AUTHORIZATION"
3. 📤 Supervisor menerima notifikasi untuk approve
4. ✅ Setelah di-approve, transaksi efektif

### Case 1b: Auto Authorized Transaction

**Scenario**: Transaksi Rp 15.000.000 (di bawah authorization limit)

```http
POST /v1/params/transaction_limit/validate
Authorization: Bearer <siti_token>

{
  "amount": 15000000,
  "role": "teller",
  "product": "transfer"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "amount": 15000000,
    "decision": "AUTO_AUTHORIZED",
    "can_input": true,
    "auto_authorized": true,
    "transaction_limit": {
      "max": 100000000,
      "remaining": 85000000
    },
    "authorization_limit": {
      "max": 25000000
    },
    "message": "Transaction automatically authorized"
  }
}
```

### Business Outcome
✅ **Transaksi langsung efektif** tanpa perlu otorisasi supervisor.

### Case 1c: Rejected Transaction (Exceeds Transaction Limit)

**Scenario**: Transaksi Rp 150.000.000 (melebihi transaction limit)

```http
POST /v1/params/transaction_limit/validate
Authorization: Bearer <siti_token>

{
  "amount": 150000000,
  "role": "teller",
  "product": "transfer"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "amount": 150000000,
    "decision": "REJECTED",
    "can_input": false,
    "auto_authorized": false,
    "transaction_limit": {
      "max": 100000000
    },
    "authorization_limit": {
      "max": 25000000
    },
    "reason": "Amount exceeds transaction limit",
    "suggestion": "Escalate to supervisor for processing"
  }
}
```

### Business Outcome
❌ **Transaksi tidak bisa diinput**. Siti harus minta supervisor untuk proses.

### Decision Matrix Summary

| Amount | vs Auth Limit | vs Trans Limit | Decision | Action |
|--------|---------------|----------------|----------|--------|
| Rp 15 jt | ≤ 25 jt ✅ | ≤ 100 jt ✅ | **AUTO_AUTHORIZED** | Langsung efektif |
| Rp 75 jt | > 25 jt ❌ | ≤ 100 jt ✅ | **REQUIRES_AUTHORIZATION** | Input + minta otorisasi |
| Rp 150 jt | > 25 jt ❌ | > 100 jt ❌ | **REJECTED** | Tidak bisa input |

---

## Case 2: Supervisor Authorization of Teller Transaction

### Scenario
Supervisor "Budi" menerima notifikasi untuk mengotorisasi transaksi Rp 75.000.000 yang diinput oleh Teller "Siti". 

**Context:**
- Teller Siti input transaksi Rp 75 jt (melebihi auto auth limit Rp 25 jt)
- Transaksi pending approval dari supervisor
- Supervisor Budi perlu cek:
  1. Apakah dia punya authority untuk otorisasi?
  2. Apakah ada limit otorisasi untuk supervisor?
  3. Berapa maksimum yang bisa diotorisasi oleh supervisor?

### API Flow

**Step 1: Get supervisor AUTHORIZATION limit (bukan approval threshold)**

```http
GET /v1/params/authorization_limit/supervisor_auth_max?applies_to=role&applies_to_id=supervisor&product=transfer
Authorization: Bearer <budi_token>
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "uuid-3",
    "category": "authorization_limit",
    "name": "supervisor_auth_max",
    "applies_to": "role",
    "applies_to_id": "supervisor",
    "product": "transfer",
    "value": 100000000,
    "unit": "IDR",
    "scope": "per_transaction",
    "purpose": "max_authorization_amount"
  }
}
```

**Step 2: Check if Budi can authorize Rp 75 jt**

```http
POST /v1/params/authorization_limit/check
Authorization: Bearer <budi_token>

{
  "amount": 75000000,
  "context": [
    { "type": "role", "id": "supervisor" },
    { "type": "product", "id": "transfer" }
  ]
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "allowed": true,
    "limit": 100000000,
    "requested": 75000000,
    "remaining_authority": 25000000,
    "message": "Supervisor can authorize this amount"
  }
}
```

### Business Outcome
✅ **Budi BISA mengotorisasi** transaksi Rp 75 jt (karena limit otorisasi supervisor = Rp 100 jt)

**Complete Workflow:**

```
┌─────────────────────────────────────────────────────────────┐
│                    TRANSACTION WORKFLOW                     │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Step 1: Teller Siti Input                                  │
│  ├── Amount: Rp 75.000.000                                  │
│  ├── Check: Rp 75jt ≤ Authorization Limit (25jt)? ❌        │
│  │   └── NOT AUTO-AUTHORIZED                                │
│  ├── Check: Rp 75jt ≤ Transaction Limit (100jt)? ✅         │
│  │   └── CAN INPUT                                          │
│  └── Status: PENDING_AUTHORIZATION                          │
│                    ↓                                        │
│  Step 2: System Check                                       │
│  ├── Query policy7 untuk supervisor auth limit              │
│  ├── Limit supervisor = Rp 100.000.000                      │
│  └── Rp 75jt ≤ 100jt? ✅ Butuh 1 level otorisasi            │
│                    ↓                                        │
│  Step 3: Notification to Supervisor Budi                    │
│  ├── workflow7 kirim task approval                          │
│  └── Budi membuka inbox otorisasi                           │
│                    ↓                                        │
│  Step 4: Budi Authorize                                     │
│  ├── Check policy7: Can supervisor auth Rp 75jt? ✅         │
│  ├── Budi approves via workflow7                            │
│  └── Transaksi menjadi EFEKTIF                              │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Case 2b: Transaction Exceeds Supervisor Authorization Limit

**Scenario:** Transaksi Rp 150.000.000 diinput oleh teller.

**API Check:**

```http
POST /v1/params/authorization_limit/check
Authorization: Bearer <budi_token>

{
  "amount": 150000000,
  "context": [
    { "type": "role", "id": "supervisor" },
    { "type": "product", "id": "transfer" }
  ]
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "allowed": false,
    "limit": 100000000,
    "requested": 150000000,
    "exceeded_by": 50000000,
    "message": "Amount exceeds supervisor authorization limit",
    "escalation_required": true,
    "next_approver": "branch_manager",
    "approval_chain": ["supervisor", "branch_manager"]
  }
}
```

### Business Outcome
❌ **Budi TIDAK bisa authorize sendiri**. Harus eskalasi ke Branch Manager.

**Multi-Level Approval Required:**
```
Teller Input → Supervisor Review → Branch Manager Final Approval
     Rp 150jt        Budi              Pak Dodi
                   (gagal auth     (punya limit
                    karena > 100jt)   500jt ✅)
```

### Parameter Types Summary

| Parameter Type | Teller | Supervisor | Branch Manager |
|----------------|--------|------------|----------------|
| **Transaction Limit** | Rp 100jt | Rp 500jt | Rp 1M |
| **Authorization Limit** | Rp 25jt | Rp 100jt | Rp 250jt |
| **Approver Limit** | ❌ (cannot auth) | Rp 100jt | Rp 500jt |

**Note:**
- **Transaction Limit** = Max amount yang bisa diinput oleh role tersebut
- **Authorization Limit** = Batas auto-otorisasi (di bawah ini langsung efektif)
- **Approver Limit** = Max amount yang bisa diotorisasi oleh approver

---

## Case 3: VIP Customer Higher Limits

### Scenario
Nasabah VIP "Pak Ahmad" ingin transfer Rp 100.000.000. Sebagai VIP, dia punya limit lebih tinggi.

### API Flow

**Step 1: Get effective limit dengan hierarchical resolution**

```http
GET /v1/params/transaction_limit/customer_transfer_max/effective
Authorization: Bearer <api_token>

Query params:
  context=customer_type:vip
  context=product:transfer
```

**Resolution Logic:**
1. Cari `customer_transfer_max` untuk `customer_type:VIP` + `product:transfer` → **FOUND**

**Response:**
```json
{
  "success": true,
  "data": {
    "parameter": {
      "id": "uuid-vip",
      "category": "transaction_limit",
      "name": "customer_transfer_max",
      "value": 200000000,
      "unit": "IDR",
      "scope": "per_transaction"
    },
    "resolved_from": {
      "applies_to": "customer_type",
      "applies_to_id": "vip",
      "product": "transfer"
    },
    "fallback_used": false
  }
}
```

**Step 2: Check if amount allowed**

```http
POST /v1/params/transaction_limit/customer_transfer_max/check
Authorization: Bearer <api_token>

{
  "value": 100000000,
  "context": [
    { "type": "customer_type", "id": "vip" },
    { "type": "product", "id": "transfer" }
  ]
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "allowed": true,
    "limit": 200000000,
    "remaining": 100000000,
    "message": "Amount within VIP limit"
  }
}
```

### Business Outcome
✅ Transaksi diperbolehkan karena Pak Ahmad adalah VIP dengan limit Rp 200M.

---

## Case 4: Deposito Interest Rate Calculation

### Scenario
Nasabah ingin membuka deposito 12 bulan. Teller perlu menampilkan suku bunga yang berlaku.

### API Flow

**Step 1: Get interest rate**

```http
GET /v1/params/rates/deposito?tenor=12m&amount=50000000
Authorization: Bearer <teller_token>
```

**Response:**
```json
{
  "success": true,
  "data": {
    "product": "deposito",
    "tenor": "12m",
    "rate": 4.5,
    "unit": "percent_per_year",
    "effective_date": "2026-04-01",
    "calculation_method": "simple_interest",
    "estimated_interest": 2250000,
    "estimated_total": 52250000
  }
}
```

### Business Outcome
✅ Teller bisa informasikan bunga 4.5% p.a. dan perkiraan hasil Rp 22.500.000.

---

## Case 5: Operational Hours Check

### Scenario
Teller "Rina" mencoba login pada hari Minggu jam 14:00. System perlu cek apakah teller boleh operasi.

### API Flow

**Step 1: Get operational hours**

```http
GET /v1/params/operational-hours?role=teller&date=2026-04-27
Authorization: Bearer <rina_token>
```

**Response:**
```json
{
  "success": true,
  "data": {
    "role": "teller",
    "date": "2026-04-27",
    "is_working_day": false,
    "day_type": "weekend",
    "hours": null,
    "message": "Teller operations not available on weekends"
  }
}
```

### Business Outcome
❌ Rina tidak bisa melakukan transaksi teller pada hari Minggu.

---

## Case 6: Fee Calculation for Transfer

### Scenario
Nasabah transfer Rp 10.000.000 dari ATM. System perlu hitung biaya admin.

### API Flow

**Step 1: Get transfer fee**

```http
GET /v1/params/fees/transfer?channel=ATM&amount=10000000&customer_type=regular
Authorization: Bearer <api_token>
```

**Response:**
```json
{
  "success": true,
  "data": {
    "product": "transfer",
    "channel": "ATM",
    "customer_type": "regular",
    "fee_type": "flat",
    "fee": 6500,
    "unit": "IDR",
    "min_amount": null,
    "max_amount": null,
    "effective_date": "2026-01-01"
  }
}
```

### Business Outcome
✅ Biaya admin Rp 6.500 akan dikenakan.

---

## Case 7: Admin Changes Parameter with Versioning

### Scenario
Bank Indonesia mengeluarkan aturan baru: limit teller harus diturunkan dari Rp 10M menjadi Rp 5M. Admin "Pak Dodi" melakukan perubahan.

### API Flow

**Step 1: Find current parameter**

```http
GET /admin/v1/params?category=transaction_limit&name=teller_transfer_max&applies_to=role
Authorization: Bearer <admin_token>
```

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": "uuid-current",
      "category": "transaction_limit",
      "name": "teller_transfer_max",
      "value": 10000000,
      "version": 3,
      "is_active": true,
      "created_at": "2026-04-01T00:00:00Z"
    }
  ]
}
```

**Step 2: Update parameter (creates new version)**

```http
PUT /admin/v1/params/uuid-current
Authorization: Bearer <admin_token>

{
  "value": 5000000,
  "effective_from": "2026-05-01T00:00:00Z",
  "change_reason": "BI Regulation 2026/04: Reduced teller transaction limits for fraud prevention"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "uuid-new",
    "category": "transaction_limit",
    "name": "teller_transfer_max",
    "value": 5000000,
    "version": 4,
    "is_active": true,
    "previous_version_id": "uuid-current",
    "effective_from": "2026-05-01T00:00:00Z",
    "created_by": "admin-dodi",
    "created_at": "2026-04-27T10:00:00Z"
  }
}
```

**Step 3: View history**

```http
GET /admin/v1/params/uuid-new/history
Authorization: Bearer <admin_token>
```

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "version": 4,
      "value": 5000000,
      "effective_from": "2026-05-01T00:00:00Z",
      "change_reason": "BI Regulation 2026/04: Reduced teller transaction limits for fraud prevention"
    },
    {
      "version": 3,
      "value": 10000000,
      "effective_from": "2026-04-01T00:00:00Z",
      "effective_until": "2026-05-01T00:00:00Z",
      "change_reason": "Annual policy review Q1 2026"
    }
  ]
}
```

### Business Outcome
✅ Perubahan tercatat dengan:
- Version baru (v4)
- Effective date masa depan (1 Mei 2026)
- Audit trail lengkap dengan reason
- Version lama (v3) tetap aktif sampai 1 Mei

---

## Case 8: Bulk Import New Tariffs

### Scenario
Akhir tahun, bank mengupdate semua tarif transfer untuk 2027. Admin perlu import 50+ parameter baru.

### API Flow

**Step 1: Upload CSV file**

```http
POST /admin/v1/params/bulk-import
Authorization: Bearer <admin_token>
Content-Type: multipart/form-data

file: tariffs_2027.csv
options: {
  "skip_validation": false,
  "update_existing": true,
  "dry_run": true
}
```

**CSV Content:**
```csv
category,name,applies_to,applies_to_id,product,value,value_type,unit,scope,effective_from,description
fee,transfer_atm_flat,customer_type,regular,transfer,7500,number,IDR,per_transaction,2027-01-01,Tarif transfer ATM 2027
fee,transfer_atm_flat,customer_type,vip,transfer,0,number,IDR,per_transaction,2027-01-01,Tarif transfer ATM VIP free
fee,transfer_teller_flat,customer_type,regular,transfer,15000,number,IDR,per_transaction,2027-01-01,Tarif transfer Teller 2027
...
```

**Response (dry-run):**
```json
{
  "success": true,
  "data": {
    "dry_run": true,
    "would_import": 45,
    "would_update": 5,
    "would_skip": 0,
    "errors": []
  }
}
```

**Step 2: Execute actual import**

```http
POST /admin/v1/params/bulk-import
Authorization: Bearer <admin_token>
Content-Type: multipart/form-data

file: tariffs_2027.csv
options: {
  "dry_run": false
}
```

### Business Outcome
✅ 50 parameter berhasil diupdate dalam sekali proses.

---

## Case 9: CTR/STR Regulatory Threshold

### Scenario
Nasabah melakukan transaksi cash Rp 125.000.000. System perlu cek apakah perlu lapor CTR (Currency Transaction Report).

### API Flow

**Step 1: Get CTR threshold**

```http
GET /v1/params/regulatory/ctr_threshold?country=ID
Authorization: Bearer <api_token>
```

**Response:**
```json
{
  "success": true,
  "data": {
    "regulatory_type": "CTR",
    "country": "ID",
    "threshold": 100000000,
    "unit": "IDR",
    "currency": "IDR",
    "reporting_deadline_hours": 24,
    "authority": "PPATK"
  }
}
```

**Step 2: Check transaction against threshold**

```http
POST /v1/params/regulatory/ctr_threshold/check
Authorization: Bearer <api_token>

{
  "value": 125000000,
  "context": [
    { "type": "country", "id": "ID" }
  ]
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "requires_reporting": true,
    "threshold": 100000000,
    "exceeded_by": 25000000,
    "report_type": "CTR",
    "deadline": "2026-04-28T14:30:00Z",
    "authority": "PPATK"
  }
}
```

### Business Outcome
🚨 CTR report wajib dibuat dalam 24 jam.

---

## Case 10: Product Access Control

### Scenario
Teller baru "Andi" mencoba akses menu Deposito, tapi system perlu cek apakah role teller boleh akses produk tersebut.

### API Flow

**Step 1: Get product access rules**

```http
GET /v1/params/product_access?role=teller&product=deposito
Authorization: Bearer <andi_token>
```

**Response:**
```json
{
  "success": true,
  "data": {
    "role": "teller",
    "product": "deposito",
    "can_view": true,
    "can_create": false,
    "can_update": false,
    "can_delete": false,
    "can_approve": false,
    "message": "Teller can view deposito but cannot create or modify"
  }
}
```

### Business Outcome
⚠️ Andi bisa melihat data deposito tapi tidak bisa membuat deposito baru (harus supervisor/CS).

---

## API Coverage Summary

| Case | Public API | Admin API | Notes |
|------|------------|-----------|-------|
| 1. Teller Limits | ✅ GET /v1/params<br>✅ POST /check | — | Basic limit checking |
| 2. Approval Workflow | ✅ GET /approval-thresholds | — | Role-based approval |
| 3. VIP Customer | ✅ GET /effective<br>✅ POST /check | — | Hierarchical resolution |
| 4. Interest Rates | ✅ GET /rates | — | Specialized endpoint |
| 5. Operational Hours | ✅ GET /operational-hours | — | Time-based access |
| 6. Fee Calculation | ✅ GET /fees | — | Product + channel |
| 7. Versioning | — | ✅ PUT /admin<br>✅ GET /history | Audit trail |
| 8. Bulk Import | — | ✅ POST /bulk-import | Mass update |
| 9. Regulatory | ✅ GET /regulatory<br>✅ POST /check | — | Compliance |
| 10. Product Access | ✅ GET /product_access | — | Feature flags |

---

## Identified Gaps & Recommendations

### ✅ Well Covered
- Basic CRUD operations
- Hierarchical parameter resolution
- Versioning & audit trail
- Bulk import

### 🔍 Potential Additions

| # | Gap | Recommendation |
|---|-----|----------------|
| 1 | **Daily usage tracking** | Case 1 memerlukan tracking usage per teller per hari. Perlu integration dengan transaction service atau cache counter. |
| 2 | **Multi-currency support** | Case 4 & 6 menggunakan IDR. Perlu pertimbangkan foreign currency parameters. |
| 3 | **Conditional parameters** | "Limit lebih rendah di hari libur" — perlu conditional logic di Spec 01 sudah approved untuk v1.0. |
| 4 | **Real-time notifications** | Parameter changes perlu notify services (NATS) — sudah covered di Spec 01 hybrid model. |
| 5 | **Parameter dependencies** | "Jika limit A berubah, limit B juga berubah" — bisa di-handle di client/service layer. |

### 📋 Spec 02 Validation Result

**Status**: ✅ **SUFFICIENT for v1.0**

Semua banking cases utama dapat dihandle dengan API yang sudah didefinisikan. Tidak ada gap kritis yang memerlukan perubahan spec.

---

*Validasi selesai: Spec 02 mencukupi untuk real-world banking scenarios*
