# Policy7 — Spec 04: Integration

> **Versi**: 0.1-draft | **Tanggal**: 2026-04-27 | **Fase**: Review

---

## 1. Integration Overview

Policy7 adalah service terpusat untuk business parameters yang di-consume oleh berbagai services di ekosistem Core7. Spec ini mendefinisikan integration patterns dengan semua consumer services.

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Core7 Ecosystem                              │
│                                                                      │
│   ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐        │
│   │  auth7   │   │  core7   │   │ workflow7│   │  notif7  │        │
│   │          │   │enterprise│   │          │   │          │        │
│   │ "BOLEH?" │   │"BERAPA?" │   │"APPROVE?"│   │"ALERT?"  │        │
│   └────┬─────┘   └────┬─────┘   └────┬─────┘   └────┬─────┘        │
│        │              │              │              │               │
│        └──────────────┼──────────────┼──────────────┘               │
│                       │              │                               │
│              ┌────────▼──────────────▼────────┐                      │
│              │           policy7               │                      │
│              │    "Transaction Limits"         │                      │
│              │    "Approval Thresholds"        │                      │
│              │    "Rates & Fees"               │                      │
│              └─────────────────────────────────┘                      │
│                       │                                              │
│              ┌────────▼────────┐                                     │
│              │  PostgreSQL 16  │                                     │
│              │     Redis       │                                     │
│              │     NATS        │                                     │
│              └─────────────────┘                                     │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 2. Auth7 Integration

### 2.1 Use Case: OPA Query Policy7 untuk ABAC Parameters

Auth7 menggunakan OPA/Rego untuk ABAC boolean rules. Beberapa rules memerlukan parameter dari policy7 (contoh: jam operasional teller).

**Integration Pattern:**

```
┌──────────┐      ┌──────────┐      ┌──────────┐
│   OPA    │ ───→ │  auth7   │ ───→ │ policy7  │
│  Rego    │      │  (gRPC)  │      │  (HTTP)  │
└──────────┘      └──────────┘      └──────────┘
```

**OPA Rego Policy Example:**

```rego
package authz

import data.policy7.operational_hours

# Query policy7 untuk jam operasional
teller_hours = operational_hours["teller"]

allow {
  input.user.roles[_] == "teller"
  input.resource.type == "transaction"
  input.action == "create"
  
  # Cek jam operasional dari policy7
  current_time := input.request.time
  current_time >= teller_hours.start_time
  current_time <= teller_hours.end_time
}
```

**Auth7 Implementation:**

```go
// internal/authz/opa.go

package authz

import (
    "context"
    "fmt"
    
    "github.com/open-policy-agent/opa/rego"
)

type Policy7Data struct {
    client *Policy7Client
    cache  *Cache
}

func (p *Policy7Data) GetOperationalHours(ctx context.Context, orgID, role string) (*OperationalHours, error) {
    const op = "authz.Policy7Data.GetOperationalHours"
    
    // Check cache first (TTL 5 menit)
    cacheKey := fmt.Sprintf("ophours:%s:%s", orgID, role)
    if cached, ok := p.cache.Get(cacheKey); ok {
        return cached.(*OperationalHours), nil
    }
    
    // Query policy7
    hours, err := p.client.GetOperationalHours(ctx, orgID, role)
    if err != nil {
        return nil, fmt.Errorf("%s: %w", op, err)
    }
    
    // Cache result
    p.cache.Set(cacheKey, hours, 5*time.Minute)
    
    return hours, nil
}

// OPA Data Provider
func (p *Policy7Data) BuildDataBundle(ctx context.Context, orgID string) (map[string]interface{}, error) {
    const op = "authz.Policy7Data.BuildDataBundle"
    
    // Fetch all policy7 data needed for ABAC evaluation
    bundle := map[string]interface{}{
        "operational_hours": p.fetchOperationalHours(ctx, orgID),
        "product_access":    p.fetchProductAccess(ctx, orgID),
    }
    
    return bundle, nil
}
```

