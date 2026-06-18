-- Remove demo parameters seeded by 000004 (the 5 non-transaction/rate categories).
DELETE FROM parameters
WHERE org_id = '00000000-0000-0000-0000-000000000001'
  AND ( (category = 'authorization_limit' AND name = 'auto_authorize_max')
     OR (category = 'approval_threshold'  AND name = 'workflow_approval_min')
     OR (category = 'fee'                 AND name = 'wire_transfer_fee')
     OR (category = 'regulatory'           AND name = 'ctr_reporting_threshold')
     OR (category = 'operational_hours'   AND name = 'branch_service_hours') );
