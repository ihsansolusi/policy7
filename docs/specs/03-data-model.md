# Policy7 — Spec 03: Data Model

> **Versi**: 0.1-draft | **Tanggal**: 2026-04-27 | **Fase**: Review

---

## 1. Schema Design Principles

| Principle | Description |
|-----------|-------------|
| **UUID Primary Key** | Semua tabel pakai `UUID` (gen_random_uuid()) |
| **Multi-tenant** | Semua tabel memiliki `org_id` untuk isolasi data |
| **Soft Delete** | Menggunakan `is_active` + `effective_until` (bukan hard delete) |
| **Versioned** | Setiap update membuat record baru dengan `version++` |
| **Audit Fields** | Standard: `created_at`, `created_by`, `updated_at` |
| **JSONB Value** | Field `value` menggunakan JSONB untuk fleksibilitas tipe data |
| **snake_case** | Naming convention konsisten |

---

## 2. PostgreSQL Schema

### 2.1 `parameters` — Master Parameter Table

```sql
CREATE TABLE parameters (
    -- Primary Key
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- Multi-tenancy
    org_id              UUID NOT NULL,
    
    -- Parameter Classification
    category            VARCHAR(50) NOT NULL,      -- 'transaction_limit', 'rate', 'fee', 'regulatory', 'operational_hours'
    name                VARCHAR(100) NOT NULL,     -- 'teller_transfer_max', 'deposito_12m_rate'
    
    -- Scope/Applicability
    applies_to          VARCHAR(50) NOT NULL,      -- 'role', 'customer_type', 'product', 'global', 'branch'
    applies_to_id       VARCHAR(100),              -- 'teller', 'vip', 'transfer', null (for global)
    product             VARCHAR(50),               -- 'transfer', 'deposito', null
    
    -- Value (JSONB untuk fleksibilitas)
    value               JSONB NOT NULL,
    value_type          VARCHAR(20) NOT NULL,      -- 'number', 'string', 'boolean', 'json', 'array'
    
    -- Metadata
    unit                VARCHAR(20),               -- 'IDR', 'percent', 'hours', 'days'
    scope               VARCHAR(50),               -- 'per_transaction', 'per_day', 'per_month', null
    
    -- Validity Period
    effective_from      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    effective_until     TIMESTAMPTZ,               -- null = indefinite
    
    -- Versioning
    version             INTEGER NOT NULL DEFAULT 1,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    
    -- Audit
    created_by          UUID,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Constraints
    CONSTRAINT chk_value_type CHECK (value_type IN ('number', 'string', 'boolean', 'json', 'array')),
    CONSTRAINT chk_applies_to CHECK (applies_to IN ('role', 'customer_type', 'product', 'global', 'branch', 'user')),
    CONSTRAINT chk_effective_range CHECK (effective_until IS NULL OR effective_until > effective_from)
);

-- Unique constraint untuk parameter aktif (hanya 1 versi aktif per kombinasi)
CREATE UNIQUE INDEX idx_parameters_unique_active 
ON parameters (org_id, category, name, applies_to, COALESCE(applies_to_id, ''), COALESCE(product, ''))
WHERE is_active = TRUE;

-- Indexes untuk query performance
CREATE INDEX idx_parameters_org ON parameters(org_id);
CREATE INDEX idx_parameters_category ON parameters(org_id, category);
CREATE INDEX idx_parameters_name ON parameters(org_id, category, name);
CREATE INDEX idx_parameters_applies ON parameters(org_id, applies_to, applies_to_id);
CREATE INDEX idx_parameters_product ON parameters(org_id, product) WHERE product IS NOT NULL;
CREATE INDEX idx_parameters_effective ON parameters(org_id, effective_from, effective_until);
CREATE INDEX idx_parameters_active ON parameters(org_id, is_active) WHERE is_active = TRUE;
CREATE INDEX idx_parameters_version ON parameters(org_id, category, name, version);

-- GIN index untuk JSONB value (flexible querying)
CREATE INDEX idx_parameters_value_gin ON parameters USING GIN (value);

-- Composite index untuk query pattern yang umum
CREATE INDEX idx_parameters_lookup ON parameters(org_id, category, name, applies_to, applies_to_id, product, is_active);

COMMENT ON TABLE parameters IS 'Master parameter table with versioning support';
COMMENT ON COLUMN parameters.category IS 'Parameter category: transaction_limit, rate, fee, regulatory, operational_hours, approval_threshold, authorization_limit';
COMMENT ON COLUMN parameters.applies_to IS 'Entity type this parameter applies to: role, customer_type, product, global, branch, user';
COMMENT ON COLUMN parameters.value IS 'JSONB value - can be number, string, object, or array';
COMMENT ON COLUMN parameters.is_active IS 'Only one version active at a time per parameter combination';
```

### 2.2 `parameter_history` — Audit Trail