### 2.2 Auth7 → Policy7 API Flow

```
1. User request masuk ke auth7 (e.g., create transaction)
2. Auth7 OPA evaluate ABAC rules
3. Jika rule butuh parameter dari policy7:
   a. Check cache (Redis)
   b. If miss: HTTP GET ke policy7
   c. Cache result (5 menit TTL)
4. OPA evaluation selesai (ALLOW/DENY)
5. Response ke client
```

### 2.3 API Endpoint untuk Auth7

```
GET /v1/params/operational-hours?role={role}
GET /v1/params/product-access?role={role}&product={product}

Headers:
  X-Service-ID: auth7
  X-API-Key: {service_api_key}
  X-Org-ID: {org_uuid}
```

---

## 3. Core7 Enterprise Integration

### 3.1 Use Case: Transaction Validation

Core7 Enterprise (bos7-enterprise, financing, treasury, dll) perlu validasi transaction limits sebelum memproses transaksi.

**Integration Pattern:**

```
┌──────────────────┐      ┌──────────┐      ┌──────────┐
│ bos7-enterprise  │ ───→ │ BFF/     │ ───→ │ policy7  │
│ (Next.js)        │      │ API GW   │      │          │
└──────────────────┘      └──────────┘      └──────────┘
```

**Transaction Validation Flow:**

```
┌─────────────────────────────────────────────────────────────────┐
│                    TRANSACTION VALIDATION FLOW                   │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Step 1: Teller Input Transaksi                                  │
│  ├── Amount: Rp 75.000.000                                       │
│  ├── Product: transfer                                           │
│  └── Teller Role: teller                                         │
│                      ↓                                           │
│  Step 2: Core7 API Validasi                                      │
│  ├── POST /v1/params/transaction_limit/validate                  │
│  ├── Request: {amount: 75000000, role: "teller", product: "transfer"}
│  └── Response: {decision: "REQUIRES_AUTHORIZATION", ...}         │
│                      ↓                                           │
│  Step 3: Decision Handling                                       │
│  ├── IF decision == "AUTO_AUTHORIZED"                            │
│  │   └── Proses langsung                                         │
│  ├── IF decision == "REQUIRES_AUTHORIZATION"                     │
│  │   └── Submit ke workflow7 untuk approval                      │
│  └── IF decision == "REJECTED"                                   │
│      └── Return error ke teller                                  │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

**Core7 Implementation:**

```go
// internal/service/transaction.go

type TransactionService struct {
    policy7Client *policy7.Client
    workflow7Client *workflow7.Client
}

func (s *TransactionService) ValidateAndProcess(ctx context.Context, tx *Transaction) error {
    const op = "TransactionService.ValidateAndProcess"
    
    // Step 1: Get user context dari auth7
    claims := auth7.ClaimsFromContext(ctx)
    
    // Step 2: Validate transaction against policy7
    validation, err := s.policy7Client.ValidateTransaction(ctx, policy7.ValidateRequest{
        Amount:    tx.Amount,
        Role:      claims.Role,
        Product:   tx.ProductType,
        OrgID:     claims.OrgID,
    })
    if err != nil {
        return fmt.Errorf("%s: %w", op, err)
    }
    
    // Step 3: Handle decision
    switch validation.Decision {
    case "AUTO_AUTHORIZED":
        // Process immediately
        return s.processTransaction(ctx, tx)
        
    case "REQUIRES_AUTHORIZATION":
        // Submit to workflow7
        approvalTask := &workflow7.Task{
            Type:     "transaction_approval",
            Amount:   tx.Amount,
            Approver: validation.RequiredApprover,
            Data:     tx,
        }
        taskID, err := s.workflow7Client.CreateTask(ctx, approvalTask)
        if err != nil {
            return fmt.Errorf("%s: %w", op, err)
        }
        
        tx.Status = "PENDING_APPROVAL"
        tx.ApprovalTaskID = taskID
        return s.saveTransaction(ctx, tx)
        
    case "REJECTED":
        return domain.ErrTransactionLimitExceeded
        
    default:
        return fmt.Errorf("%s: unknown decision: %s", op, validation.Decision)
    }
}
```

### 3.2 Core7 → Policy7 API Endpoints

```
# Transaction Validation
POST /v1/params/transaction_limit/validate

