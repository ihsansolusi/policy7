-- Sample seed data for policy7 admin UI verification.
-- Covers all 6 categories the bos7-enterprise pages display.
-- Org: 00000000-0000-0000-0000-000000000001 (default seed org)

BEGIN;

DELETE FROM parameters WHERE org_id = '00000000-0000-0000-0000-000000000001';

-- ── FEES per product ─────────────────────────────────────────────────────────
INSERT INTO parameters (org_id, category, name, applies_to, applies_to_id, product, value, value_type, unit) VALUES
('00000000-0000-0000-0000-000000000001', 'fees', 'admin_fee_monthly',  'global', NULL, 'tabungan',  '{"fee_name":"Admin Bulanan","flat_amount":5000,"currency":"IDR"}'::jsonb, 'json', 'IDR'),
('00000000-0000-0000-0000-000000000001', 'fees', 'atm_withdrawal',     'global', NULL, 'tabungan',  '{"fee_name":"Tarik Tunai ATM","flat_amount":7500,"currency":"IDR"}'::jsonb, 'json', 'IDR'),
('00000000-0000-0000-0000-000000000001', 'fees', 'closing_fee',        'global', NULL, 'tabungan',  '{"fee_name":"Penutupan Rekening","flat_amount":25000,"currency":"IDR"}'::jsonb, 'json', 'IDR'),
('00000000-0000-0000-0000-000000000001', 'fees', 'admin_fee_monthly',  'global', NULL, 'giro',      '{"fee_name":"Admin Bulanan","flat_amount":35000,"currency":"IDR"}'::jsonb, 'json', 'IDR'),
('00000000-0000-0000-0000-000000000001', 'fees', 'cheque_book',        'global', NULL, 'giro',      '{"fee_name":"Buku Cek","flat_amount":150000,"currency":"IDR"}'::jsonb, 'json', 'IDR'),
('00000000-0000-0000-0000-000000000001', 'fees', 'rtgs',               'global', NULL, 'transfer',  '{"fee_name":"RTGS","flat_amount":30000,"min_amount":100000000,"currency":"IDR"}'::jsonb, 'json', 'IDR'),
('00000000-0000-0000-0000-000000000001', 'fees', 'sknbi',              'global', NULL, 'transfer',  '{"fee_name":"SKNBI Kliring","flat_amount":2900,"max_amount":1000000000,"currency":"IDR"}'::jsonb, 'json', 'IDR'),
('00000000-0000-0000-0000-000000000001', 'fees', 'bi_fast',            'global', NULL, 'transfer',  '{"fee_name":"BI-FAST","flat_amount":2500,"currency":"IDR"}'::jsonb, 'json', 'IDR'),
('00000000-0000-0000-0000-000000000001', 'fees', 'early_termination',  'global', NULL, 'deposito',  '{"fee_name":"Pencairan Sebelum Jatuh Tempo","rate_pct":1.5,"currency":"IDR"}'::jsonb, 'json', 'IDR'),
('00000000-0000-0000-0000-000000000001', 'fees', 'late_payment',       'global', NULL, 'pembiayaan','{"fee_name":"Denda Keterlambatan","rate_pct":0.05,"currency":"IDR"}'::jsonb, 'json', 'IDR'),
('00000000-0000-0000-0000-000000000001', 'fees', 'sknbi_outgoing',     'global', NULL, 'kliring',   '{"fee_name":"Kliring Keluar","flat_amount":3500,"currency":"IDR"}'::jsonb, 'json', 'IDR');