```sql
CREATE TABLE parameter_history (
    -- Primary Key
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- References
    parameter_id        UUID NOT NULL REFERENCES parameters(id),
    org_id              UUID NOT NULL,
    
    -- Change Details
    previous_value      JSONB,
    new_value           JSONB NOT NULL,
    change_type         VARCHAR(20) NOT NULL,      -- 'create', 'update', 'delete', 'activate', 'deactivate'
    
    -- Version Info
    previous_version    INTEGER,
    new_version         INTEGER NOT NULL,
    
    -- Reason/Context
    change_reason       TEXT,                      -- Alasan perubahan (wajib untuk update)
    change_metadata     JSONB DEFAULT '{}',        -- Additional context: IP, user agent, etc
    
    -- Audit
    changed_by          UUID,
    changed_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Constraints
    CONSTRAINT chk_change_type CHECK (change_type IN ('create', 'update', 'delete', 'activate', 'deactivate'))
);

-- Indexes
CREATE INDEX idx_param_history_parameter ON parameter_history(parameter_id);
CREATE INDEX idx_param_history_org ON parameter_history(org_id);
CREATE INDEX idx_param_history_changed_at ON parameter_history(org_id, changed_at DESC);
CREATE INDEX idx_param_history_change_type ON parameter_history(org_id, change_type);

-- Partitioning untuk history (opsional untuk v1.1)
-- CREATE TABLE parameter_history_2026_01 PARTITION OF parameter_history
--     FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');

COMMENT ON TABLE parameter_history IS 'Audit trail for all parameter changes';
COMMENT ON COLUMN parameter_history.change_reason IS 'Required for update operations - business justification';
```

### 2.3 `parameter_categories` — Category Metadata (Optional)

```sql
CREATE TABLE parameter_categories (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id              UUID NOT NULL,
    
    code                VARCHAR(50) NOT NULL,      -- 'transaction_limit'
    name                VARCHAR(100) NOT NULL,     -- 'Transaction Limits'
    description         TEXT,
    
    -- Configuration
    value_schema        JSONB,                     -- JSON Schema untuk validasi value
    default_value       JSONB,
    
    -- UI Metadata
    display_order       INTEGER DEFAULT 0,
    icon                VARCHAR(50),
    color               VARCHAR(20),
    
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE (org_id, code)
);

CREATE INDEX idx_param_categories_org ON parameter_categories(org_id);
CREATE INDEX idx_param_categories_active ON parameter_categories(org_id, is_active);

COMMENT ON TABLE parameter_categories IS 'Metadata and configuration for parameter categories';
```

---

## 3. JSONB Value Structures

### 3.1 Transaction Limit

```json
{
  "type": "transaction_limit",
  "transaction_limit": 100000000,
  "authorization_limit": 25000000,
  "currency": "IDR",
  "scope": "per_transaction",
  "daily_aggregate": false
}
```

### 3.2 Interest Rate

```json
{
  "type": "interest_rate",
  "rate": 4.5,
  "rate_unit": "percent_per_year",
  "calculation_method": "simple_interest",
  "tenor_months": 12,
  "minimum_amount": 10000000,
  "maximum_amount": null
}
```

### 3.3 Operational Hours

```json
{
  "type": "operational_hours",
  "timezone": "Asia/Jakarta",
  "working_days": ["monday", "tuesday", "wednesday", "thursday", "friday"],
  "hours": {
    "start": "08:00",
    "end": "16:00"
  },
  "breaks": [
    { "start": "12:00", "end": "13:00" }
  ],
  "cutoff_time": "15:30",
  "holidays_excluded": true
}
```

### 3.4 Fee Structure

```json
{
  "type": "fee",
  "fee_type": "flat",
  "fee_amount": 6500,
  "currency": "IDR",
  "minimum_fee": 0,
  "maximum_fee": null,
  "tax_included": true
}
```

### 3.5 Approval Threshold

```json
{
  "type": "approval_threshold",
  "threshold": 50000000,
  "currency": "IDR",
  "approver_roles": ["supervisor"],
  "escalation_required": false
}
```

### 3.6 Authorization Limit (Approver)

```json
{
  "type": "authorization_limit",
  "authorization_max": 100000000,
  "currency": "IDR",
  "scope": "per_transaction",
  "daily_max": 500000000
}
```

### 3.7 Regulatory Threshold

```json
{
  "type": "regulatory_threshold",
  "threshold": 100000000,
  "currency": "IDR",
  "reporting_authority": "PPATK",
  "reporting_deadline_hours": 24,
  "report_type": "CTR"
}
```

---

## 4. Redis Key Patterns

### 4.1 Hot Parameter Cache

```
# Pattern: policy7:{org_id}:{category}:{name}:{applies_to}:{applies_to_id}:{product}
policy7:uuid-bjbs:transaction_limit:teller_transfer_max:role:teller:transfer
policy7:uuid-bjbs:transaction_limit:teller_authorization_limit:role:teller:transfer
policy7:uuid-bjbs:rate:deposito_12m:product:deposito:null
policy7:uuid-bjbs:fee:transfer_atm:product:transfer:ATM

# Wildcard patterns untuk invalidation
policy7:uuid-bjbs:transaction_limit:*
policy7:uuid-bjbs:*:role:teller:*
```

### 4.2 Cache Value Structure

```json
{
  "id": "uuid",
  "value": { ... },
  "version": 3,
  "effective_from": "2026-01-01T00:00:00Z",
  "cached_at": "2026-04-27T10:00:00Z",
  "ttl": 300
}
```

### 4.3 TTL Strategy

| Parameter Type | TTL | Reason |
|----------------|-----|--------|
| Transaction Limits | 5 minutes | Frequent changes |
| Interest Rates | 1 hour | Relatively stable |
| Fees | 15 minutes | Occasional updates |
| Operational Hours | 24 hours | Daily changes |
| Regulatory | 1 hour | Stable |

