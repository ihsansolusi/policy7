# Policy7 — Spec 02: API Detail

> **Versi**: 0.1-draft | **Tanggal**: 2026-04-27 | **Fase**: Review

---

## 1. API Overview

Policy7 menyediakan dua jenis API:

| API Type | Prefix | Purpose | Auth |
|----------|--------|---------|------|
| **Public API** | `/v1/` | Query parameter (read-only) | JWT atau API Key |
| **Admin API** | `/admin/v1/` | CRUD parameter | JWT (admin role) |

### 1.1 Base URL

```
Development: http://localhost:8080
Production:  https://policy7.core7.internal
```

### 1.2 Content Type

```
Content-Type: application/json
```

### 1.3 Response Format

Semua response menggunakan format:

```json
{
  "success": true,
  "data": { ... },
  "meta": { ... }     // untuk list endpoints
}
```

Error response:

```json
{
  "success": false,
  "error": {
    "code": "PARAMETER_NOT_FOUND",
    "message": "Parameter not found",
    "details": { ... }
  }
}
```

---

## 2. Authentication

### 2.1 JWT Bearer Token

```
Authorization: Bearer <jwt_token>
```

Token harus mengandung claims:
- `sub` — User ID
- `org_id` — Organization ID
- `branch_id` — Branch ID (optional)
- `roles` — Array of roles

### 2.2 Service-to-Service API Key

```
X-API-Key: <service_api_key>
X-Org-ID: <org_uuid>
```

Digunakan oleh internal services (workflow7, notif7, dll).

---

## 3. Public API Endpoints

### 3.1 Get Parameter by Category & Name

Mengambil single parameter value.

```
GET /v1/params/:category/:name
Authorization: Bearer <token>

Query params:
  applies_to=role           (optional: role|customer_type|product|global)
  applies_to_id=teller      (optional)
  product=transfer          (optional)
```

**Response:**

```json
{
  "success": true,
  "data": {
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
    "version": 3,
    "is_active": true
  }
}
```

**Error Responses:**
- `404` — Parameter not found
- `403` — Forbidden (tenant mismatch)
- `410` — Parameter expired or inactive

### 3.2 Get Parameters by Category

Mengambil semua parameter dalam satu category.

```
GET /v1/params/:category
Authorization: Bearer <token>

Query params:
  applies_to=role           (optional filter)
  applies_to_id=teller      (optional filter)
  product=transfer          (optional filter)
  active_only=true          (default: true)
```

**Response:**

```json
{
  "success": true,
  "data": [
    {
      "id": "uuid",
      "category": "transaction_limit",
      "name": "teller_transfer_max",
      "applies_to": "role",
      "applies_to_id": "teller",
      "product": "transfer",
      "value": 10000000,
      "unit": "IDR",
      "scope": "per_transaction"
    },
    {
      "id": "uuid",
      "category": "transaction_limit",
      "name": "teller_daily_limit",
      "applies_to": "role",
      "applies_to_id": "teller",
      "product": null,
      "value": 50000000,
      "unit": "IDR",
      "scope": "per_day"
    }
  ]
}
```

### 3.3 Get Effective Parameter (Hierarchical)

Mengambil parameter dengan hierarchical resolution (global → org → role → user).

```
GET /v1/params/:category/:name/effective
Authorization: Bearer <token>

Query params:
  context=role:teller         (required context)
  context=product:transfer    (optional additional context)
  context=customer_type:vip   (optional additional context)
```

**Resolution Logic:**
1. Cari parameter spesifik (role=teller, product=transfer)
2. Jika tidak ada, cari parameter product=transfer (tanpa role)
3. Jika tidak ada, cari parameter role=teller (tanpa product)
4. Jika tidak ada, cari parameter global
5. Return default jika tidak ada yang cocok

**Response:**

```json
{
  "success": true,
  "data": {
    "parameter": {
      "id": "uuid",
      "category": "transaction_limit",
      "name": "teller_transfer_max",
      "value": 10000000,
      "unit": "IDR",
      "scope": "per_transaction"
    },
    "resolved_from": {
      "applies_to": "role",
      "applies_to_id": "teller",
      "product": "transfer"
    },
    "fallback_used": false
  }
}
```

### 3.4 Query Parameters by Context

Batch query untuk multiple parameter sekaligus.

```
POST /v1/params/query
Authorization: Bearer <token>

{
  "contexts": [
    { "type": "role", "id": "teller" },
    { "type": "product", "id": "transfer" }
  ],
  "categories": ["transaction_limit", "approval_threshold"],
  "names": ["teller_transfer_max", "supervisor_approval_min"]
}
```

**Response:**

