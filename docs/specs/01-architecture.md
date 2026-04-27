# Policy7 — Spec 01: Architecture & Design

> **Versi**: 0.2-draft | **Tanggal**: 2026-04-27 | **Fase**: Reviewed — Decisions Made

---

## 1. Design Principles

### 1.1 Core Principles

| Principle | Description |
|-----------|-------------|
| **Parameter as Data** | Policy7 stores data/parameters, not business logic |
| **Versioned & Auditable** | Every change creates new version with audit trail |
| **Multi-tenant** | Parameters isolated per organization |
| **Cache-friendly** | Hot parameters cached in Redis with TTL |
| **Interface-first** | Define interfaces before implementation (enables parallel dev) |

### 1.2 What Policy7 Does NOT Do

- ❌ ABAC boolean rules (auth7)
- ❌ Approval workflow orchestration (workflow7)
- ❌ User/role management (auth7)
- ❌ Real-time scoring engine (v2.0)

---

## 2. Clean Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         API Layer                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   REST API   │  │   Admin API  │  │ Health Check │      │
│  │  (public)    │  │  (protected) │  │              │      │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘      │
└─────────┼──────────────────┼──────────────────┼─────────────┘
          │                  │                  │
          ▼                  ▼                  ▼
┌─────────────────────────────────────────────────────────────┐
│                      Service Layer                           │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   Parameter  │  │   Cache      │  │   Audit      │      │
│  │   Service    │  │   Service    │  │   Service    │      │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘      │
└─────────┼──────────────────┼──────────────────┼─────────────┘
          │                  │                  │
          ▼                  ▼                  ▼
┌─────────────────────────────────────────────────────────────┐
│                       Store Layer                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ Parameter    │  │ Parameter    │  │ Cache        │      │
│  │ Store        │  │ History Store│  │ Store (Redis)│      │
│  │ (PostgreSQL) │  │ (PostgreSQL) │  │              │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
          │
          ▼
┌─────────────────────────────────────────────────────────────┐
│                     Domain Layer                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │  Parameter   │  │   Errors     │  │  Interfaces  │      │
│  │   Entity     │  │              │  │              │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
```

---

## 3. Folder Structure

```
policy7/
├── cmd/
│   └── server/
│       └── main.go                 # Entry point
│
├── internal/
│   ├── api/
│   │   ├── middleware/
│   │   │   ├── auth.go            # JWT validation (mockable)
│   │   │   ├── tenant.go          # Multi-tenant extraction
│   │   │   └── logging.go         # Request logging
│   │   ├── handler/
│   │   │   ├── parameter.go       # Public parameter queries
│   │   │   └── admin.go           # Admin CRUD operations
│   │   └── router.go              # Gin router setup
│   │
│   ├── service/
│   │   ├── parameter.go           # Business logic
│   │   ├── cache.go               # Cache management
│   │   └── audit.go               # Audit trail
│   │
│   ├── store/
│   │   ├── parameter.go           # Parameter DB operations
│   │   ├── history.go             # Audit history DB operations
│   │   └── cache.go               # Redis operations
│   │
│   └── domain/
│       ├── parameter.go           # Parameter entity
│       ├── errors.go              # Domain errors
│       └── interfaces.go          # Service/store interfaces
│
├── pkg/
│   └── client/                    # Go client for consumers
│       └── policy7.go
│
├── migrations/
│   └── *.sql                      # golang-migrate files
│
├── scripts/
│   ├── migrate.sh                 # DB migration scripts
│   └── seed.sh                    # Initial data seeding
│
├── docs/
│   ├── specs/                     # Specifications
│   └── plans/                     # Implementation plans
│
├── docker-compose.yml             # Dev environment
├── Dockerfile
├── go.mod
└── Makefile
```

---

## 4. Key Interfaces

### 4.1 Parameter Service Interface

```go
// internal/domain/interfaces.go

package domain

import (
    "context"
    "github.com/google/uuid"
)

// ParameterService defines business logic for parameters
type ParameterService interface {
    // Query methods (cached)
    GetParameter(ctx context.Context, orgID uuid.UUID, category, name string) (*Parameter, error)
    GetParametersByCategory(ctx context.Context, orgID uuid.UUID, category string) ([]*Parameter, error)
    GetEffectiveParameter(ctx context.Context, orgID uuid.UUID, category, name string, appliesTo, appliesToID string) (*Parameter, error)
    
    // Admin methods (bypass cache)
    CreateParameter(ctx context.Context, orgID uuid.UUID, creatorID uuid.UUID, param *CreateParameterInput) (*Parameter, error)
    UpdateParameter(ctx context.Context, orgID uuid.UUID, updaterID uuid.UUID, paramID uuid.UUID, input *UpdateParameterInput) (*Parameter, error)
    DeleteParameter(ctx context.Context, orgID uuid.UUID, deleterID uuid.UUID, paramID uuid.UUID) error
    
    // History
    GetParameterHistory(ctx context.Context, orgID uuid.UUID, paramID uuid.UUID) ([]*ParameterHistory, error)
}

