-- Demo parameters illustrating the two-limit pattern + the inheritance hierarchy
-- (spec §5.1). org_id = canonical demo org. value_type='json' (object payload in `value`).
INSERT INTO parameters
  (id, org_id, category, name, applies_to, applies_to_id, product, value, value_type, unit, scope, effective_from, version, is_active, created_at, updated_at)
VALUES
  -- Level 1 — global default teller transfer limit
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'transaction_limit', 'teller_transfer_max', 'global', NULL, NULL,
   '{"transaction_limit":50000000,"authorization_limit":10000000,"currency":"IDR","scope":"per_transaction"}'::jsonb,
   'json', 'IDR', 'per_transaction', NOW(), 1, TRUE, NOW(), NOW()),

  -- Level 3 — role=teller override
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'transaction_limit', 'teller_transfer_max', 'role', 'teller', NULL,
   '{"transaction_limit":100000000,"authorization_limit":25000000,"currency":"IDR","scope":"per_transaction"}'::jsonb,
   'json', 'IDR', 'per_transaction', NOW(), 1, TRUE, NOW(), NOW()),

  -- Level 4 — role=teller + product=transfer (secondary product dimension)
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'transaction_limit', 'teller_transfer_max', 'role', 'teller', 'transfer',
   '{"transaction_limit":150000000,"authorization_limit":30000000,"currency":"IDR","scope":"per_transaction"}'::jsonb,
   'json', 'IDR', 'per_transaction', NOW(), 1, TRUE, NOW(), NOW()),

  -- Level 2 — product-scoped deposito 12m rate (product code in applies_to_id; product column NULL)
  (gen_random_uuid(), '00000000-0000-0000-0000-000000000001', 'rate', 'deposito_12m', 'product', 'deposito', NULL,
   '{"rate":4.5,"rate_unit":"percent_per_year","calculation_method":"simple_interest","tenor_months":12}'::jsonb,
   'json', 'percent', NULL, NOW(), 1, TRUE, NOW(), NOW());