```json
{
  "success": true,
  "data": {
    "transaction_limit": {
      "teller_transfer_max": {
        "value": 10000000,
        "unit": "IDR",
        "resolved_from": { "applies_to": "role", "applies_to_id": "teller" }
      }
    },
    "approval_threshold": {
      "supervisor_approval_min": {
        "value": 50000000,
        "unit": "IDR",
        "resolved_from": { "applies_to": "role", "applies_to_id": "supervisor" }
      }
    }
  }
}
```

### 3.5 Check Parameter Condition

Validasi apakah suatu nilai memenuhi parameter condition.

```
POST /v1/params/:category/:name/check
Authorization: Bearer <token>

{
  "value": 15000000,
  "context": [
    { "type": "role", "id": "teller" },
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
    "limit": 10000000,
    "exceeded_by": 5000000,
    "message": "Amount exceeds teller transfer limit"
  }
}
```

### 3.6 Transaction Validation (Transaction Limit + Authorization Limit)

Validasi transaksi dengan **2 limit**:
1. **Transaction Limit** — Maksimum nilai yang bisa diinput oleh teller
2. **Authorization Limit** — Batas auto otorisasi (di bawah ini langsung efektif)

```
POST /v1/params/transaction_limit/validate
Authorization: Bearer <token>

{
  "amount": 75000000,
  "role": "teller",
  "product": "transfer"
}
```

**Response (Requires Authorization):**

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

**Response (Auto Authorized):**

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

**Response (Rejected):**

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
    "reason": "Amount exceeds transaction limit"
  }
}
```

**Decision Matrix:**

| Amount | Condition | Decision | Can Input | Auto Authorized |
|--------|-----------|----------|-----------|-----------------|
| Rp 15 jt | ≤ Auth Limit (25jt) | **AUTO_AUTHORIZED** | ✅ | ✅ |
| Rp 75 jt | > Auth, ≤ Trans (100jt) | **REQUIRES_AUTHORIZATION** | ✅ | ❌ |
| Rp 150 jt | > Trans Limit (100jt) | **REJECTED** | ❌ | ❌ |

### 3.7 Approver Authorization Limit Check

Cek apakah approver (supervisor, branch manager) punya authority untuk mengotorisasi amount tertentu.

```
POST /v1/params/authorization_limit/check
Authorization: Bearer <token>

{
  "amount": 75000000,
  "approver_role": "supervisor",
  "product": "transfer"
}
```

**Response (Allowed):**

```json
{
  "success": true,
  "data": {
    "allowed": true,
    "approver_limit": 100000000,
    "requested": 75000000,
    "remaining": 25000000,
    "message": "Supervisor can authorize this amount"
  }
}
```

**Response (Exceeded - Requires Escalation):**

```json
{
  "success": true,
  "data": {
    "allowed": false,
    "approver_limit": 100000000,
    "requested": 150000000,
    "exceeded_by": 50000000,
    "message": "Amount exceeds supervisor authorization limit",
    "escalation_required": true,
    "next_approver": "branch_manager"
  }
}
```

---

## 4. Admin API Endpoints

### 4.1 List Parameters

```
GET /admin/v1/params
Authorization: Bearer <token>

Query params:
  org_id=uuid               (required untuk org_admin)
  category=transaction_limit (optional filter)
  name=teller               (optional search by name)
  applies_to=role           (optional filter)
  is_active=true            (optional filter)
  page=1
  limit=20
  sort=created_at:desc
```

**Response:**

```json
{
  "success": true,
  "data": [
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
      "version": 3,
      "is_active": true,
      "created_by": "admin-uuid",
      "created_at": "2026-01-01T00:00:00Z",
      "updated_at": "2026-04-01T00:00:00Z"
    }
  ],
  "meta": {
    "total": 150,
    "page": 1,
    "limit": 20,
    "total_pages": 8
  }
}
```

### 4.2 Get Single Parameter

```
GET /admin/v1/params/:id
Authorization: Bearer <token>
```

**Response:** (sama dengan list item)

### 4.3 Create Parameter

```
POST /admin/v1/params
Authorization: Bearer <token>

