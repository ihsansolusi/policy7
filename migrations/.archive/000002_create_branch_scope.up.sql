-- branch_scope: projection synced from enterprise /v1/source-contracts/branch-scope
-- Powers the BRANCH_TYPE tier in Option C parameter resolution.
CREATE TABLE IF NOT EXISTS branch_scope (
    branch_id        UUID          PRIMARY KEY,
    org_id           VARCHAR(36)   NOT NULL,
    branch_type      VARCHAR(50),                  -- matches enterprise branch_types.code
    parent_branch_id UUID,
    updated_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    synced_at        TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    UNIQUE (org_id, branch_id)
);
CREATE INDEX IF NOT EXISTS idx_branch_scope_org_id      ON branch_scope (org_id);
CREATE INDEX IF NOT EXISTS idx_branch_scope_branch_type ON branch_scope (org_id, branch_type);

-- Extend applies_to constraint to support branch_type tier in Option C resolution.
ALTER TABLE parameters DROP CONSTRAINT IF EXISTS chk_applies_to;
ALTER TABLE parameters ADD CONSTRAINT chk_applies_to CHECK (
    applies_to IN ('role', 'customer_type', 'product', 'global', 'branch', 'branch_type', 'user')
);