### 4.4 Cache Invalidation Patterns

```go
// On parameter update
func invalidateParameterCache(orgID, category, name string) {
    // Specific key
    redis.Del(fmt.Sprintf("policy7:%s:%s:%s:*", orgID, category, name))
    
    // Category pattern
    redis.Del(fmt.Sprintf("policy7:%s:%s:*", orgID, category))
    
    // Org-wide pattern (jika perlu)
    // redis.Del(fmt.Sprintf("policy7:%s:*", orgID))
}
```

### 4.5 Cache Invalidation Strategy

#### 4.5.1 Invalidation Triggers

| Trigger | Action | Scope |
|---------|--------|-------|
| **Parameter Created** | Warm cache | Specific key |
| **Parameter Updated** | Invalidate + Warm | Pattern match |
| **Parameter Deleted** | Invalidate | Pattern match |
| **Version Activated** | Invalidate old, Warm new | Pattern match |
| **TTL Expired** | Auto-remove | Specific key |
| **Manual Flush** | Invalidate all | Org-wide or global |

#### 4.5.2 Cache-Aside Pattern (Lazy Loading)

```
Client Request
     ↓
[1] Check Cache (Redis)
     ↓
Cache Hit? ──Yes──→ Return cached value
     ↓ No
[2] Query Database (PostgreSQL)
     ↓
[3] Store in Cache (Redis dengan TTL)
     ↓
[4] Return value
```

```go
// internal/service/cache.go

func (s *ParameterService) GetParameter(ctx context.Context, orgID uuid.UUID, category, name string) (*Parameter, error) {
    const op = "service.ParameterService.GetParameter"
    
    cacheKey := buildCacheKey(orgID, category, name)
    
    // [1] Check cache
    cached, err := s.cache.Get(ctx, cacheKey)
    if err == nil && cached != nil {
        var param Parameter
        if err := json.Unmarshal(cached, &param); err == nil {
            return &param, nil
        }
    }
    
    // [2] Cache miss - query database
    param, err := s.store.GetByName(ctx, orgID, category, name)
    if err != nil {
        return nil, fmt.Errorf("%s: %w", op, err)
    }
    
    // [3] Store in cache
    data, _ := json.Marshal(param)
    ttl := s.getTTLForCategory(category)
    s.cache.Set(ctx, cacheKey, data, ttl)
    
    return param, nil
}
```

#### 4.5.3 Write-Through Pattern (Update)

```
Update Request
     ↓
[1] Update Database (PostgreSQL)
     ↓
[2] Invalidate Cache (Redis DEL)
     ↓ (Don't warm cache - wait for next read)
[3] Publish Event (NATS)
     ↓
[4] Return success
```

```go
func (s *ParameterService) UpdateParameter(ctx context.Context, req *UpdateRequest) (*Parameter, error) {
    const op = "service.ParameterService.UpdateParameter"
    
    // [1] Update database (creates new version)
    param, err := s.store.Update(ctx, req)
    if err != nil {
        return nil, fmt.Errorf("%s: %w", op, err)
    }
    
    // [2] Invalidate cache patterns
    patterns := []string{
        buildCacheKey(req.OrgID, param.Category, param.Name),
        fmt.Sprintf("policy7:%s:%s:*", req.OrgID, param.Category),
    }
    for _, pattern := range patterns {
        if err := s.cache.DelPattern(ctx, pattern); err != nil {
            log.Error().Err(err).Str("pattern", pattern).Msg("cache invalidation failed")
        }
    }
    
    // [3] Publish event for other services
    s.eventBus.Publish(ctx, "policy7.parameter.updated", &ParameterEvent{
        OrgID:    req.OrgID,
        Category: param.Category,
        Name:     param.Name,
        Version:  param.Version,
    })
    
    return param, nil
}
```

#### 4.5.4 Cache Warming Strategy

**Pre-warm on startup:**
```go
func (s *ParameterService) WarmCache(ctx context.Context, orgID uuid.UUID) error {
    const op = "service.ParameterService.WarmCache"
    
    // Get frequently accessed parameters
    hotParams := []string{
        "transaction_limit:teller_transfer_max",
        "transaction_limit:teller_authorization_limit",
        "rate:deposito_12m",
        "fee:transfer_atm",
    }
    
    for _, paramKey := range hotParams {
        parts := strings.Split(paramKey, ":")
        category, name := parts[0], parts[1]
        
        param, err := s.store.GetByName(ctx, orgID, category, name)
        if err != nil {
            log.Warn().Err(err).Str("param", paramKey).Msg("failed to warm cache")
            continue
        }
        
        cacheKey := buildCacheKey(orgID, category, name)
        data, _ := json.Marshal(param)
        s.cache.Set(ctx, cacheKey, data, s.getTTLForCategory(category))
    }
    
    return nil
}
```

#### 4.5.5 Cache Stampede Prevention

**Problem:** Multiple requests hit cache miss simultaneously, causing DB overload.

**Solution: Singleflight (Request Coalescing)**