# Get Interest Rate
GET /v1/params/rates/:product?tenor={tenor}&amount={amount}

# Get Fee
GET /v1/params/fees/:product?channel={channel}&amount={amount}

# Get Product Access
GET /v1/params/product-access?role={role}&product={product}

Headers:
  Authorization: Bearer {service_token}
  X-Org-ID: {org_uuid}
```

---

## 4. Workflow7 Integration

### 4.1 Use Case: Approval Workflow

Workflow7 mengelola approval tasks. Policy7 menyediakan approval thresholds yang menentukan kapan approval diperlukan dan siapa yang bisa approve.

**Integration Pattern:**

```
┌──────────┐      ┌──────────┐      ┌──────────┐
│ workflow7│ ───→ │ policy7  │      │  auth7   │
│          │      │(threshold│      │(approver │
│          │      │ & auth   │      │  limit)  │
└──────────┘      └──────────┘      └──────────┘
```

**Approval Flow:**

```
┌─────────────────────────────────────────────────────────────────┐
│                     APPROVAL WORKFLOW FLOW                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Step 1: workflow7 Menerima Task                                 │
│  ├── Task: Transaction Approval                                  │
│  ├── Amount: Rp 75.000.000                                       │
│  └── Requestor: teller                                           │
│                      ↓                                           │
│  Step 2: Query policy7 untuk Approval Threshold                  │
│  ├── GET /v1/params/approval-thresholds?role=teller&amount=75jt  │
│  └── Response: {approver_roles: ["supervisor"], level: 1}        │
│                      ↓                                           │
│  Step 3: Query policy7 untuk Approver Authorization Limit        │
│  ├── POST /v1/params/authorization_limit/check                   │
│  ├── Request: {amount: 75000000, approver_role: "supervisor"}    │
│  └── Response: {allowed: true, limit: 100000000}                 │
│                      ↓                                           │
│  Step 4: workflow7 Buat Approval Task                            │
│  ├── Assignee: supervisor                                        │
│  ├── Task: Approve Rp 75jt transfer                              │
│  └── Escalation: none (dibawah BM limit)                         │
│                      ↓                                           │
│  Step 5: Supervisor Approves                                     │
│  ├── Check policy7: Can supervisor auth 75jt? ✅                 │
│  └── Transaksi efektif                                           │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

**Workflow7 Implementation:**

```go
// internal/service/approval.go

type ApprovalService struct {
    policy7Client *policy7.Client
}

func (s *ApprovalService) CreateApprovalTask(ctx context.Context, task *Task) (*Task, error) {
    const op = "ApprovalService.CreateApprovalTask"
    
    // Get approval threshold dari policy7
    threshold, err := s.policy7Client.GetApprovalThreshold(ctx, policy7.ApprovalThresholdRequest{
        Role:   task.RequestorRole,
        Amount: task.Amount,
        Product: task.Product,
    })
    if err != nil {
        return nil, fmt.Errorf("%s: %w", op, err)
    }
    
    // Get authorization limits untuk setiap approver role
    approvers := []Approver{}
    for _, role := range threshold.ApproverRoles {
        authLimit, err := s.policy7Client.GetAuthorizationLimit(ctx, policy7.AuthorizationLimitRequest{
            Role: role,
        })
        if err != nil {
            return nil, fmt.Errorf("%s: %w", op, err)
        }
        
        // Check if this role can authorize the amount
        if task.Amount <= authLimit.AuthorizationMax {
            approvers = append(approvers, Approver{
                Role:       role,
                CanApprove: true,
            })
        }
    }
    
    task.Approvers = approvers
    task.Level = threshold.ApprovalLevel
    
    return s.saveTask(ctx, task)
}

func (s *ApprovalService) ValidateApprover(ctx context.Context, taskID uuid.UUID, approverID uuid.UUID) error {
    const op = "ApprovalService.ValidateApprover"
    
    task, err := s.getTask(ctx, taskID)
    if err != nil {
        return fmt.Errorf("%s: %w", op, err)
    }
    
    // Check if approver can authorize this amount
    canAuth, err := s.policy7Client.CheckAuthorizationLimit(ctx, policy7.AuthorizationCheckRequest{
        Amount:       task.Amount,
        ApproverRole: task.CurrentApproverRole,
    })
    if err != nil {
        return fmt.Errorf("%s: %w", op, err)
    }
    
    if !canAuth.Allowed {
        return domain.ErrApproverNotAuthorized
    }
    
    return nil
}
```

### 4.2 Workflow7 → Policy7 API Endpoints

```
# Get Approval Threshold
GET /v1/params/approval-thresholds?role={role}&amount={amount}&product={product}

# Check Approver Authorization
POST /v1/params/authorization_limit/check

Headers:
  X-Service-ID: workflow7
  X-API-Key: {service_api_key}
  X-Org-ID: {org_uuid}
```

---

## 5. Notif7 Integration

### 5.1 Use Case: Regulatory Alerts (CTR/STR)

Notif7 mengirimkan notifikasi untuk regulatory compliance (CTR/STR thresholds).

**Integration Pattern:**

```
┌──────────┐      ┌──────────┐      ┌──────────┐
│  core7   │ ───→ │ policy7  │ ───→ │  notif7  │
│          │      │(regulatory│      │(alert)   │
│          │      │threshold)│      │          │
└──────────┘      └──────────┘      └──────────┘
```

**CTR Alert Flow:**

```
┌─────────────────────────────────────────────────────────────────┐
│                    REGULATORY ALERT FLOW                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Step 1: Core7 Proses Transaksi Cash                             │
│  ├── Amount: Rp 125.000.000                                      │
│  ├── Type: Cash Deposit                                          │
│  └── Nasabah: Pak Ahmad                                          │
│                      ↓                                           │
│  Step 2: Check CTR Threshold di policy7                          │
│  ├── GET /v1/params/regulatory/ctr_threshold?country=ID          │
│  └── Response: {threshold: 100000000}                            │
│                      ↓                                           │
│  Step 3: Amount > Threshold?                                     │
│  ├── Rp 125jt > Rp 100jt? ✅ CTR REQUIRED                        │
│  └── Trigger notifikasi                                          │
│                      ↓                                           │
│  Step 4: Core7 Publish Event ke NATS                             │
│  ├── Topic: policy7.regulatory.threshold_exceeded                │
│  └── Payload: {type: "CTR", amount: 125000000, ...}              │
│                      ↓                                           │
│  Step 5: Notif7 Consume Event                                    │
│  ├── Buat notifikasi CTR untuk compliance officer                │
│  ├── Kirim email/SMS alert                                       │
│  └── Log ke audit trail                                          │
│                      ↓                                           │
│  Step 6: Compliance Officer Action                               │
│  └── Proses laporan CTR ke PPATK                                 │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

**Notif7 Implementation:**

```go
// internal/handler/regulatory.go

func (h *RegulatoryHandler) HandleThresholdExceeded(ctx context.Context, event *nats.Msg) error {
    const op = "RegulatoryHandler.HandleThresholdExceeded"
    
    var payload struct {
        Type        string    `json:"type"`        // "CTR" or "STR"
        Amount      float64   `json:"amount"`
        Currency    string    `json:"currency"`
        CustomerID  string    `json:"customer_id"`
        TransactionID string `json:"transaction_id"`
        OrgID       string    `json:"org_id"`
        Timestamp   time.Time `json:"timestamp"`
    }
    
    if err := json.Unmarshal(event.Data, &payload); err != nil {
        return fmt.Errorf("%s: %w", op, err)
    }
    
    // Get regulatory details dari policy7
    regulatory, err := h.policy7Client.GetRegulatoryThreshold(ctx, policy7.RegulatoryRequest{
        Type: payload.Type,
        Country: "ID",
    })
    if err != nil {
        return fmt.Errorf("%s: %w", op, err)
    }
    
    // Create notification
    notification := &notif7.Notification{
        Type:        "REGULATORY_ALERT",
        Priority:    "HIGH",
        Title:       fmt.Sprintf("%s Report Required", payload.Type),
        Message:     fmt.Sprintf("Transaction ID %s exceeds %s threshold (Rp %.0f)", 
                       payload.TransactionID, payload.Type, regulatory.Threshold),
        Recipients:  []string{"compliance@bank.co.id"},
        Data: map[string]interface{}{
            "transaction_id": payload.TransactionID,
            "amount":         payload.Amount,
            "threshold":      regulatory.Threshold,
            "deadline_hours": regulatory.ReportingDeadlineHours,
        },
    }
    
    return h.notif7.Send(ctx, notification)
}
```

### 5.2 Notif7 → Policy7 API Endpoints

```
# Get Regulatory Threshold
GET /v1/params/regulatory/:type?country={country}

Headers:
  X-Service-ID: notif7
  X-API-Key: {service_api_key}
  X-Org-ID: {org_uuid}
```

---

## 6. NATS Event Streaming

### 6.1 Event Types

| Event Topic | Publisher | Consumer | Purpose |
|-------------|-----------|----------|---------|
| `policy7.parameter.created` | policy7 | auth7, core7 | Cache invalidation |
| `policy7.parameter.updated` | policy7 | auth7, core7 | Cache invalidation |
| `policy7.parameter.deleted` | policy7 | auth7, core7 | Cache invalidation |
| `policy7.regulatory.threshold_exceeded` | core7 | notif7 | CTR/STR alerts |
| `policy7.transaction.requires_authorization` | core7 | workflow7 | Approval task |
| `policy7.transaction.auto_authorized` | core7 | audit | Audit logging |

### 6.2 Event Structure

```json
{
  "event_id": "uuid",
  "event_type": "policy7.parameter.updated",
  "timestamp": "2026-04-27T10:00:00Z",
  "org_id": "uuid-bjbs",
  "data": {
    "parameter_id": "uuid",
    "category": "transaction_limit",
    "name": "teller_transfer_max",
    "previous_value": 10000000,
    "new_value": 15000000,
    "changed_by": "admin-uuid"
  }
}
```

### 6.3 Cache Invalidation via NATS

```go
// internal/service/cache.go

func (s *CacheService) InvalidateOnEvent(ctx context.Context, event *nats.Msg) error {
    const op = "CacheService.InvalidateOnEvent"
    
    var payload struct {
        OrgID      string `json:"org_id"`
        Category   string `json:"category"`
        Name       string `json:"name"`
        AppliesTo  string `json:"applies_to"`
        AppliesToID string `json:"applies_to_id"`
        Product    string `json:"product"`
    }
    
    if err := json.Unmarshal(event.Data, &payload); err != nil {
        return fmt.Errorf("%s: %w", op, err)
    }
    
    // Build cache key patterns
    patterns := []string{
        fmt.Sprintf("policy7:%s:%s:%s:*", payload.OrgID, payload.Category, payload.Name),
        fmt.Sprintf("policy7:%s:%s:*", payload.OrgID, payload.Category),
    }
    
    // Invalidate all matching keys
    for _, pattern := range patterns {
        if err := s.redis.DelPattern(ctx, pattern); err != nil {
            log.Error().Err(err).Str("pattern", pattern).Msg("failed to invalidate cache")
        }
    }
    
    return nil
}
```

### 6.4 Health Check via NATS (Request-Reply)

Policy7 menyediakan health check endpoint via NATS request-reply pattern untuk service discovery dan monitoring.

**Subject:** `policy7.health`

**Request:**
```json
{
  "request_id": "uuid",
  "timestamp": "2026-04-27T10:00:00Z",
  "check_type": "full"  // "full" | "ping"
}
```

**Response:**
```json
{
  "request_id": "uuid",
  "timestamp": "2026-04-27T10:00:00Z",
  "status": "healthy",  // "healthy" | "degraded" | "unhealthy"
  "version": "1.0.0",
  "instance_id": "pod-xyz-123",
  "checks": {
    "database": {
      "status": "healthy",
      "latency_ms": 5,
      "connections": {
        "active": 10,
        "idle": 5,
        "max": 25
      }
    },
    "cache": {
      "status": "healthy", 
      "latency_ms": 2,
      "hit_rate": 0.94
    },
    "nats": {
      "status": "healthy",
      "connected": true,
      "subscriptions": 5
    }
  }
}
```

**Implementation:**

```go
// internal/api/nats/handler.go

func (h *HealthHandler) HandleHealthCheck(msg *nats.Msg) {
    const op = "nats.HealthHandler.HandleHealthCheck"
    
    var req HealthRequest
    if err := json.Unmarshal(msg.Data, &req); err != nil {
        log.Error().Err(err).Msg("failed to parse health check request")
        return
    }
    
    // Run health checks
    checks := HealthChecks{
        Database: h.checkDatabase(),
        Cache:    h.checkCache(),
        NATS:     h.checkNATS(),
    }
    
    // Determine overall status
    status := "healthy"
    if checks.Database.Status != "healthy" || checks.Cache.Status != "healthy" {
        status = "degraded"
    }
    if checks.Database.Status == "unhealthy" {
        status = "unhealthy"
    }
    
    resp := HealthResponse{
        RequestID:  req.RequestID,
        Timestamp:  time.Now().UTC(),
        Status:     status,
        Version:    h.version,
        InstanceID: h.instanceID,
        Checks:     checks,
    }
    
    data, _ := json.Marshal(resp)
    msg.Respond(data)
}
```

**Usage untuk Kubernetes Readiness Probe:**

```yaml
# k8s deployment
readinessProbe:
  exec:
    command:
    - /bin/sh
    - -c
    - |
      nats request policy7.health \
        '{"check_type":"ping"}' \
        --timeout=5s | grep -q "healthy"
  initialDelaySeconds: 10
  periodSeconds: 5
```

### 6.5 JetStream Decision

**Decision: Core NATS Only untuk v1.0**

| Aspect | Core NATS (v1.0) | JetStream (v1.1 - Future) |
|--------|------------------|---------------------------|
| **Persistence** | Fire-and-forget (no persistence) | Durable streams dengan replay |
| **Use Case** | Cache invalidation events | Event audit trail, guaranteed delivery |
| **Complexity** | Low - simple pub/sub | Medium - streams, consumers, ACKs |
| **Resource Usage** | Minimal | Higher (disk untuk message log) |
| **Ordering** | Best-effort | Guaranteed per subject |
| **Replay** | Not possible | Yes - by time or sequence |

**Rationale untuk Core NATS v1.0:**

1. **Cache Invalidation Events** adalah ephemeral — jika missed, cache akan refresh on next miss
2. **Idempotent Parameters** — services can always refetch dari PostgreSQL
3. **Audit Trail** — Sudah covered oleh `parameter_history` table
4. **Simplicity** — Less infrastructure, faster development
5. **Sufficiency** — Core NATS meets all v1.0 requirements

**When to Upgrade to JetStream:**

- Event audit trail requirement ("siapa yang publish event apa kapan")
- Guaranteed delivery untuk critical business events
- Need event replay untuk debugging/recovery
- > 1000 events/second sustained load

**Migration Path:**

```
v1.0: Core NATS
  - Subjects: policy7.params.created, updated, deleted
  - No persistence needed

v1.1: Add JetStream (if needed)
  - Create Stream: "POLICY7_EVENTS"
  - Subjects: policy7.params.>
  - Retention: 7 days
  - Consumers: audit-logger, event-replay
  - Backwards compatible: Core NATS clients still work
```

**Implementation Checklist:**

- [ ] v1.0: Use `nats.Connect()` — standard connection
- [ ] v1.0: Use `nc.Publish()` — fire-and-forget
- [ ] v1.0: Use `nc.Subscribe()` — simple subscription
- [ ] Future v1.1: Consider `jetstream.Publish()` untuk guaranteed delivery
- [ ] Future v1.1: Add `Ack()` handling jika needed

---

## 7. Service-to-Service Authentication

### 7.1 API Key Pattern

```
Headers untuk service-to-service calls:

X-Service-ID: {service_name}      # e.g., "auth7", "core7-enterprise", "workflow7"
X-API-Key: {service_api_key}      # UUID, stored in policy7.config
X-Org-ID: {org_uuid}              # Target organization
X-Request-ID: {request_id}        # For tracing
```

### 7.2 Service API Key Validation

```go
// internal/api/middleware/service_auth.go

func ServiceAuthMiddleware(store *ServiceKeyStore) gin.HandlerFunc {
    return func(c *gin.Context) {
        const op = "middleware.ServiceAuth"
        
        serviceID := c.GetHeader("X-Service-ID")
        apiKey := c.GetHeader("X-API-Key")
        orgID := c.GetHeader("X-Org-ID")
        
        if serviceID == "" || apiKey == "" {
            c.AbortWithStatusJSON(401, gin.H{"error": "missing service credentials"})
            return
        }
        
        // Validate API key
        valid, err := store.ValidateServiceKey(c.Request.Context(), serviceID, apiKey, orgID)
        if err != nil {
            log.Error().Err(err).Str("service", serviceID).Msg("service auth validation failed")
            c.AbortWithStatusJSON(500, gin.H{"error": "internal error"})
            return
        }
        
        if !valid {
            c.AbortWithStatusJSON(401, gin.H{"error": "invalid service credentials"})
            return
        }
        
        c.Set("service_id", serviceID)
        c.Set("org_id", orgID)
        c.Next()
    }
}
```

---

## 8. Go Client Library

### 8.1 Client Interface

```go
// pkg/client/policy7.go

package client

import (
    "context"
    "time"
    
    "github.com/google/uuid"
)

// Client interface untuk consumers
type Client interface {
    // Transaction Validation
    ValidateTransaction(ctx context.Context, req ValidateRequest) (*ValidateResponse, error)
    
    // Parameter Queries
    GetParameter(ctx context.Context, category, name string) (*Parameter, error)
    GetEffectiveParameter(ctx context.Context, category, name string, context map[string]string) (*Parameter, error)
    
    // Limits
    GetTransactionLimit(ctx context.Context, role, product string) (*TransactionLimit, error)
    GetAuthorizationLimit(ctx context.Context, role string) (*AuthorizationLimit, error)
    CheckAuthorizationLimit(ctx context.Context, req AuthorizationCheckRequest) (*AuthorizationCheckResponse, error)
    
    // Approval
    GetApprovalThreshold(ctx context.Context, role string, amount float64) (*ApprovalThreshold, error)
    
    // Rates & Fees
    GetInterestRate(ctx context.Context, product, tenor string, amount float64) (*InterestRate, error)
    GetFee(ctx context.Context, product, channel string, amount float64) (*Fee, error)
    
    // Regulatory
    GetRegulatoryThreshold(ctx context.Context, thresholdType, country string) (*RegulatoryThreshold, error)
}

// Implementation
type policy7Client struct {
    baseURL    string
    httpClient *http.Client
    apiKey     string
    serviceID  string
}

func NewClient(baseURL, apiKey, serviceID string) Client {
    return &policy7Client{
        baseURL:    baseURL,
        httpClient: &http.Client{Timeout: 5 * time.Second},
        apiKey:     apiKey,
        serviceID:  serviceID,
    }
}
```

### 8.2 Usage Example

```go
// In consumer service (e.g., core7-enterprise)

import (
    "github.com/ihsansolusi/policy7/pkg/client"
)

func main() {
    // Initialize client
    policy7 := client.NewClient(
        "https://policy7.core7.internal",
        "api-key-from-config",
        "core7-enterprise",
    )
    
    // Use in service
    validation, err := policy7.ValidateTransaction(ctx, client.ValidateRequest{
        Amount:  75000000,
        Role:    "teller",
        Product: "transfer",
    })
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Decision: %s", validation.Decision)
}
```

---

## 9. Integration Testing

### 9.1 Test Scenarios

| Scenario | Services Involved | Test Steps |
|----------|-------------------|------------|
| Transaction Auto-Authorized | core7 ↔ policy7 | Create transaction < auth limit → Verify auto-approved |
| Transaction Requires Approval | core7 ↔ policy7 ↔ workflow7 | Create transaction > auth limit → Verify approval task created |
| CTR Alert | core7 ↔ policy7 ↔ notif7 | Create cash transaction > CTR threshold → Verify alert sent |
| Cache Invalidation | policy7 → NATS → auth7/core7 | Update parameter → Verify consumers refresh cache |
| OPA ABAC | auth7 ↔ policy7 | Request outside operational hours → Verify denied |

### 9.2 E2E Test Example

```go
// tests/integration/transaction_test.go

func TestTransactionRequiresAuthorization(t *testing.T) {
    // Setup
    ctx := context.Background()
    core7 := test.NewCore7Client()
    policy7 := test.NewPolicy7Client()
    workflow7 := test.NewWorkflow7Client()
    
    // Set teller auth limit = 25jt
    policy7.SetParameter(ctx, &policy7.Parameter{
        Category: "transaction_limit",
        Name:     "teller_authorization_limit",
        Value:    25000000,
    })
    
    // Create transaction 75jt
    tx := &core7.Transaction{
        Amount:  75000000,
        Product: "transfer",
        Role:    "teller",
    }
    
    // Execute
    result, err := core7.CreateTransaction(ctx, tx)
    
    // Verify
    require.NoError(t, err)
    assert.Equal(t, "PENDING_APPROVAL", result.Status)
    
    // Verify approval task created in workflow7
    tasks, _ := workflow7.GetPendingTasks(ctx, "supervisor")
    assert.Len(t, tasks, 1)
}
```

---

## 10. Summary

### Integration Matrix

| Consumer | Use Case | API Endpoints | Events |
|----------|----------|---------------|--------|
| **auth7** | OPA ABAC params | GET /operational-hours, GET /product-access | Consume: parameter.* |
| **core7-enterprise** | Transaction validation | POST /validate, GET /rates, GET /fees | Consume: parameter.*, Publish: regulatory.*, transaction.* |
| **workflow7** | Approval workflow | GET /approval-thresholds, POST /auth-limit/check | Consume: transaction.requires_authorization |
| **notif7** | Regulatory alerts | GET /regulatory/:type | Consume: regulatory.threshold_exceeded |

### Key Integration Decisions

| Decision | Value |
|----------|-------|
| **Auth** | API Key (X-Service-ID, X-API-Key headers) |
| **Cache Invalidation** | NATS pub/sub + Redis DEL pattern |
| **Client Library** | Go package: `github.com/ihsansolusi/policy7/pkg/client` |
| **Tracing** | X-Request-ID header untuk distributed tracing |
| **Timeout** | 5s untuk sync calls, retry 3x |

---

*Next: Implementation Planning (GitHub Issues)*
