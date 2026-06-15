CREATE TABLE IF NOT EXISTS parameter_categories (
    id                               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                           UUID NOT NULL,
    code                             VARCHAR(50) NOT NULL DEFAULT '',
    name                             VARCHAR(100) NOT NULL DEFAULT '',
    description                      VARCHAR(500),
    value_schema                     JSONB,
    default_value                    JSONB,
    display_order                    INTEGER NOT NULL DEFAULT 0,
    icon                             VARCHAR(50),
    color                            VARCHAR(20),
    is_active                        BOOLEAN NOT NULL DEFAULT true,
    created_by                       UUID,
    created_at                       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by                       UUID,
    updated_at                       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_parameter_categories_org_id_code UNIQUE (org_id, code)
);

CREATE INDEX IF NOT EXISTS idx_parameter_categories_org_id ON parameter_categories(org_id);
CREATE INDEX IF NOT EXISTS idx_param_categories_active
    ON parameter_categories(org_id, is_active);