```go
import "golang.org/x/sync/singleflight"

type ParameterService struct {
    cache      Cache
    store      ParameterStore
    singleflight singleflight.Group
}

func (s *ParameterService) GetParameter(ctx context.Context, orgID uuid.UUID, category, name string) (*Parameter, error) {
    cacheKey := buildCacheKey(orgID, category, name)
    
    // Try cache first
    if cached, err := s.cache.Get(ctx, cacheKey); err == nil {
        return unmarshalParam(cached)
    }
    
    // Use singleflight to prevent stampede
    v, err, _ := s.singleflight.Do(cacheKey, func() (interface{}, error) {
        // Double-check cache (might have been populated by another goroutine)
        if cached, err := s.cache.Get(ctx, cacheKey); err == nil {
            return unmarshalParam(cached)
        }
        
        // Query database
        param, err := s.store.GetByName(ctx, orgID, category, name)
        if err != nil {
            return nil, err
        }
        
        // Populate cache
        data, _ := json.Marshal(param)
        s.cache.Set(ctx, cacheKey, data, s.getTTLForCategory(category))
        
        return param, nil
    })
    
    if err != nil {
        return nil, err
    }
    
    return v.(*Parameter), nil
}
```

#### 4.5.6 Fallback on Cache Miss/Failure

```go
func (s *ParameterService) GetParameterWithFallback(ctx context.Context, orgID uuid.UUID, category, name string) (*Parameter, error) {
    const op = "service.ParameterService.GetParameterWithFallback"
    
    cacheKey := buildCacheKey(orgID, category, name)
    
    // Try cache
    cached, err := s.cache.Get(ctx, cacheKey)
    if err == nil {
        return unmarshalParam(cached)
    }
    
    // Cache miss or error - query DB
    log.Warn().Err(err).Msg("cache miss, querying database")
    
    param, err := s.store.GetByName(ctx, orgID, category, name)
    if err != nil {
        return nil, fmt.Errorf("%s: %w", op, err)
    }
    
    // Async cache population (don't block response)
    go func() {
        data, _ := json.Marshal(param)
        if err := s.cache.Set(context.Background(), cacheKey, data, s.getTTLForCategory(category)); err != nil {
            log.Error().Err(err).Msg("failed to populate cache")
        }
    }()
    
    return param, nil
}
```

#### 4.5.7 Monitoring Cache Performance

```go
// Metrics to track
type CacheMetrics struct {
    Hits        uint64  `json:"hits"`
    Misses      uint64  `json:"misses"`
    HitRate     float64 `json:"hit_rate"`
    Evictions   uint64  `json:"evictions"`
    AvgLatency  float64 `json:"avg_latency_ms"`
}

// Prometheus metrics
var (
    cacheHits = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "policy7_cache_hits_total",
            Help: "Total cache hits",
        },
        []string{"org_id", "category"},
    )
    cacheMisses = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "policy7_cache_misses_total",
            Help: "Total cache misses",
        },
        []string{"org_id", "category"},
    )
)
```

---

## 5. Parameter Inheritance Algorithm

### 5.1 Inheritance Hierarchy

Policy7 mendukung hierarchical parameter resolution:

```
Hierarchy (Highest to Lowest Priority):

Level 5: User-specific (applies_to=user, applies_to_id=user_uuid)
Level 4: Role + Product (applies_to=role, applies_to_id=teller, product=transfer)
Level 3: Role (applies_to=role, applies_to_id=teller, product=null)
Level 2: Product (applies_to=product, applies_to_id=transfer)
Level 1: Global (applies_to=global, applies_to_id=null)
```

### 5.2 Resolution Algorithm

```
FUNCTION GetEffectiveParameter(orgID, category, name, context):
    
    // Context: {role: "teller", product: "transfer", user_id: "uuid", ...}
    
    // Priority order (highest to lowest)
    priorities = [
        {applies_to: "user", applies_to_id: context.user_id},
        {applies_to: "role", applies_to_id: context.role, product: context.product},
        {applies_to: "role", applies_to_id: context.role, product: null},
        {applies_to: "product", applies_to_id: context.product},
        {applies_to: "global", applies_to_id: null}
    ]
    
    FOR priority IN priorities:
        param = QueryDB(
            org_id = orgID
            AND category = category
            AND name = name
            AND applies_to = priority.applies_to
            AND (applies_to_id IS NULL OR applies_to_id = priority.applies_to_id)
            AND (product IS NULL OR product = priority.product)
            AND is_active = TRUE
            AND effective_from <= NOW()
            AND (effective_until IS NULL OR effective_until > NOW())
        )
        
        IF param FOUND:
            RETURN {
                parameter: param,
                resolved_from: priority,
                fallback_used: (priority != priorities[0])
            }
    
    RETURN NOT_FOUND
```

### 5.3 Implementation

```go
// internal/service/parameter.go

func (s *ParameterService) GetEffectiveParameter(ctx context.Context, req *EffectiveParamRequest) (*EffectiveParameter, error) {
    const op = "service.ParameterService.GetEffectiveParameter"
    
    // Build priority list
    priorities := []struct {
        appliesTo   string
        appliesToID *string
        product     *string
    }{
        {"user", &req.UserID, nil},
        {"role", &req.Role, &req.Product},
        {"role", &req.Role, nil},
        {"product", nil, &req.Product},
        {"global", nil, nil},
    }
    
    // Try each priority in order
    for i, priority := range priorities {
        param, err := s.store.GetEffective(ctx, GetEffectiveQuery{
            OrgID:       req.OrgID,
            Category:    req.Category,
            Name:        req.Name,
            AppliesTo:   priority.appliesTo,
            AppliesToID: priority.appliesToID,
            Product:     priority.product,
        })
        
        if err == nil && param != nil {
            return &EffectiveParameter{
                Parameter:    param,
                ResolvedFrom: priority,
                FallbackUsed: i > 0,
                Priority:     i + 1,
            }, nil
        }
        
        if err != nil && !errors.Is(err, ErrNotFound) {
            return nil, fmt.Errorf("%s: %w", op, err)
        }
    }
    
    return nil, ErrParameterNotFound
}
```

