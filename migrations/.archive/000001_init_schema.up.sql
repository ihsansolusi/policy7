CREATE TABLE parameters (
    -- Primary Key
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- Multi-tenancy
    org_id              UUID NOT NULL,
    
    -- Parameter Classification
    category            VARCHAR(50) NOT NULL,      -- 'transaction_limit', 'rate', 'fee', 'regulatory', 'operational_hours'
    name                VARCHAR(100) NOT NULL,     -- 'teller_transfer_max', 'deposito_12m_rate'
    
    -- Scope/Applicability
    applies_to          VARCHAR(50) NOT NULL,      -- 'role', 'customer_type', 'product', 'global', 'branch', 'user'
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

COMMENT ON TABLE parameter_history IS 'Audit trail for all parameter changes';
COMMENT ON COLUMN parameter_history.change_reason IS 'Required for update operations - business justification';


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
