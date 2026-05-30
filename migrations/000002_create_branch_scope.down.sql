ALTER TABLE parameters DROP CONSTRAINT IF EXISTS chk_applies_to;
ALTER TABLE parameters ADD CONSTRAINT chk_applies_to CHECK (
    applies_to IN ('role', 'customer_type', 'product', 'global', 'branch', 'user')
);

DROP TABLE IF EXISTS branch_scope;