### 5.4 Database Query

```sql
-- Query untuk mencari parameter dengan priority tertentu
CREATE OR REPLACE FUNCTION get_effective_parameter(
    p_org_id UUID,
    p_category VARCHAR,
    p_name VARCHAR,
    p_applies_to VARCHAR,
    p_applies_to_id VARCHAR DEFAULT NULL,
    p_product VARCHAR DEFAULT NULL
) RETURNS SETOF parameters AS $$
BEGIN
    RETURN QUERY
    SELECT *
    FROM parameters
    WHERE org_id = p_org_id
      AND category = p_category
      AND name = p_name
      AND applies_to = p_applies_to
      AND (p_applies_to_id IS NULL OR applies_to_id = p_applies_to_id)
      AND (p_product IS NULL OR product = p_product OR product IS NULL)
      AND is_active = TRUE
      AND effective_from <= NOW()
      AND (effective_until IS NULL OR effective_until > NOW())
    ORDER BY 
        CASE WHEN product IS NOT NULL THEN 0 ELSE 1 END,  -- Product-specific first
        version DESC
    LIMIT 1;
END;
$$ LANGUAGE plpgsql;
```

### 5.5 Caching Inheritance Results

```go
func (s *ParameterService) GetEffectiveParameterWithCache(ctx context.Context, req *EffectiveParamRequest) (*EffectiveParameter, error) {
    // Build cache key yang include context
    cacheKey := fmt.Sprintf("policy7:%s:%s:%s:effective:%s:%s:%s",
        req.OrgID,
        req.Category,
        req.Name,
        req.Role,
        req.Product,
        req.UserID,
    )
    
    // Try cache
    if cached, err := s.cache.Get(ctx, cacheKey); err == nil {
        var result EffectiveParameter
        if err := json.Unmarshal(cached, &result); err == nil {
            return &result, nil
        }
    }
    
    // Query dengan inheritance
    result, err := s.GetEffectiveParameter(ctx, req)
    if err != nil {
        return nil, err
    }
    
    // Cache result (shorter TTL karena lebih kompleks)
    data, _ := json.Marshal(result)
    s.cache.Set(ctx, cacheKey, data, 2*time.Minute)  // 2 minutes for effective params
    
    return result, nil
}
```

### 5.6 Override Rules

| Scenario | Behavior |
|----------|----------|
| User-specific exists | Use user-specific (highest priority) |
| Role + Product exists | Use role+product specific |
| Only Role exists | Use role-specific, product = default |
| Only Product exists | Use product-specific |
| Nothing found | Use global |
| Multiple versions | Use latest version (max version number) |

### 5.7 Example Scenarios

**Scenario 1: User Override**
```
Global: teller_transfer_max = 10jt
Role: teller_transfer_max = 15jt (for role=teller)
User: teller_transfer_max = 20jt (for user=john)

Result for user john: 20jt (user-specific wins)
```

**Scenario 2: Role + Product Specific**
```
Role: teller_transfer_max = 10jt (role=teller, product=null)
Role+Product: teller_transfer_max = 15jt (role=teller, product=transfer)

Result for teller doing transfer: 15jt (more specific wins)
Result for teller doing deposit: 10jt (role default)
```

**Scenario 3: Fallback Chain**
```
User: (none)
Role+Product: (none)
Role: (none)
Product: teller_transfer_max = 8jt (product=transfer)
Global: teller_transfer_max = 5jt

Result: 8jt (product-specific, fallback from role)
```

---

## 6. Multi-Tenancy Implementation

### 6.1 Tenant Isolation

```sql
-- Semua query HARUS include org_id filter
SELECT * FROM parameters 
WHERE org_id = $1 
  AND category = $2 
  AND name = $3
  AND is_active = TRUE;
```

### 6.2 Tenant Context Extraction

```go
// Middleware extracts org_id dari JWT claims
func TenantMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        claims := c.Get("claims").(*domain.TokenClaims)
        c.Set("org_id", claims.OrgID)
        c.Next()
    }
}
```

### 6.3 Store Layer Pattern

```go
// internal/store/parameter.go

func (s *parameterStore) GetByName(ctx context.Context, orgID uuid.UUID, category, name string) (*domain.Parameter, error) {
    const op = "store.ParameterStore.GetByName"
    
    query := `
        SELECT id, org_id, category, name, applies_to, applies_to_id, 
               product, value, value_type, unit, scope,
               effective_from, effective_until, version, is_active,
               created_by, created_at, updated_at
        FROM parameters
        WHERE org_id = $1
          AND category = $2
          AND name = $3
          AND is_active = TRUE
          AND effective_from <= NOW()
          AND (effective_until IS NULL OR effective_until > NOW())
        LIMIT 1
    `
    
    var param domain.Parameter
    err := s.db.QueryRow(ctx, query, orgID, category, name).Scan(
        &param.ID, &param.OrgID, &param.Category, &param.Name,
        &param.AppliesTo, &param.AppliesToID, &param.Product,
        &param.Value, &param.ValueType, &param.Unit, &param.Scope,
        &param.EffectiveFrom, &param.EffectiveUntil, &param.Version,
        &param.IsActive, &param.CreatedBy, &param.CreatedAt, &param.UpdatedAt,
    )
    
    if err != nil {
        return nil, fmt.Errorf("%s: %w", op, err)
    }
    
    return &param, nil
}
```

