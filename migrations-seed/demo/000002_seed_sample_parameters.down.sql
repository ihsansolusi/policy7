DELETE FROM parameters
WHERE org_id = '00000000-0000-0000-0000-000000000001'
  AND ( (category = 'transaction_limit' AND name = 'teller_transfer_max')
     OR (category = 'rate'              AND name = 'deposito_12m') );
