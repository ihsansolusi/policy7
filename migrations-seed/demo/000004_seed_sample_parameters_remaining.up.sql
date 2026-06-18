-- Demo parameters for the remaining 5 categories (000002 only covered
-- transaction_limit + rate), so every category page shows sample rows.
-- value payloads conform to the demo value_schema seeded in 000003.
-- org_id = canonical demo org. value_type='json'.
INSERT INTO parameters
  (id, org_id, category, name, applies_to, applies_to_id, product, value, value_type, unit, scope, effective_from, version, is_active, created_at, updated_at)
VALUES
  -- authorization_limit — global auto-authorization ceiling + a role override
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'authorization_limit', 'auto_authorize_max', 'global', NULL, NULL,
   '{"currency":"IDR","limit_amount":25000000}'::jsonb,
   'json', 'IDR', NULL, NOW(), 1, TRUE, NOW(), NOW()),
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'authorization_limit', 'auto_authorize_max', 'role', 'spv', NULL,
   '{"currency":"IDR","limit_amount":50000000}'::jsonb,
   'json', 'IDR', NULL, NOW(), 1, TRUE, NOW(), NOW()),

  -- approval_threshold — amount above which workflow approval is required
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'approval_threshold', 'workflow_approval_min', 'global', NULL, NULL,
   '{"currency":"IDR","threshold_amount":100000000}'::jsonb,
   'json', 'IDR', NULL, NOW(), 1, TRUE, NOW(), NOW()),

  -- fee — flat wire-transfer fee
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'fee', 'wire_transfer_fee', 'global', NULL, NULL,
   '{"fee_type":"flat","currency":"IDR","fee_amount":15000}'::jsonb,
   'json', 'IDR', NULL, NOW(), 1, TRUE, NOW(), NOW()),

  -- regulatory — CTR reporting threshold
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'regulatory', 'ctr_reporting_threshold', 'global', NULL, NULL,
   '{"currency":"IDR","ctr_threshold":500000000,"report_code":"CTR-001"}'::jsonb,
   'json', 'IDR', NULL, NOW(), 1, TRUE, NOW(), NOW()),

  -- operational_hours — branch service window
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'operational_hours', 'branch_service_hours', 'global', NULL, NULL,
   '{"open_time":"08:00","close_time":"15:00","cutoff_time":"14:00"}'::jsonb,
   'json', NULL, NULL, NOW(), 1, TRUE, NOW(), NOW());
