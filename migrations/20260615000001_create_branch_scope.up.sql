CREATE TABLE IF NOT EXISTS branch_scope (
    branch_id                        UUID PRIMARY KEY,
    org_id                           UUID NOT NULL,
    branch_type                      VARCHAR(50) NOT NULL DEFAULT '',
    parent_branch_id                 UUID,
    updated_at                       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    synced_at                        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_branch_scope_org_id_branch_id UNIQUE (org_id, branch_id)
);

CREATE INDEX IF NOT EXISTS idx_branch_scope_org_id ON branch_scope(org_id);
CREATE INDEX IF NOT EXISTS idx_branch_scope_branch_type
    ON branch_scope(org_id, branch_type);