{
  "category": "transaction_limit",
  "name": "teller_transfer_max",
  "applies_to": "role",
  "applies_to_id": "teller",
  "product": "transfer",
  "value": 10000000,
  "value_type": "number",
  "unit": "IDR",
  "scope": "per_transaction",
  "effective_from": "2026-01-01T00:00:00Z",
  "effective_until": null,
  "description": "Maximum transfer amount for teller role",
  "metadata": {
    "department": "operations",
    "review_cycle": "quarterly"
  }
}
```

**Validation:**
- `category`: required, max 50 chars
- `name`: required, max 100 chars, alphanumeric + underscore
- `applies_to`: required, enum [role, customer_type, product, global]
- `value`: required, type sesuai `value_type`
- `effective_from`: default NOW() jika tidak diisi
- Unique constraint: `(org_id, category, name, applies_to, applies_to_id, product)` untuk version aktif

**Response:**

```json
{
  "success": true,
  "data": {
    "id": "new-uuid",
    "category": "transaction_limit",
    "name": "teller_transfer_max",
    "value": 10000000,
    "version": 1,
    "is_active": true,
    "created_by": "admin-uuid",
    "created_at": "2026-04-27T10:00:00Z"
  }
}
```

**Error Responses:**
- `409` — Parameter already exists (duplicate unique constraint)
- `400` — Invalid value type or format

### 4.4 Update Parameter

Update membuat version baru (tidak mengubah record existing).

```
PUT /admin/v1/params/:id
Authorization: Bearer <token>

{
  "value": 15000000,
  "unit": "IDR",
  "effective_from": "2026-05-01T00:00:00Z",
  "effective_until": null,
  "description": "Updated limit for teller transfers",
  "change_reason": "Annual policy review Q2 2026"
}
```

**Notes:**
- `change_reason` — Wajib untuk audit trail
- Tidak bisa ubah: `category`, `name`, `applies_to`, `applies_to_id`, `product`
- Record lama di-soft-delete (is_active = false)
- Record baru dibuat dengan version = old_version + 1

**Response:**

```json
{
  "success": true,
  "data": {
    "id": "new-uuid",
    "category": "transaction_limit",
    "name": "teller_transfer_max",
    "value": 15000000,
    "version": 4,
    "is_active": true,
    "previous_version_id": "old-uuid",
    "created_by": "admin-uuid",
    "created_at": "2026-04-27T10:30:00Z"
  }
}
```

### 4.5 Soft Delete Parameter

```
DELETE /admin/v1/params/:id
Authorization: Bearer <token>

{
  "reason": "Parameter no longer needed after policy change"
}
```

**Response:**

```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "deleted_at": "2026-04-27T10:35:00Z",
    "deleted_by": "admin-uuid"
  }
}
```

### 4.6 Get Parameter History

```
GET /admin/v1/params/:id/history
Authorization: Bearer <token>

Query params:
  page=1
  limit=20
```

**Response:**

```json
{
  "success": true,
  "data": [
    {
      "version": 3,
      "value": 10000000,
      "effective_from": "2026-04-01T00:00:00Z",
      "effective_until": "2026-05-01T00:00:00Z",
      "is_active": false,
      "created_by": "admin-uuid",
      "created_at": "2026-04-01T00:00:00Z",
      "change_reason": "Annual policy review Q1 2026"
    },
    {
      "version": 2,
      "value": 5000000,
      "effective_from": "2026-02-01T00:00:00Z",
      "effective_until": "2026-04-01T00:00:00Z",
      "is_active": false,
      "created_by": "admin-uuid",
      "created_at": "2026-02-01T00:00:00Z",
      "change_reason": "Emergency limit reduction"
    },
    {
      "version": 1,
      "value": 5000000,
      "effective_from": "2026-01-01T00:00:00Z",
      "effective_until": "2026-02-01T00:00:00Z",
      "is_active": false,
      "created_by": "admin-uuid",
      "created_at": "2026-01-01T00:00:00Z",
      "change_reason": "Initial parameter creation"
    }
  ]
}
```

### 4.7 Bulk Import Parameters

```
POST /admin/v1/params/bulk-import
Authorization: Bearer <token>
Content-Type: multipart/form-data

file: <csv_or_json_file>
options: {
  "skip_validation": false,
  "update_existing": true,
  "dry_run": false
}
```

**CSV Format:**

```csv
category,name,applies_to,applies_to_id,product,value,value_type,unit,scope,effective_from,description
transaction_limit,teller_transfer_max,role,teller,transfer,10000000,number,IDR,per_transaction,2026-01-01,Max transfer for tellers
transaction_limit,teller_daily_limit,role,teller,,50000000,number,IDR,per_day,2026-01-01,Daily limit for tellers
```

**Response:**

```json
{
  "success": true,
  "data": {
    "imported": 45,
    "updated": 5,
    "skipped": 2,
    "errors": [
      {
        "row": 12,
        "field": "value",
        "error": "Invalid number format"
      }
    ]
  }
}
```

---

## 5. Special Parameter Types

### 5.1 Interest Rates

```
GET /v1/params/rates/:product
Authorization: Bearer <token>

