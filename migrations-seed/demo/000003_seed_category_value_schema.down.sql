-- Revert demo value_schema seeding: clear value_schema for the 7 base demo
-- categories (restores them to the no-schema state created by 000001).
UPDATE parameter_categories SET value_schema = NULL
WHERE org_id = '00000000-0000-0000-0000-000000000001'
  AND code IN (
    'transaction_limit', 'authorization_limit', 'approval_threshold',
    'rate', 'fee', 'regulatory', 'operational_hours'
  );