---

## 6. Migration Strategy

### 6.1 Migration Files Structure

```
migrations/
├── 000001_create_parameters_table.up.sql
├── 000001_create_parameters_table.down.sql
├── 000002_create_parameter_history_table.up.sql
├── 000002_create_parameter_history_table.down.sql
├── 000003_create_indexes.up.sql
├── 000003_create_indexes.down.sql
└── 000004_seed_default_categories.up.sql
```

### 6.2 Sample Migration

```sql
-- 000001_create_parameters_table.up.sql

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE parameters (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id              UUID NOT NULL,
    category            VARCHAR(50) NOT NULL,
    name                VARCHAR(100) NOT NULL,
    applies_to          VARCHAR(50) NOT NULL,
    applies_to_id       VARCHAR(100),
    product             VARCHAR(50),
    value               JSONB NOT NULL,
    value_type          VARCHAR(20) NOT NULL,
    unit                VARCHAR(20),
    scope               VARCHAR(50),
    effective_from      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    effective_until     TIMESTAMPTZ,
    version             INTEGER NOT NULL DEFAULT 1,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    created_by          UUID,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_value_type CHECK (value_type IN ('number', 'string', 'boolean', 'json', 'array')),
    CONSTRAINT chk_applies_to CHECK (applies_to IN ('role', 'customer_type', 'product', 'global', 'branch', 'user')),
    CONSTRAINT chk_effective_range CHECK (effective_until IS NULL OR effective_until > effective_from)
);

CREATE UNIQUE INDEX idx_parameters_unique_active 
ON parameters (org_id, category, name, applies_to, COALESCE(applies_to_id, ''), COALESCE(product, ''))
WHERE is_active = TRUE;
```

### 6.3 Migration Commands

```bash
# Up
make migrate-up

# Down (1 step)
make migrate-down

# Version
make migrate-version

# Create new migration
make migrate-create name=add_parameter_categories
```

---

## 7. Performance Considerations

### 7.1 Query Patterns & Index Coverage

| Query Pattern | Index Used |
|---------------|------------|
| Lookup by org + category + name | `idx_parameters_lookup` |
| List by category | `idx_parameters_category` |
| Find active parameters | `idx_parameters_active` |
| Time-based queries | `idx_parameters_effective` |
| JSONB value search | `idx_parameters_value_gin` |

### 7.2 Expected Performance

| Operation | Target Latency | Notes |
|-----------|----------------|-------|
| Single parameter lookup (cached) | < 5ms | Redis hit |
| Single parameter lookup (DB) | < 20ms | Indexed query |
| List by category (100 params) | < 50ms | Index scan |
| Parameter update | < 100ms | Includes version creation |
| Bulk import (100 params) | < 2s | Transaction batch |

### 7.3 Connection Pool

```yaml
# PostgreSQL connection pool
db:
  max_open_conns: 25
  max_idle_conns: 10
  conn_max_lifetime: 5m
  conn_max_idle_time: 10m

# Redis connection pool
redis:
  pool_size: 20
  min_idle_conns: 5
  max_retries: 3
```

---

## 8. Backup & Disaster Recovery

### 8.1 Backup Strategy

#### 8.1.1 PostgreSQL Backups

| Backup Type | Frequency | Retention | Method | Storage |
|-------------|-----------|-----------|--------|---------|
| **Full Backup** | Daily | 30 days | `pg_dump` | S3 (encrypted) |
| **Incremental** | Every 4 hours | 7 days | WAL archiving | S3 (encrypted) |
| **Point-in-Time** | Continuous | 7 days | WAL + Base backup | S3 (encrypted) |

**Backup Schedule:**
```
00:00 - Full backup (daily)
04:00 - Incremental (WAL)
08:00 - Incremental (WAL)
12:00 - Incremental (WAL)
16:00 - Incremental (WAL)
20:00 - Incremental (WAL)
```

**Full Backup Script:**
```bash
#!/bin/bash
# scripts/backup/postgres-full.sh

set -e

BACKUP_DIR="/backups/postgres"
DATE=$(date +%Y%m%d_%H%M%S)
DB_NAME="policy7"
S3_BUCKET="s3://policy7-backups"

# Create backup
pg_dump -h $DB_HOST -U $DB_USER -d $DB_NAME \
    --format=custom \
    --file="$BACKUP_DIR/policy7_${DATE}.dump"

# Compress
gzip "$BACKUP_DIR/policy7_${DATE}.dump"

# Encrypt
openssl enc -aes-256-cbc -salt \
    -in "$BACKUP_DIR/policy7_${DATE}.dump.gz" \
    -out "$BACKUP_DIR/policy7_${DATE}.dump.gz.enc" \
    -pass pass:"$BACKUP_ENCRYPTION_KEY"

# Upload to S3
aws s3 cp "$BACKUP_DIR/policy7_${DATE}.dump.gz.enc" \
    "$S3_BUCKET/full/policy7_${DATE}.dump.gz.enc"

# Cleanup local
rm "$BACKUP_DIR/policy7_${DATE}.dump.gz.enc"

# Verify
aws s3 ls "$S3_BUCKET/full/policy7_${DATE}.dump.gz.enc"
echo "Backup completed: policy7_${DATE}.dump.gz.enc"
```

