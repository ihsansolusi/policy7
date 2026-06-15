DO $$ BEGIN
    CREATE TYPE parameters_applies_to_enum AS ENUM ('global', 'branch_type', 'branch', 'role', 'user', 'product');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE parameters_value_type_enum AS ENUM ('number', 'string', 'boolean', 'json', 'array');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS parameters (
    id                               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                           UUID NOT NULL,
    category                         VARCHAR(50) NOT NULL DEFAULT '',
    name                             VARCHAR(100) NOT NULL DEFAULT '',
    applies_to                       parameters_applies_to_enum NOT NULL DEFAULT 'global',
    applies_to_id                    VARCHAR(100),
    product                          VARCHAR(50),
    value                            JSONB NOT NULL DEFAULT '{}',
    value_type                       parameters_value_type_enum NOT NULL DEFAULT 'number',
    unit                             VARCHAR(20),
    scope                            VARCHAR(50),
    effective_from                   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    effective_until                  TIMESTAMPTZ,
    version                          INTEGER NOT NULL DEFAULT 1,
    is_active                        BOOLEAN NOT NULL DEFAULT true,
    created_by                       UUID,
    created_at                       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by                       UUID,
    updated_at                       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_parameters_effective_range CHECK (effective_until IS NULL OR effective_until > effective_from),
    CONSTRAINT chk_parameters_scope_id CHECK (applies_to = 'global' OR applies_to_id IS NOT NULL),
    CONSTRAINT chk_parameters_product_scope CHECK (applies_to <> 'product' OR product IS NULL)
);

CREATE INDEX IF NOT EXISTS idx_parameters_org_id ON parameters(org_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_parameters_unique_active
    ON parameters(org_id, category, name, applies_to, COALESCE(applies_to_id, ''), COALESCE(product, ''))
    WHERE is_active = TRUE;
CREATE INDEX IF NOT EXISTS idx_parameters_category
    ON parameters(org_id, category);
CREATE INDEX IF NOT EXISTS idx_parameters_name
    ON parameters(org_id, category, name);
CREATE INDEX IF NOT EXISTS idx_parameters_applies
    ON parameters(org_id, applies_to, applies_to_id);
CREATE INDEX IF NOT EXISTS idx_parameters_product
    ON parameters(org_id, product)
    WHERE product IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_parameters_effective
    ON parameters(org_id, effective_from, effective_until);
CREATE INDEX IF NOT EXISTS idx_parameters_active
    ON parameters(org_id, is_active)
    WHERE is_active = TRUE;
CREATE INDEX IF NOT EXISTS idx_parameters_version
    ON parameters(org_id, category, name, version);
CREATE INDEX IF NOT EXISTS idx_parameters_value_gin
    ON parameters USING gin (value);
CREATE INDEX IF NOT EXISTS idx_parameters_lookup
    ON parameters(org_id, category, name, applies_to, applies_to_id, product, is_active);