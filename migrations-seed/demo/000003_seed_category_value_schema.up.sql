-- Demo value_schema for the 7 base parameter categories (canonical demo org).
-- These exercise the schema-driven dynamic value forms in bos7-enterprise
-- (Wave C #570): JSON Schema + x-ui (UI hints) + x-rules (cross-field rules).
--
-- DEMO data only — intended to make the policy module fully explorable. The
-- canonical value_schema authoring is owned by #568; replace as needed.
-- Idempotent: plain UPDATEs scoped by (org_id, code).

-- transaction_limit — two-limit pattern + currency + lte rule (auth <= tx).
UPDATE parameter_categories SET value_schema = '{
  "type": "object",
  "required": ["currency", "transaction_limit", "authorization_limit"],
  "x-rules": [
    { "op": "lte", "left": "authorization_limit", "right": "transaction_limit",
      "message": "Authorization limit must be <= transaction limit" }
  ],
  "properties": {
    "currency": { "type": "string", "enum": ["IDR", "USD"], "default": "IDR",
      "x-ui": { "label": "Currency", "widget": "select", "span": 4, "order": 1 } },
    "transaction_limit": { "type": "number", "minimum": 0,
      "x-ui": { "label": "Transaction Limit", "span": 4, "order": 2, "numeric": { "kind": "currency", "currencyField": "currency" } } },
    "authorization_limit": { "type": "number", "minimum": 0,
      "x-ui": { "label": "Authorization Limit", "span": 4, "order": 3, "numeric": { "kind": "currency", "currencyField": "currency" } } },
    "scope": { "type": "string", "enum": ["per_transaction", "per_day"], "default": "per_transaction",
      "x-ui": { "label": "Scope", "widget": "select", "span": 6, "order": 4 } }
  }
}'::jsonb
WHERE org_id = '00000000-0000-0000-0000-000000000001' AND code = 'transaction_limit';

-- authorization_limit — single auto-authorization amount.
UPDATE parameter_categories SET value_schema = '{
  "type": "object",
  "required": ["currency", "limit_amount"],
  "properties": {
    "currency": { "type": "string", "enum": ["IDR", "USD"], "default": "IDR",
      "x-ui": { "label": "Currency", "widget": "select", "span": 6, "order": 1 } },
    "limit_amount": { "type": "number", "minimum": 0,
      "x-ui": { "label": "Authorization Limit", "span": 6, "order": 2, "numeric": { "kind": "currency", "currencyField": "currency" } } }
  }
}'::jsonb
WHERE org_id = '00000000-0000-0000-0000-000000000001' AND code = 'authorization_limit';

-- approval_threshold — amount above which workflow approval is required.
UPDATE parameter_categories SET value_schema = '{
  "type": "object",
  "required": ["currency", "threshold_amount"],
  "properties": {
    "currency": { "type": "string", "enum": ["IDR", "USD"], "default": "IDR",
      "x-ui": { "label": "Currency", "widget": "select", "span": 6, "order": 1 } },
    "threshold_amount": { "type": "number", "minimum": 0,
      "x-ui": { "label": "Approval Threshold", "span": 6, "order": 2, "numeric": { "kind": "currency", "currencyField": "currency" } } }
  }
}'::jsonb
WHERE org_id = '00000000-0000-0000-0000-000000000001' AND code = 'approval_threshold';

-- rate — percentage rate + basis.
UPDATE parameter_categories SET value_schema = '{
  "type": "object",
  "required": ["rate"],
  "properties": {
    "rate": { "type": "number", "minimum": 0, "maximum": 100,
      "x-ui": { "label": "Rate (%)", "span": 6, "order": 1, "numeric": { "kind": "percent" } } },
    "basis": { "type": "string", "enum": ["annual", "monthly"], "default": "annual",
      "x-ui": { "label": "Basis", "widget": "select", "span": 6, "order": 2 } }
  }
}'::jsonb
WHERE org_id = '00000000-0000-0000-0000-000000000001' AND code = 'rate';

-- fee — flat amount or percentage fee.
UPDATE parameter_categories SET value_schema = '{
  "type": "object",
  "required": ["currency", "fee_amount"],
  "properties": {
    "fee_type": { "type": "string", "enum": ["flat", "percent"], "default": "flat",
      "x-ui": { "label": "Fee Type", "widget": "select", "span": 4, "order": 1 } },
    "currency": { "type": "string", "enum": ["IDR", "USD"], "default": "IDR",
      "x-ui": { "label": "Currency", "widget": "select", "span": 4, "order": 2 } },
    "fee_amount": { "type": "number", "minimum": 0,
      "x-ui": { "label": "Fee Amount", "span": 4, "order": 3, "numeric": { "kind": "currency", "currencyField": "currency" } } }
  }
}'::jsonb
WHERE org_id = '00000000-0000-0000-0000-000000000001' AND code = 'fee';

-- regulatory — CTR/STR reporting threshold.
UPDATE parameter_categories SET value_schema = '{
  "type": "object",
  "required": ["currency", "ctr_threshold"],
  "properties": {
    "currency": { "type": "string", "enum": ["IDR", "USD"], "default": "IDR",
      "x-ui": { "label": "Currency", "widget": "select", "span": 6, "order": 1 } },
    "ctr_threshold": { "type": "number", "minimum": 0,
      "x-ui": { "label": "CTR Threshold", "span": 6, "order": 2, "numeric": { "kind": "currency", "currencyField": "currency" } } },
    "report_code": { "type": "string", "maxLength": 32,
      "x-ui": { "label": "Report Code", "span": 6, "order": 3, "placeholder": "CTR-001" } }
  }
}'::jsonb
WHERE org_id = '00000000-0000-0000-0000-000000000001' AND code = 'regulatory';

-- operational_hours — service open/close/cut-off times (HH:MM).
UPDATE parameter_categories SET value_schema = '{
  "type": "object",
  "required": ["open_time", "close_time"],
  "properties": {
    "open_time": { "type": "string",
      "x-ui": { "label": "Open Time", "span": 4, "order": 1, "placeholder": "08:00" } },
    "close_time": { "type": "string",
      "x-ui": { "label": "Close Time", "span": 4, "order": 2, "placeholder": "15:00" } },
    "cutoff_time": { "type": "string",
      "x-ui": { "label": "Cut-off Time", "span": 4, "order": 3, "placeholder": "14:00" } }
  }
}'::jsonb
WHERE org_id = '00000000-0000-0000-0000-000000000001' AND code = 'operational_hours';