Query params:
  tenor=12m
  amount=10000000
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
    "calculation_method": "simple_interest"
  }
}
```

### 5.2 Operational Hours

```
GET /v1/params/operational-hours
Authorization: Bearer <token>

Query params:
  role=teller
  date=2026-04-27
```

**Response:**

```json
{
  "success": true,
  "data": {
    "role": "teller",
    "date": "2026-04-27",
    "is_working_day": true,
    "hours": {
      "start": "08:00",
      "end": "16:00",
      "timezone": "Asia/Jakarta"
    },
    "breaks": [
      { "start": "12:00", "end": "13:00" }
    ],
    "cutoff_time": "15:30"
  }
}
```

### 5.3 Approval Thresholds

```
GET /v1/params/approval-thresholds
Authorization: Bearer <token>

Query params:
  role=teller
  product=transfer
  amount=100000000
```

**Response:**

```json
{
  "success": true,
  "data": {
    "role": "teller",
    "product": "transfer",
    "amount": 100000000,
    "requires_approval": true,
    "approver_roles": ["supervisor", "branch_manager"],
    "approval_level": 2,
    "next_approval_threshold": 500000000
  }
}
```

### 5.4 Authorization Limits (Approver Limits)

Maksimum amount yang bisa diotorisasi oleh approver (supervisor, branch manager). Berbeda dengan approval thresholds yang menentukan SIAPA yang perlu approve, authorization limits menentukan BERAPA yang BISA diapprove oleh masing-masing approver.

```
GET /v1/params/authorization_limits
Authorization: Bearer <token>

Query params:
  role=supervisor
  product=transfer
```

**Response:**

```json
{
  "success": true,
  "data": {
    "role": "supervisor",
    "product": "transfer",
    "authorization_max": 100000000,
    "unit": "IDR",
    "scope": "per_transaction",
    "daily_authorization_max": 500000000,
    "effective_date": "2026-01-01"
  }
}
```

**Parameter Types:**

| Parameter Type | Description | Example |
|----------------|-------------|---------|
| **Transaction Limit** | Max amount yang bisa diinput oleh role tersebut | Teller: Rp 100jt |
| **Authorization Limit** | Threshold auto-otorisasi | Teller: Rp 25jt |
| **Approver Limit** | Max amount yang bisa diotorisasi oleh approver | Supervisor: Rp 100jt, BM: Rp 500jt |

**Two-Limit Pattern (Teller):**

```
┌─────────────────────────────────────────────────────────────┐
│                    TELLER WORKFLOW                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Amount: Rp 75.000.000                                      │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ AUTHORIZATION LIMIT (Rp 25jt)                       │   │
│  │ Rp 75jt ≤ Rp 25jt? ❌ NOT AUTO-AUTHORIZED           │   │
│  └─────────────────────────────────────────────────────┘   │
│                          ↓                                  │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ TRANSACTION LIMIT (Rp 100jt)                        │   │
│  │ Rp 75jt ≤ Rp 100jt? ✅ CAN INPUT                    │   │
│  │ Status: NEEDS SUPERVISOR AUTHORIZATION              │   │
│  └─────────────────────────────────────────────────────┘   │
│                          ↓                                  │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ SUPERVISOR AUTH LIMIT (Rp 100jt)                    │   │
│  │ Rp 75jt ≤ Rp 100jt? ✅ CAN AUTHORIZE                │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘

**Simplified Decision:**
• Amount ≤ Auth Limit     → AUTO AUTHORIZED
• Auth < Amount ≤ Trans   → REQUIRES AUTHORIZATION  
• Amount > Trans Limit    → REJECTED
```

---

## 6. Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `INVALID_REQUEST` | 400 | Request format invalid |
| `MISSING_REQUIRED_FIELD` | 400 | Required field tidak diisi |
| `INVALID_VALUE_TYPE` | 400 | Value type tidak sesuai |
| `UNAUTHORIZED` | 401 | Token invalid atau expired |
| `FORBIDDEN` | 403 | Tidak punya permission |
| `PARAMETER_NOT_FOUND` | 404 | Parameter tidak ditemukan |
| `PARAMETER_EXISTS` | 409 | Parameter sudah ada (duplicate) |
| `PARAMETER_EXPIRED` | 410 | Parameter sudah expired |
| `CACHE_UNAVAILABLE` | 503 | Cache service unavailable |
| `RATE_LIMIT_EXCEEDED` | 429 | Too many requests |

---

## 7. Rate Limiting

### 7.1 Rate Limit Tiers

| Tier | Endpoint Type | Limit | Burst | Window |
|------|---------------|-------|-------|--------|
| **Public Read** | GET /v1/params/* | 1,000 req/min | 50 req | 60 seconds |
| **Public Query** | POST /v1/params/query | 500 req/min | 25 req | 60 seconds |
| **Admin Read** | GET /admin/v1/params | 100 req/min | 10 req | 60 seconds |
| **Admin Write** | POST/PUT/DELETE /admin/* | 30 req/min | 5 req | 60 seconds |
| **Bulk Import** | POST /admin/v1/params/bulk-import | 1 req/min | 1 req | 60 seconds |
| **Health Check** | GET /health | No limit | - | - |

### 7.2 Rate Limit Headers

Semua response includes rate limiting headers:

```http
HTTP/1.1 200 OK
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1714209600
X-RateLimit-Policy: 1000;w=60;burst=50
```

| Header | Description | Example |
|--------|-------------|---------|
| `X-RateLimit-Limit` | Maximum requests allowed per window | 1000 |
| `X-RateLimit-Remaining` | Remaining requests in current window | 999 |
| `X-RateLimit-Reset` | Unix timestamp when limit resets | 1714209600 |
| `X-RateLimit-Policy` | Rate limit policy (RFC 6585) | 1000;w=60;burst=50 |

### 7.3 Rate Limit Exceeded Response

When rate limit is exceeded:

```http
HTTP/1.1 429 Too Many Requests
Retry-After: 45
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1714209645
Content-Type: application/json

{
  "success": false,
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "Rate limit exceeded. Please retry after 45 seconds.",
    "details": {
      "limit": 1000,
      "window": "60s",
      "retry_after": 45
    }
  }
}
```

### 7.4 Rate Limiting Implementation

**Architecture:**
```
Client Request
     ↓
API Gateway (nginx) — Rate limiting layer
     ↓
Redis (rate counter) — Distributed tracking
     ↓
Policy7 Service
```

**Redis Key Pattern:**
```
rate_limit:{tier}:{identifier}:{window}

Examples:
rate_limit:public_read:apikey_abc123:2024042710
rate_limit:admin_write:user_xyz789:202404271000
```

**Algorithm: Token Bucket**
```go
// Simplified token bucket implementation
func checkRateLimit(ctx context.Context, tier string, identifier string) (*RateLimitStatus, error) {
    key := fmt.Sprintf("rate_limit:%s:%s:%s", tier, identifier, currentWindow())
    
    // Get current count
    count, err := redis.Get(ctx, key)
    if err == redis.Nil {
        // First request in window
        redis.SetEx(ctx, key, 1, windowDuration)
        return &RateLimitStatus{Allowed: true, Remaining: limit - 1}, nil
    }
    
    // Check limit
    currentCount, _ := strconv.Atoi(count)
    if currentCount >= limit {
        return &RateLimitStatus{Allowed: false, Remaining: 0}, ErrRateLimitExceeded
    }
    
    // Increment counter
    redis.Incr(ctx, key)
    return &RateLimitStatus{Allowed: true, Remaining: limit - currentCount - 1}, nil
}
```

### 7.5 Client Classification

| Client Type | Identifier | Rate Limit Tier |
|-------------|------------|-----------------|
| **Public API Key** | API Key (X-API-Key header) | public_read / public_query |
| **Authenticated User** | User ID (from JWT) | admin_read / admin_write |
| **Service-to-Service** | Service ID (X-Service-ID header) | public_read |
| **Internal Service** | Service ID + IP whitelist | Higher limits |

### 7.6 Burst Handling

**Burst Definition:** Short spike of requests that exceeds steady rate but within burst capacity.

```
Normal: 1000 req/min = ~16.67 req/sec
Burst: 50 req allowed in 1 second

Example:
- Second 1: 50 requests (burst) → Allowed
- Second 2: 5 requests → Allowed
- Second 3: 20 requests → 11 blocked (only 9 remaining in window)
```

**Burst Headers:**
```http
X-RateLimit-Burst-Limit: 50
X-RateLimit-Burst-Remaining: 35
```

### 7.7 Whitelist & Exemptions

| Scenario | Handling |
|----------|----------|
| Health check endpoints | No rate limiting |
| Internal service (localhost) | No rate limiting |
| Emergency override | Admin API to temporarily whitelist IP |
| Bulk operations | Use bulk import endpoint (1 req/min) |

### 7.8 Monitoring & Alerting

| Metric | Threshold | Alert |
|--------|-----------|-------|
| Rate limit hits per minute | > 100 | Warning |
| Rate limit hits per minute | > 500 | Critical |
| Burst capacity exhausted | > 80% | Warning |

---

## 8. Pagination Specification

### 8.1 Pagination Model: Offset-Based

Policy7 menggunakan **offset-based pagination** untuk simplicity. Cursor-based akan dipertimbangkan untuk v1.1 jika dataset > 100K records.

### 8.2 Request Parameters

```http
GET /admin/v1/params?page=2&limit=50&sort=created_at:desc
```

| Parameter | Type | Default | Min | Max | Description |
|-----------|------|---------|-----|-----|-------------|
| `page` | integer | 1 | 1 | - | Page number (1-indexed) |
| `limit` | integer | 20 | 1 | 100 | Items per page |
| `sort` | string | `created_at:desc` | - | - | Sort field and direction |

### 8.3 Sort Syntax

```
sort={field}:{direction}

