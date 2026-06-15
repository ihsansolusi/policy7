-- Demo: base parameter categories (same set as prod baseline) for the canonical org.
INSERT INTO parameter_categories (id, org_id, code, name, description, display_order, is_active, created_at, updated_at)
VALUES
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'transaction_limit',   'Transaction Limits',    'Employee & customer transaction limits (two-limit pattern)', 1, TRUE, NOW(), NOW()),
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'authorization_limit', 'Authorization Limits',  'Auto-authorization thresholds for approvers',                2, TRUE, NOW(), NOW()),
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'approval_threshold',  'Approval Thresholds',   'Amounts above which workflow approval is required',          3, TRUE, NOW(), NOW()),
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'rate',                'Interest Rates',        'Deposit & financing interest rates',                         4, TRUE, NOW(), NOW()),
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'fee',                 'Fees',                  'Transaction & service fees',                                 5, TRUE, NOW(), NOW()),
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'regulatory',          'Regulatory Thresholds', 'CTR/STR and other regulatory thresholds',                    6, TRUE, NOW(), NOW()),
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'operational_hours',   'Operational Hours',     'Service & cut-off operational hours',                        7, TRUE, NOW(), NOW())
ON CONFLICT (org_id, code) DO NOTHING;