#### 8.1.2 Redis Backups

| Backup Type | Frequency | Retention | Method |
|-------------|-----------|-----------|--------|
| **RDB Snapshot** | Every 6 hours | 7 days | `BGSAVE` |
| **AOF** | Real-time | 7 days | Append-only file |

**Redis Configuration:**
```conf
# redis.conf
save 900 1      # Save after 900 sec if 1 key changed
save 300 10     # Save after 300 sec if 10 keys changed
save 60 10000   # Save after 60 sec if 10000 keys changed

appendonly yes
appendfsync everysec
```

### 8.2 Disaster Recovery Procedures

#### 8.2.1 RTO & RPO Targets

| Scenario | RTO (Recovery Time Objective) | RPO (Recovery Point Objective) |
|----------|-------------------------------|-------------------------------|
| **Single AZ failure** | < 5 minutes | 0 (Multi-AZ) |
| **Database corruption** | < 30 minutes | < 4 hours |
| **Complete region failure** | < 4 hours | < 24 hours |
| **Catastrophic data loss** | < 8 hours | < 24 hours |

#### 8.2.2 Recovery Procedures

**Scenario 1: Database Corruption (Point-in-Time Recovery)**

```bash
#!/bin/bash
# scripts/disaster-recovery/pitr-recovery.sh

# Stop application
kubectl scale deployment policy7 --replicas=0

# Restore from backup
aws s3 cp s3://policy7-backups/full/policy7_20240427_000000.dump.gz.enc /tmp/

# Decrypt
openssl enc -aes-256-cbc -d \
    -in /tmp/policy7_20240427_000000.dump.gz.enc \
    -out /tmp/policy7_20240427_000000.dump.gz \
    -pass pass:"$BACKUP_ENCRYPTION_KEY"

# Decompress
gunzip /tmp/policy7_20240427_000000.dump.gz

# Restore
dropdb policy7
createdb policy7
pg_restore -d policy7 /tmp/policy7_20240427_000000.dump

# Apply WAL to reach target point-in-time
pg_waldump ...

# Verify
psql -d policy7 -c "SELECT COUNT(*) FROM parameters;"

# Start application
kubectl scale deployment policy7 --replicas=3
```

**Scenario 2: Complete Region Failure (DR Region)**

```
Primary Region (ap-southeast-1)    DR Region (ap-southeast-3)
         │                                    │
    ┌────▼────┐                        ┌────▼────┐
    │Primary  │─────Async Replication──→│ Standby │
    │   DB    │    (WAL streaming)     │   DB    │
    └────┬────┘                        └────┬────┘
         │                                    │
    ┌────▼────┐                        ┌────▼────┐
    │Primary  │                        │ Standby │
    │ Redis   │                        │ Redis   │
    └─────────┘                        └─────────┘

Failover Process:
1. Promote standby DB to primary
2. Update DNS to point to DR region
3. Scale up DR application instances
4. Verify functionality
```

**Failover Runbook:**
```bash
#!/bin/bash
# scripts/disaster-recovery/region-failover.sh

# 1. Stop replication
psql -h dr-db-host -c "SELECT pg_promote();"

# 2. Update DNS
aws route53 change-resource-record-sets \
    --hosted-zone-id $ZONE_ID \
    --change-batch '{
        "Changes": [{
            "Action": "UPSERT",
            "ResourceRecordSet": {
                "Name": "policy7.core7.internal",
                "Type": "CNAME",
                "TTL": 60,
                "ResourceRecords": [{"Value": "dr-policy7.core7.internal"}]
            }
        }]
    }'

# 3. Scale up DR
kubectl --context dr-cluster scale deployment policy7 --replicas=3

# 4. Verify
curl -f https://policy7.core7.internal/health
```

#### 8.2.3 Backup Verification

**Monthly Restore Test:**
```bash
#!/bin/bash
# scripts/backup/verify-backup.sh

# Pick random backup from last 7 days
BACKUP_FILE=$(aws s3 ls s3://policy7-backups/full/ | sort -R | head -1 | awk '{print $4}')

# Restore to test instance
dropdb policy7_test || true
createdb policy7_test

aws s3 cp "s3://policy7-backups/full/$BACKUP_FILE" /tmp/
# Decrypt & decompress ...
pg_restore -d policy7_test /tmp/restore.dump

# Verify data integrity
psql -d policy7_test -c "SELECT 
    COUNT(*) as total_params,
    COUNT(DISTINCT org_id) as orgs,
    MAX(version) as max_version
FROM parameters;"

# Cleanup
dropdb policy7_test
rm /tmp/restore.dump

echo "Backup verification completed: $BACKUP_FILE"
```

### 8.3 High Availability

#### 8.3.1 PostgreSQL HA (Multi-AZ)