// ParameterStore defines data access interface
type ParameterStore interface {
    GetByID(ctx context.Context, orgID, id uuid.UUID) (*Parameter, error)
    GetByName(ctx context.Context, orgID uuid.UUID, category, name string) (*Parameter, error)
    GetByCategory(ctx context.Context, orgID uuid.UUID, category string) ([]*Parameter, error)
    GetEffective(ctx context.Context, orgID uuid.UUID, category, name, appliesTo, appliesToID string) (*Parameter, error)
    
    Create(ctx context.Context, param *Parameter) error
    Update(ctx context.Context, param *Parameter) error
    SoftDelete(ctx context.Context, orgID, id uuid.UUID) error
    
    List(ctx context.Context, orgID uuid.UUID, filter ListFilter) ([]*Parameter, error)
}

// AuthClient interface (for auth7 integration)
type AuthClient interface {
    ValidateToken(ctx context.Context, token string) (*TokenClaims, error)
    CheckPermission(ctx context.Context, userID, resource, action string) (bool, error)
}

// CacheStore interface (for Redis)
type CacheStore interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    InvalidatePattern(ctx context.Context, pattern string) error
}
```

### 4.2 Mock Implementations

```go
// internal/domain/mock/auth.go

package mock

// MockAuthClient for parallel development
type MockAuthClient struct {
    ValidTokens map[string]*domain.TokenClaims
}

func (m *MockAuthClient) ValidateToken(ctx context.Context, token string) (*domain.TokenClaims, error) {
    if claims, ok := m.ValidTokens[token]; ok {
        return claims, nil
    }
    return nil, domain.ErrUnauthorized
}

func (m *MockAuthClient) CheckPermission(ctx context.Context, userID, resource, action string) (bool, error) {
    // Mock: always allow for development
    return true, nil
}
```

---

## 5. Multi-Tenancy Strategy

### 5.1 Tenant Isolation

```sql
-- All queries MUST include org_id filter
SELECT * FROM parameters 
WHERE org_id = $1 AND category = $2 AND name = $3;
```

### 5.2 Tenant Extraction

```go
// internal/api/middleware/tenant.go

func TenantMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Option 1: From JWT claims
        claims, exists := c.Get("claims")
        if !exists {
            c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
            return
        }
        
        tokenClaims := claims.(*domain.TokenClaims)
        c.Set("org_id", tokenClaims.OrgID)
        
        // Option 2: From header (for service-to-service)
        // orgID := c.GetHeader("X-Org-ID")
        
        c.Next()
    }
}
```

---

## 6. Caching Strategy

### 6.1 Cache Key Pattern

```
policy7:{org_id}:{category}:{name}:{applies_to}:{applies_to_id}
policy7:{org_id}:{category}:{name}:global

Examples:
policy7:uuid-bjbs:transaction_limit:teller_transfer_max:role:teller
policy7:uuid-bjbs:rate:deposito_12m:product:deposito
```

### 6.2 Cache Invalidation

```go
// On parameter update
func (s *parameterService) UpdateParameter(...) {
    // 1. Update DB
    err := s.store.Update(ctx, param)
    
    // 2. Invalidate cache
    cacheKey := buildCacheKey(orgID, param.Category, param.Name, param.AppliesTo, param.AppliesToID)
    s.cache.Delete(ctx, cacheKey)
    
    // 3. Invalidate pattern (for wildcard queries)
    s.cache.InvalidatePattern(ctx, fmt.Sprintf("policy7:%s:*", orgID))
}
```

---

## 7. Error Handling

### 7.1 Domain Errors

```go
// internal/domain/errors.go

package domain

import "errors"

var (
    ErrParameterNotFound     = errors.New("parameter not found")
    ErrParameterExists       = errors.New("parameter already exists")
    ErrInvalidParameterValue = errors.New("invalid parameter value")
    ErrUnauthorized          = errors.New("unauthorized")
    ErrForbidden             = errors.New("forbidden")
    ErrInvalidCategory       = errors.New("invalid category")
    ErrCacheUnavailable      = errors.New("cache unavailable")
)
```

### 7.2 Error Response Format

```json
{
  "error": {
    "code": "PARAMETER_NOT_FOUND",
    "message": "Parameter 'teller_transfer_max' not found",
    "details": {
      "category": "transaction_limit",
      "name": "teller_transfer_max"
    }
  }
}
```

---

## 8. Configuration

### 8.1 Environment Variables

```yaml
# .env.example

# Database
DB_HOST=localhost
DB_PORT=5432
DB_NAME=policy7
DB_USER=policy7
DB_PASSWORD="${DB_PASSWORD}"
DB_SSL_MODE=disable

# Redis (optional for v1.0)
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD="${REDIS_PASSWORD}"
REDIS_DB=0
CACHE_TTL_SECONDS=300

# Server
SERVER_PORT=8080
SERVER_READ_TIMEOUT=30s
SERVER_WRITE_TIMEOUT=30s

# Auth7 (for production)
AUTH7_URL=http://auth7:8080
AUTH7_JWKS_URL=http://auth7:8080/.well-known/jwks.json