Examples:
sort=created_at:desc          # Newest first
sort=created_at:asc           # Oldest first
sort=name:asc                 # Alphabetical
sort=category:asc,name:asc    # Multiple fields
```

**Valid sort fields:**
- `created_at`
- `updated_at`
- `name`
- `category`
- `version`
- `effective_from`

### 8.4 Response Format

```json
{
  "success": true,
  "data": [
    { ... parameter object ... },
    { ... parameter object ... }
  ],
  "meta": {
    "pagination": {
      "page": 2,
      "limit": 50,
      "total": 847,
      "total_pages": 17,
      "has_next": true,
      "has_prev": true,
      "next_page": 3,
      "prev_page": 1
    },
    "sort": {
      "field": "created_at",
      "direction": "desc"
    }
  }
}
```

### 8.5 Pagination Metadata

| Field | Type | Description |
|-------|------|-------------|
| `page` | integer | Current page number |
| `limit` | integer | Items per page |
| `total` | integer | Total items matching query |
| `total_pages` | integer | Total pages (ceil(total/limit)) |
| `has_next` | boolean | Is there a next page? |
| `has_prev` | boolean | Is there a previous page? |
| `next_page` | integer | Next page number (null if no next) |
| `prev_page` | integer | Previous page number (null if no prev) |

### 8.6 Edge Cases

**Page exceeds total:**
```http
GET /admin/v1/params?page=100&limit=20  # When total=50

Response:
{
  "success": true,
  "data": [],
  "meta": {
    "pagination": {
      "page": 100,
      "limit": 20,
      "total": 50,
      "total_pages": 3,
      "has_next": false,
      "has_prev": true,
      "next_page": null,
      "prev_page": 99
    }
  }
}
```

**Limit exceeds maximum:**
```http
GET /admin/v1/params?limit=500  # Max is 100

Response: 400 Bad Request
{
  "success": false,
  "error": {
    "code": "INVALID_LIMIT",
    "message": "Limit cannot exceed 100"
  }
}
```

### 8.7 Performance Considerations

**Query Optimization:**
```sql
-- Efficient pagination with LIMIT/OFFSET
SELECT * FROM parameters
WHERE org_id = $1 AND is_active = true
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- Offset calculation: (page - 1) * limit
-- Page 2, Limit 50: OFFSET 100
```

**Note:** Offset-based pagination has O(n) complexity for large offsets. For tables > 100K rows, consider:
- Keyset pagination (cursor-based) v1.1
- Search filters to reduce result set
- Archive old records

---

## 9. Bulk Import Error Handling

### 9.1 Import Modes

```http
POST /admin/v1/params/bulk-import
Content-Type: multipart/form-data

