#!/usr/bin/env python3
"""
transform.py — generate policy7 demo seed SQL.

No external data source — emits hand-curated parameters for the two-limit
pattern, sample rates, fees, regulatory thresholds, and operational hours.

The output is idempotent via ON CONFLICT DO UPDATE on the deterministic UUID id.
"""

from __future__ import annotations

import json
import sys
import uuid
from pathlib import Path

ROOT = Path(__file__).parent
NS = uuid.UUID("11111111-1111-1111-1111-111111111111")


def deterministic_uuid(prefix: str, key: str) -> str:
    return str(uuid.uuid5(NS, f"{prefix}:{key}"))


def t(v: str | None) -> str:
    if v is None or v == "" or v.upper() == "NULL":
        return "NULL"
    return "'" + v.replace("'", "''") + "'"


def j(obj) -> str:
    return "'" + json.dumps(obj).replace("'", "''") + "'::jsonb"


def load_env() -> dict[str, str]:
    env: dict[str, str] = {}
    p = ROOT / ".env.seed"
    if p.exists():
        for line in p.read_text().splitlines():
            line = line.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            k, val = line.split("=", 1)
            env[k.strip()] = val.strip()
    return env


# ─── parameter_categories ────────────────────────────────────────────────────

CATEGORIES = [
    ("transaction_limit",  "Transaction Limits",     "Per-role / per-product transaction caps", "money", 1, "limit"),
    ("authorization_limit","Authorization Limits",   "Auto-auth thresholds + approver caps",    "shield", 2, "limit"),
    ("rate",               "Rates",                  "Interest rates, deposit rates",           "percent", 3, "money"),
    ("fee",                "Fees",                   "Transaction fees, monthly fees",          "tag", 4, "money"),
    ("regulatory",         "Regulatory Thresholds",  "CTR / STR / KYC thresholds",              "alert-triangle", 5, "compliance"),
    ("operational_hours",  "Operational Hours",      "Branch / module operating windows",       "clock", 6, "hours"),
]

# ─── parameters (two-limit, rates, fees, regulatory, hours) ──────────────────

# (category, name, applies_to, applies_to_id, value (dict), value_type, unit, scope, product)
PARAMETERS = [
    # ─── Two-limit pattern per role ────────────────────────────────────────
    ("transaction_limit", "teller_transfer_max",     "role", "TELLER",
        {"transaction_limit": 100_000_000, "authorization_limit": 25_000_000,
         "currency": "IDR", "scope": "per_transaction"},
        "json", "IDR", "per_transaction", "transfer"),

    ("authorization_limit", "teller_authorization_limit", "role", "TELLER",
        {"authorization_limit": 25_000_000, "currency": "IDR", "scope": "per_transaction"},
        "json", "IDR", "per_transaction", "transfer"),

    ("authorization_limit", "supervisor_auth_max", "role", "SUPERVISOR",
        {"authorization_limit": 100_000_000, "currency": "IDR", "scope": "per_transaction"},
        "json", "IDR", "per_transaction", "transfer"),

    ("authorization_limit", "branch_manager_auth_max", "role", "BRANCH_MANAGER",
        {"authorization_limit": 500_000_000, "currency": "IDR", "scope": "per_transaction"},
        "json", "IDR", "per_transaction", "transfer"),

    # ─── Sample rates (per product) ─────────────────────────────────────────
    ("rate", "deposito_3m_rate", "product", "deposito",
        {"rate": 3.25, "rate_unit": "percent_per_year", "tenor_months": 3, "calculation_method": "simple_interest"},
        "json", "percent", None, "deposito_3m"),

    ("rate", "deposito_12m_rate", "product", "deposito",
        {"rate": 4.50, "rate_unit": "percent_per_year", "tenor_months": 12, "calculation_method": "simple_interest"},
        "json", "percent", None, "deposito_12m"),

    ("rate", "tabungan_rate", "product", "tabungan",
        {"rate": 1.00, "rate_unit": "percent_per_year", "calculation_method": "daily_average"},
        "json", "percent", None, "tabungan"),

    # ─── Sample fees ────────────────────────────────────────────────────────
    ("fee", "interbank_transfer_fee", "product", "transfer",
        {"fee": 6500, "currency": "IDR", "fee_type": "fixed"},
        "json", "IDR", "per_transaction", "interbank"),

    ("fee", "monthly_admin_fee_tabungan", "product", "tabungan",
        {"fee": 11000, "currency": "IDR", "fee_type": "fixed"},
        "json", "IDR", "per_month", "tabungan"),

    # ─── Regulatory thresholds ──────────────────────────────────────────────
    ("regulatory", "ctr_threshold", "global", None,
        {"threshold": 500_000_000, "currency": "IDR", "report_name": "Cash Transaction Report",
         "scope": "per_day"},
        "json", "IDR", "per_day", None),

    ("regulatory", "str_threshold", "global", None,
        {"threshold": 1_000_000_000, "currency": "IDR", "report_name": "Suspicious Transaction Report",
         "scope": "per_transaction"},
        "json", "IDR", "per_transaction", None),

    # ─── Operational hours (global) ─────────────────────────────────────────
    ("operational_hours", "teller_operating_hours", "global", None,
        {"weekday": {"open": "08:00", "close": "15:00"},
         "saturday": {"open": "08:00", "close": "12:00"},
         "sunday": None, "timezone": "WIB"},
        "json", "hours", None, None),
]