-- ── RATES per product ────────────────────────────────────────────────────────
INSERT INTO parameters (org_id, category, name, applies_to, applies_to_id, product, value, value_type) VALUES
('00000000-0000-0000-0000-000000000001', 'rates', 'savings_rate',        'global', NULL, 'tabungan',  '{"rate_name":"Bunga Tabungan","rate_pct":2.25,"currency":"IDR"}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'rates', 'deposito_1m',         'global', NULL, 'deposito',  '{"rate_name":"Deposito 1 Bulan","rate_pct":3.50,"tenor_days":30,"min_balance":10000000,"currency":"IDR"}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'rates', 'deposito_3m',         'global', NULL, 'deposito',  '{"rate_name":"Deposito 3 Bulan","rate_pct":3.75,"tenor_days":90,"min_balance":10000000,"currency":"IDR"}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'rates', 'deposito_6m',         'global', NULL, 'deposito',  '{"rate_name":"Deposito 6 Bulan","rate_pct":4.00,"tenor_days":180,"min_balance":10000000,"currency":"IDR"}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'rates', 'deposito_12m',        'global', NULL, 'deposito',  '{"rate_name":"Deposito 12 Bulan","rate_pct":4.25,"tenor_days":365,"min_balance":10000000,"currency":"IDR"}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'rates', 'giro_rate',           'global', NULL, 'giro',      '{"rate_name":"Bunga Giro","rate_pct":1.00,"currency":"IDR"}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'rates', 'kpr_floating',        'global', NULL, 'pembiayaan','{"rate_name":"KPR Floating","rate_pct":7.25,"effective_date":"2026-01-01","currency":"IDR"}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'rates', 'kkb',                 'global', NULL, 'pembiayaan','{"rate_name":"Kredit Kendaraan Bermotor","rate_pct":8.50,"effective_date":"2026-01-01","currency":"IDR"}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'rates', 'kur',                 'global', NULL, 'pembiayaan','{"rate_name":"Kredit Usaha Rakyat","rate_pct":6.00,"effective_date":"2026-01-01","currency":"IDR"}'::jsonb, 'json');

-- ── REGULATORY THRESHOLDS ────────────────────────────────────────────────────
-- applies_to_id holds the regulatory type (ctr, str, bi_giro_wajib, ojk_car, kyc).
-- applies_to is constrained to {role,customer_type,product,global,branch,user},
-- so we use 'global' here and let the UI filter client-side on applies_to_id.
INSERT INTO parameters (org_id, category, name, applies_to, applies_to_id, value, value_type) VALUES
('00000000-0000-0000-0000-000000000001', 'regulatory', 'cash_transaction_500m',  'global', 'ctr',           '{"issuer":"PPATK","reg_name":"Cash Transaction Report (Rp 500jt)","effective_date":"2010-01-01","status":"ACTIVE","threshold":500000000}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'regulatory', 'suspicious_activity',    'global', 'str',           '{"issuer":"PPATK","reg_name":"Suspicious Transaction Report","effective_date":"2010-01-01","status":"ACTIVE","notes":"No threshold; based on behavioural rules"}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'regulatory', 'bi_giro_wajib',          'global', 'bi_giro_wajib', '{"issuer":"Bank Indonesia","reg_name":"Giro Wajib Minimum","effective_date":"2024-09-01","status":"ACTIVE","rate_pct":9.0}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'regulatory', 'ojk_capital_adequacy',   'global', 'ojk_car',       '{"issuer":"OJK","reg_name":"Capital Adequacy Ratio (CAR)","effective_date":"2014-01-01","status":"ACTIVE","rate_pct":8.0}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'regulatory', 'kyc_pep_threshold',      'global', 'kyc',           '{"issuer":"OJK","reg_name":"PEP Enhanced Due Diligence","effective_date":"2017-08-01","status":"ACTIVE","threshold":100000000}'::jsonb, 'json');

-- ── OPERATIONAL HOURS per day ────────────────────────────────────────────────
INSERT INTO parameters (org_id, category, name, applies_to, applies_to_id, value, value_type) VALUES
('00000000-0000-0000-0000-000000000001', 'operational_hours', 'branch_senin',  'global', NULL, '{"day_of_week":"senin","open_time":"08:00","close_time":"15:00","is_open":true,"branch_code":"ALL"}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'operational_hours', 'branch_selasa', 'global', NULL, '{"day_of_week":"selasa","open_time":"08:00","close_time":"15:00","is_open":true,"branch_code":"ALL"}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'operational_hours', 'branch_rabu',   'global', NULL, '{"day_of_week":"rabu","open_time":"08:00","close_time":"15:00","is_open":true,"branch_code":"ALL"}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'operational_hours', 'branch_kamis',  'global', NULL, '{"day_of_week":"kamis","open_time":"08:00","close_time":"15:00","is_open":true,"branch_code":"ALL"}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'operational_hours', 'branch_jumat',  'global', NULL, '{"day_of_week":"jumat","open_time":"08:00","close_time":"15:30","is_open":true,"branch_code":"ALL","notes":"Tutup istirahat Jumat 11:30-13:00"}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'operational_hours', 'branch_sabtu',  'global', NULL, '{"day_of_week":"sabtu","open_time":"00:00","close_time":"00:00","is_open":false,"branch_code":"ALL"}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'operational_hours', 'branch_minggu', 'global', NULL, '{"day_of_week":"minggu","open_time":"00:00","close_time":"00:00","is_open":false,"branch_code":"ALL"}'::jsonb, 'json');