file: parameters.csv
options: {
  "mode": "strict",        // "strict" | "lenient" | "dry_run"
  "skip_validation": false,
  "update_existing": true,
  "continue_on_error": false,
  "batch_size": 100
}
```

| Mode | Behavior |
|------|----------|
| **strict** | Stop on first error, rollback all changes |
| **lenient** | Continue processing, report all errors at end |
| **dry_run** | Validate only, no changes persisted |

### 9.2 Success Response

```json
{
  "success": true,
  "data": {
    "imported": 45,
    "updated": 5,
    "skipped": 2,
    "errors": [],
    "processing_time_ms": 1250,
    "batches": 1
  }
}
```

### 9.3 Partial Success Response (Lenient Mode)

```json
{
  "success": true,
  "data": {
    "imported": 42,
    "updated": 3,
    "skipped": 2,
    "failed": 3,
    "errors": [
      {
        "row": 12,
        "field": "value",
        "error_code": "INVALID_VALUE_TYPE",
        "error_message": "Expected number, got string",
        "raw_data": {
          "category": "transaction_limit",
          "name": "teller_max",
          "value": "invalid"
        }
      },
      {
        "row": 28,
        "field": "category",
        "error_code": "INVALID_CATEGORY",
        "error_message": "Category 'unknown_category' not found",
        "raw_data": { ... }
      },
      {
        "row": 45,
        "field": null,
        "error_code": "DUPLICATE_KEY",
        "error_message": "Parameter already exists with same key",
        "raw_data": { ... }
      }
    ],
    "processing_time_ms": 2100,
    "batches": 1,
    "partial_success": true
  }
}
```

### 9.4 Complete Failure Response (Strict Mode)

```json
{
  "success": false,
  "error": {
    "code": "BULK_IMPORT_FAILED",
    "message": "Import failed at row 12. All changes rolled back.",
    "details": {
      "failed_at_row": 12,
      "processed_rows": 11,
      "imported_before_failure": 8,
      "error": {
        "row": 12,
        "field": "value",
        "error_code": "INVALID_VALUE_TYPE",
        "error_message": "Expected number, got string"
      },
      "rollback_status": "completed"
    }
  }
}
```

### 9.5 Error Types

| Error Code | Description | Row-Level | Fatal |
|------------|-------------|-----------|-------|
| `INVALID_VALUE_TYPE` | Value doesn't match value_type | ✅ | No |
| `INVALID_CATEGORY` | Category not found | ✅ | No |
| `INVALID_NAME` | Name format invalid | ✅ | No |
| `MISSING_REQUIRED_FIELD` | Required field missing | ✅ | No |
| `DUPLICATE_KEY` | Parameter already exists (if update_existing=false) | ✅ | No |
| `INVALID_DATE_FORMAT` | effective_from/until format invalid | ✅ | No |
| `INVALID_JSON` | JSON parsing error | ✅ | No |
| `CSV_PARSE_ERROR` | Malformed CSV | ❌ | Yes |
| `FILE_TOO_LARGE` | File exceeds max size (10MB) | ❌ | Yes |
| `DATABASE_ERROR` | DB connection/transaction error | ❌ | Yes |

### 9.6 Rollback Behavior

**Strict Mode:**
```
BEGIN TRANSACTION
FOR each row:
  TRY:
    Validate
    Insert/Update
  CATCH:
    ROLLBACK
    RETURN error
COMMIT
```

**Lenient Mode:**
```
BEGIN TRANSACTION
errors = []
FOR each row:
  TRY:
    Validate
    Insert/Update
  CATCH error:
    errors.append({row, error})
    IF continue_on_error:
      CONTINUE
    ELSE:
      ROLLBACK
      RETURN errors
COMMIT
RETURN {imported, errors}
```

### 9.7 Validation Pipeline

```
Row Input
    ↓
[1] CSV Parsing → Extract fields
    ↓
[2] Schema Validation → Required fields, data types
    ↓
[3] Business Validation → Category exists, name valid
    ↓
[4] Duplicate Check → Existing parameter (if update_existing)
    ↓
[5] Value Validation → JSON valid, matches value_type
    ↓
[6] DB Constraints → Unique constraints, FK checks
    ↓