```yaml
# docker-compose.ha.yml
version: '3.8'

services:
  postgres-primary:
    image: postgres:16
    environment:
      POSTGRES_DB: policy7
      POSTGRES_USER: policy7
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - pg_primary:/var/lib/postgresql/data
    command: |
      postgres 
      -c wal_level=replica 
      -c max_wal_senders=10 
      -c max_replication_slots=10
    networks:
      - policy7-ha

  postgres-replica:
    image: postgres:16
    environment:
      POSTGRES_DB: policy7
      POSTGRES_USER: policy7
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - pg_replica:/var/lib/postgresql/data
    command: |
      bash -c "
        pg_basebackup -h postgres-primary -D /var/lib/postgresql/data -U replicator -v -P -W &&
        echo 'standby_mode = on' >> /var/lib/postgresql/data/recovery.conf &&
        echo 'primary_conninfo = \"host=postgres-primary port=5432 user=replicator\"' >> /var/lib/postgresql/data/recovery.conf &&
        postgres
      "
    depends_on:
      - postgres-primary
    networks:
      - policy7-ha

  pgpool:
    image: pgpool/pgpool:latest
    environment:
      PGPOOL_BACKEND_NODES: "0:postgres-primary:5432,1:postgres-replica:5432"
      PGPOOL_SR_CHECK_USER: policy7
      PGPOOL_SR_CHECK_PASSWORD: ${DB_PASSWORD}
      PGPOOL_POSTGRES_USERNAME: policy7
      PGPOOL_POSTGRES_PASSWORD: ${DB_PASSWORD}
    ports:
      - "5432:5432"
    depends_on:
      - postgres-primary
      - postgres-replica
    networks:
      - policy7-ha

volumes:
  pg_primary:
  pg_replica:

networks:
  policy7-ha:
    driver: bridge
```

#### 8.3.2 Redis HA (Sentinel)

```yaml
# docker-compose.redis-ha.yml
version: '3.8'

services:
  redis-master:
    image: redis:7-alpine
    volumes:
      - redis_master:/data
    command: redis-server --appendonly yes
    networks:
      - redis-ha

  redis-replica-1:
    image: redis:7-alpine
    command: redis-server --replicaof redis-master 6379
    networks:
      - redis-ha

  redis-replica-2:
    image: redis:7-alpine
    command: redis-server --replicaof redis-master 6379
    networks:
      - redis-ha

  redis-sentinel-1:
    image: redis:7-alpine
    command: redis-sentinel /etc/redis/sentinel.conf
    volumes:
      - ./sentinel.conf:/etc/redis/sentinel.conf
    networks:
      - redis-ha

  redis-sentinel-2:
    image: redis:7-alpine
    command: redis-sentinel /etc/redis/sentinel.conf
    volumes:
      - ./sentinel.conf:/etc/redis/sentinel.conf
    networks:
      - redis-ha

volumes:
  redis_master:

networks:
  redis-ha:
    driver: bridge
```

### 8.4 Monitoring Backups

```yaml
# Prometheus alerts
- alert: Policy7BackupFailed
  expr: time() - policy7_last_successful_backup > 86400
  for: 5m
  labels:
    severity: critical
  annotations:
    summary: "Policy7 backup failed"
    description: "No successful backup in last 24 hours"

- alert: Policy7BackupSizeAnomaly
  expr: |
    (
      policy7_backup_size / 
      avg_over_time(policy7_backup_size[7d])
    ) < 0.5 or (
      policy7_backup_size / 
      avg_over_time(policy7_backup_size[7d])
    ) > 2
  for: 1h
  labels:
    severity: warning
  annotations:
    summary: "Backup size anomaly detected"
```

---

## 9. Data Retention & Archival

### 9.1 Parameter History Retention

| Data Type | Retention | Action |
|-----------|-----------|--------|
| Active parameters | Indefinite | Keep forever |
| Inactive versions | 2 years | Archive to cold storage |
| History records | 5 years | Partition & archive |

### 9.2 Archival Strategy (v1.1)

```sql
-- Archive old inactive parameters
INSERT INTO parameters_archive 
SELECT * FROM parameters 
WHERE is_active = FALSE 
  AND updated_at < NOW() - INTERVAL '2 years';

DELETE FROM parameters 
WHERE is_active = FALSE 
  AND updated_at < NOW() - INTERVAL '2 years';
```

---

## 10. Open Questions

| # | Question | Status |
|---|----------|--------|
| 1 | Partition parameter_history by month? | v1.1 — jika > 1M records/month |
| 2 | Separate table untuk parameter_categories? | Optional — bisa pakai JSONB di code |
| 3 | Full-text search pada parameter name/description? | v1.1 — jika diperlukan |
| 4 | Encrypt sensitive parameter values? | v1.x — untuk PCI DSS compliance |

---

## 11. Summary

### Tables

| Table | Purpose | Rows (est. v1.0) |
|-------|---------|------------------|
| `parameters` | Master parameter data | ~10K per org |
| `parameter_history` | Audit trail | ~100K per org |
| `parameter_categories` | Category metadata | ~50 global |

### Indexes

| Index | Purpose |
|-------|---------|
| `idx_parameters_unique_active` | Ensure 1 active version |
| `idx_parameters_lookup` | Fast parameter lookup |
| `idx_parameters_value_gin` | JSONB querying |

### Next: Spec 04 — Integration

*Spec 03 focuses on data layer. Spec 04 akan membahas integrasi dengan auth7, workflow7, dan services lainnya.*

---

*Next: [04-integration.md](./04-integration.md)*
