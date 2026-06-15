DELETE FROM parameter_categories
WHERE org_id = '00000000-0000-0000-0000-000000000001'
  AND code IN ('transaction_limit','authorization_limit','approval_threshold','rate','fee','regulatory','operational_hours');