Insert/Update
```

### 9.8 Dry Run Mode

Validate without persisting:

```json
{
  "success": true,
  "data": {
    "dry_run": true,
    "would_import": 45,
    "would_update": 5,
    "would_skip": 2,
    "would_fail": 3,
    "errors": [...],
    "validation_summary": {
      "total_rows": 55,
      "valid_rows": 50,
      "invalid_rows": 3,
      "skipped_rows": 2
    }
  }
}
```

---

## 10. Complete Error Code Reference

### 10.1 HTTP Status Codes

| Status | Usage |
|--------|-------|
| 200 OK | Successful GET, PUT (update) |
| 201 Created | Successful POST (create) |
| 204 No Content | Successful DELETE |
| 400 Bad Request | Invalid request format, validation error |
| 401 Unauthorized | Missing/invalid authentication |
| 403 Forbidden | Valid auth, but no permission |
| 404 Not Found | Resource doesn't exist |
| 409 Conflict | Resource already exists, version conflict |
| 410 Gone | Resource expired or inactive |
| 422 Unprocessable Entity | Valid JSON, but semantic error |
| 429 Too Many Requests | Rate limit exceeded |
| 500 Internal Server Error | Unexpected server error |
| 503 Service Unavailable | Service temporarily unavailable |

### 10.2 Error Code Reference

| Code | HTTP | Description | Retryable |
|------|------|-------------|-----------|
| **General Errors** ||||
| `INVALID_REQUEST` | 400 | Request format invalid | No |
| `MISSING_REQUIRED_FIELD` | 400 | Required field not provided | No |
| `INVALID_FIELD_VALUE` | 400 | Field value invalid | No |
| `INVALID_JSON` | 400 | JSON parsing error | No |
| `INVALID_CONTENT_TYPE` | 400 | Content-Type not supported | No |
| **Authentication Errors** ||||
| `UNAUTHORIZED` | 401 | Authentication required | No |
| `INVALID_TOKEN` | 401 | Token invalid or expired | No |
| `INVALID_API_KEY` | 401 | API key invalid | No |
| `FORBIDDEN` | 403 | Permission denied | No |
| `INSUFFICIENT_PERMISSIONS` | 403 | Valid auth, insufficient scope | No |
| **Resource Errors** ||||
| `PARAMETER_NOT_FOUND` | 404 | Parameter doesn't exist | No |
| `CATEGORY_NOT_FOUND` | 404 | Category doesn't exist | No |
| `ORG_NOT_FOUND` | 404 | Organization doesn't exist | No |
| `PARAMETER_EXISTS` | 409 | Parameter already exists | No |
| `VERSION_CONFLICT` | 409 | Concurrent modification conflict | Yes |
| `PARAMETER_EXPIRED` | 410 | Parameter expired or inactive | No |
| **Validation Errors** ||||
| `INVALID_VALUE_TYPE` | 422 | Value doesn't match value_type | No |
| `INVALID_CATEGORY` | 422 | Category not recognized | No |
| `INVALID_NAME_FORMAT` | 422 | Name format invalid | No |
| `INVALID_DATE_RANGE` | 422 | effective_from > effective_until | No |
| `INVALID_SCOPE` | 422 | Scope not valid for category | No |
| `INVALID_UNIT` | 422 | Unit not recognized | No |
| `INVALID_JSON_VALUE` | 422 | Invalid JSON in value field | No |
| `EXCEEDS_MAX_VALUE` | 422 | Value exceeds maximum allowed | No |
| `BELOW_MIN_VALUE` | 422 | Value below minimum allowed | No |
| **Rate Limiting Errors** ||||
| `RATE_LIMIT_EXCEEDED` | 429 | Rate limit exceeded | Yes |
| `BURST_LIMIT_EXCEEDED` | 429 | Burst limit exceeded | Yes |
| **Server Errors** ||||
| `INTERNAL_ERROR` | 500 | Unexpected server error | Yes |
| `DATABASE_ERROR` | 500 | Database operation failed | Yes |
| `CACHE_ERROR` | 500 | Cache operation failed | Yes |
| `SERVICE_UNAVAILABLE` | 503 | Service temporarily unavailable | Yes |
| `MAINTENANCE_MODE` | 503 | System in maintenance | Yes |
| **Bulk Import Errors** ||||
| `CSV_PARSE_ERROR` | 400 | CSV parsing failed | No |
| `FILE_TOO_LARGE` | 400 | File exceeds 10MB limit | No |
| `BULK_IMPORT_FAILED` | 422 | Import failed (strict mode) | Yes |
| `PARTIAL_IMPORT_FAILED` | 422 | Some rows failed (lenient mode) | Yes |
| **Pagination Errors** ||||
| `INVALID_PAGE` | 400 | Page number invalid | No |
| `INVALID_LIMIT` | 400 | Limit exceeds maximum | No |
| `INVALID_SORT_FIELD` | 400 | Sort field not recognized | No |
| `INVALID_SORT_DIRECTION` | 400 | Sort direction invalid | No |

### 10.3 Error Message Localization

Error messages support multi-language via `Accept-Language` header:

```http
GET /v1/params/unknown
Accept-Language: id

Response:
{
  "success": false,
  "error": {
    "code": "PARAMETER_NOT_FOUND",
    "message": "Parameter tidak ditemukan",
    "details": {...}
  }
}
```

**Supported languages (v1.0):**
- `en` (default)
- `id`

### 10.4 Error Response Format

Standard error structure:

```json
{
  "success": false,
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable message",
    "details": {
      // Error-specific details
    },
    "request_id": "uuid-for-tracing",
    "timestamp": "2026-04-27T10:00:00Z"
  }
}
```

---

## 11. Open Questions

| # | Question | Status |
|---|----------|--------|
| 1 | Apakah perlu versioning di URL (e.g., `/v2/`)? | Default v1, evaluasi saat breaking changes |
| 2 | Apakah perlu GraphQL endpoint untuk flexible queries? | v1.5 — evaluate demand |
| 3 | Apakah perlu webhook notification saat parameter change? | v1.1 — dengan NATS events |
| 4 | Apakah perlu caching headers (ETag, Cache-Control)? | v1.0 — Redis cache dengan TTL |

---

*Next: [03-data-model.md](./03-data-model.md)*