def write_seed_001(env: dict[str, str]) -> None:
    org_id = env.get("SEED_ORG_ID") or deterministic_uuid("organization", env.get("SEED_ORG_CODE", "BJBS"))
    out = ROOT / "seed_001_categories.sql"
    print(f"[transform] writing {out}", file=sys.stderr)

    with out.open("w") as f:
        f.write("-- seed_001_categories.sql — generated by transform.py\n\n")
        for code, name, desc, icon, order, color in CATEGORIES:
            cid = deterministic_uuid("parameter_category", code)
            f.write(
                "INSERT INTO parameter_categories (id, org_id, code, name, description, display_order, icon, color, is_active) "
                f"VALUES ({t(cid)}, {t(org_id)}, {t(code)}, {t(name)}, {t(desc)}, {order}, {t(icon)}, {t(color)}, TRUE) "
                "ON CONFLICT (org_id, code) DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description, "
                "display_order = EXCLUDED.display_order;\n"
            )


def write_seed_002(env: dict[str, str]) -> None:
    org_id = env.get("SEED_ORG_ID") or deterministic_uuid("organization", env.get("SEED_ORG_CODE", "BJBS"))
    created_by = env.get("SEED_CREATED_BY") or deterministic_uuid("user", "SYSTEM")
    out = ROOT / "seed_002_parameters.sql"
    print(f"[transform] writing {out}", file=sys.stderr)

    with out.open("w") as f:
        f.write("-- seed_002_parameters.sql — generated by transform.py\n\n")
        for cat, name, applies_to, applies_to_id, val, vtype, unit, scope, product in PARAMETERS:
            pid = deterministic_uuid("parameter", f"{cat}:{name}:{applies_to}:{applies_to_id or ''}:{product or ''}")
            f.write(
                "INSERT INTO parameters (id, org_id, category, name, applies_to, applies_to_id, product, "
                "value, value_type, unit, scope, effective_from, version, is_active, created_by) VALUES ("
                f"{t(pid)}, {t(org_id)}, {t(cat)}, {t(name)}, {t(applies_to)}, "
                f"{t(applies_to_id) if applies_to_id else 'NULL'}, {t(product) if product else 'NULL'}, "
                f"{j(val)}, {t(vtype)}, {t(unit) if unit else 'NULL'}, {t(scope) if scope else 'NULL'}, "
                f"NOW(), 1, TRUE, {t(created_by)}) "
                "ON CONFLICT (id) DO UPDATE SET value = EXCLUDED.value, unit = EXCLUDED.unit, "
                "scope = EXCLUDED.scope, version = parameters.version + 1, is_active = TRUE;\n"
            )
            # Initial history entry
            hid = deterministic_uuid("parameter_history", f"{pid}:create")
            f.write(
                "INSERT INTO parameter_history (id, parameter_id, org_id, previous_value, new_value, "
                "change_type, previous_version, new_version, change_reason, changed_by) VALUES ("
                f"{t(hid)}, {t(pid)}, {t(org_id)}, NULL, {j(val)}, 'create', NULL, 1, "
                f"'Initial demo seed', {t(created_by)}) "
                "ON CONFLICT (id) DO NOTHING;\n"
            )


if __name__ == "__main__":
    env = load_env()
    write_seed_001(env)
    write_seed_002(env)
    print("[transform] done", file=sys.stderr)