# Logging
LOG_LEVEL=info
LOG_FORMAT=json
```

---

## 9. Dependencies

### 9.1 External Dependencies

| Service | Purpose | Required |
|---------|---------|----------|
| PostgreSQL 16 | Primary database | ✅ Yes |
| Redis | Cache (hot params) | 🟡 Optional v1.0 |
| auth7 | JWT validation, permissions | 🟡 Optional v1.0 (mock) |

### 9.2 Go Dependencies

```go
// go.mod
require (
    github.com/gin-gonic/gin v1.9.1
    github.com/jackc/pgx/v5 v5.5.0
    github.com/redis/go-redis/v9 v9.3.0
    github.com/google/uuid v1.5.0
    github.com/golang-migrate/migrate/v4 v4.17.0
    github.com/stretchr/testify v1.8.4
)
```

---

## 10. Decisions (Updated 2026-04-27)

| # | Question | Decision | Notes |
|---|----------|----------|-------|
| 1 | **Event streaming untuk parameter changes?** | ✅ **YES v1.0 — NATS** | **Hybrid: Redis (cache) + NATS (events)** |
| 2 | **Conditional parameters (if-then)?** | ✅ **YES v1.0** | Simple JSON logic expression engine |
| 3 | **Parameter inheritance (org → branch → user)?** | ✅ **YES v1.0** | Hierarchical dengan override mechanism |
| 4 | **Rate limiting?** | ✅ **nginx/API gateway** | Basic rate limiting di infrastructure layer |

### 10.1 Architecture Decision: Hybrid Messaging Model

**Pilihan**: **Opsi C — Hybrid Approach**
- **Redis**: Caching layer (hot parameters, session data)
- **NATS**: Event streaming & service communication

#### Why Hybrid?

```
┌─────────────────────────────────────────────────────────────┐
│                     Core7 Ecosystem                          │
│                                                              │
│   ┌─────────────────────────────────────────────────────┐   │
│   │                    Policy7                          │   │
│   │                                                     │   │
│   │  ┌─────────────┐        ┌─────────────────────┐    │   │
│   │  │   Redis     │        │       NATS          │    │   │
│   │  │  (Cache)    │        │  (Event Streaming)  │    │   │
│   │  │             │        │                     │    │   │
│   │  │ • Hot params│        │ • Parameter changes │    │   │
│   │  │ • Sessions  │        │ • Cache invalidation│    │   │
│   │  │ • Rate limit│        │ • Service discovery │    │   │
│   │  └──────┬──────┘        │ • Request-reply     │    │   │
│   │         │               └──────────┬──────────┘    │   │
│   │         │                          │               │   │
│   │    Cache Store                Event Bus           │   │
│   │    (Read-heavy)              (Real-time)          │   │
│   └─────────────────────────────────────────────────────┘   │
│                            │                                 │
│     ┌──────────────────────┼──────────────────────┐         │
│     │                      │                      │         │
│     ▼                      ▼                      ▼         │
│  ┌────────┐           ┌────────┐           ┌────────┐      │
│  │ Auth7  │◄─────────│ Core7  │◄─────────│Workflow│      │
│  │        │   Query  │Enter-  │   Query  │   7    │      │
│  │        │  params  │ prise  │  params  │        │      │
│  └────────┘           └────────┘           └────────┘      │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

#### Redis Responsibilities
- Hot parameter caching (TTL-based)
- Rate limiting counters
- Session metadata (optional)
- Distributed locks (parameter updates)

#### NATS Responsibilities
- Parameter change events (pub/sub)
- Cache invalidation broadcasts
- Service health checks (request-reply)
- Inter-service communication
- Audit log streaming (future)

#### Benefits
1. **Separation of Concerns**: Cache vs. Messaging optimized untuk use case masing-masing
2. **Performance**: Redis untuk read-heavy, NATS untuk real-time events
3. **Reliability**: NATS durable subscriptions ensure delivery
4. **Scalability**: Independent scaling dari cache dan messaging layers

### 10.1 Architecture Alignment with service7-template

✅ **Confirmed**: Policy7 menggunakan clean architecture yang sama dengan service7-template:
- `cmd/` → Entry point
- `internal/api/` → HTTP handlers (Gin)
- `internal/service/` → Business logic
- `internal/store/` → Data access (pgx + sqlc)
- `internal/domain/` → Entities & interfaces
- `pkg/` → Public client library

### 10.2 Parallel Development Support

✅ **Interface-first approach enabled**:
- `AuthClient` interface dengan mock implementation
- Bisa develop Policy7 Plan 01-04 tanpa menunggu Auth7 selesai
- Integration (Plan 05) baru butuh Auth7 gRPC

### 10.3 Open Question Remaining

**Question 1a**: Event streaming technology — pilih yang mana?
- **Option A**: Redis pub/sub (simplest, sudah ada Redis untuk cache)
- **Option B**: NATS (lightweight, good for service mesh)
- **Option C**: Kafka (enterprise grade, overkill untuk v1.0?)

---

*Next: [02-api-detail.md](./02-api-detail.md)*