-- ── APPROVAL THRESHOLDS per product ──────────────────────────────────────────
INSERT INTO parameters (org_id, category, name, applies_to, applies_to_id, product, value, value_type) VALUES
('00000000-0000-0000-0000-000000000001', 'approval_threshold', 'teller_self_auth',     'role', 'teller',          'transfer',  '{"threshold_name":"Teller Self Authorization","min_amount":0,"max_amount":25000000,"approver_level":"teller","currency":"IDR","effective_date":"2026-01-01"}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'approval_threshold', 'supervisor_approval',  'role', 'supervisor',      'transfer',  '{"threshold_name":"Supervisor Approval","min_amount":25000001,"max_amount":100000000,"approver_level":"supervisor","currency":"IDR","effective_date":"2026-01-01"}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'approval_threshold', 'branch_manager_apv',   'role', 'branch_manager',  'transfer',  '{"threshold_name":"Branch Manager Approval","min_amount":100000001,"max_amount":500000000,"approver_level":"branch_manager","currency":"IDR","effective_date":"2026-01-01"}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'approval_threshold', 'head_office_apv',      'role', 'head_office',     'transfer',  '{"threshold_name":"Head Office Approval","min_amount":500000001,"approver_level":"head_office","currency":"IDR","effective_date":"2026-01-01"}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'approval_threshold', 'kpr_branch_manager',   'role', 'branch_manager',  'pembiayaan','{"threshold_name":"KPR Branch Manager","min_amount":0,"max_amount":1000000000,"approver_level":"branch_manager","currency":"IDR","effective_date":"2026-01-01"}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'approval_threshold', 'kpr_committee',        'role', 'credit_committee','pembiayaan','{"threshold_name":"KPR Credit Committee","min_amount":1000000001,"approver_level":"credit_committee","currency":"IDR","effective_date":"2026-01-01"}'::jsonb, 'json');

-- ── TRANSACTION LIMITS per channel ───────────────────────────────────────────
INSERT INTO parameters (org_id, category, name, applies_to, applies_to_id, value, value_type) VALUES
('00000000-0000-0000-0000-000000000001', 'transaction_limit', 'atm_withdrawal_daily',   'role', 'customer', '{"limit_name":"ATM Withdrawal Daily","max_amount":10000000,"daily_limit":10000000,"channel":"atm","currency":"IDR"}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'transaction_limit', 'atm_transfer_daily',     'role', 'customer', '{"limit_name":"ATM Transfer Daily","max_amount":25000000,"daily_limit":25000000,"channel":"atm","currency":"IDR"}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'transaction_limit', 'mobile_transfer_daily',  'role', 'customer', '{"limit_name":"Mobile Transfer Daily","max_amount":50000000,"daily_limit":50000000,"channel":"mobile","currency":"IDR"}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'transaction_limit', 'mobile_topup_daily',     'role', 'customer', '{"limit_name":"Mobile E-Wallet Topup","max_amount":20000000,"daily_limit":20000000,"channel":"mobile","currency":"IDR"}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'transaction_limit', 'ib_transfer_daily',      'role', 'customer', '{"limit_name":"Internet Banking Transfer","max_amount":500000000,"daily_limit":500000000,"channel":"internet","currency":"IDR"}'::jsonb, 'json'),
('00000000-0000-0000-0000-000000000001', 'transaction_limit', 'teller_cash_daily',      'role', 'teller',   '{"limit_name":"Teller Cash Daily","max_amount":1000000000,"daily_limit":1000000000,"channel":"teller","currency":"IDR"}'::jsonb, 'json');

COMMIT;

-- Verify counts per category
SELECT category, COUNT(*) FROM parameters WHERE org_id = '00000000-0000-0000-0000-000000000001' GROUP BY category ORDER BY category;
