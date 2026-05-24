# Policy7 demo seed

Static seed of the **two-limit pattern** for the demo tenant (BJBS):
teller / supervisor / branch_manager transaction limits + authorization limits,
plus sample rates, fees, regulatory thresholds, and operational hours.

No Oracle extract is required — the ibent `PARAMETERGLOBAL` table doesn't map
cleanly to policy7's role-scoped JSONB structure, so the seed is hand-curated.

## Cross-service ID consistency

`policy7.parameters.applies_to_id` is `VARCHAR(100)` — it stores the **business
code** of the target, not a UUID:

| applies_to | applies_to_id | Origin |
|---|---|---|
| `role` | role code (e.g. `TELLER`, `SUPERVISOR`) | auth7.roles.code |
| `branch` | branch code (e.g. `010`) | auth7.branches.code / enterprise.branches.branch_code |
| `user` | user UUID as string | auth7.users.id |
| `product` | product code (e.g. `transfer`, `deposito_12m`) | (free-form) |
| `customer_type` | type code (e.g. `vip`, `retail`) | (free-form) |
| `global` | NULL | — |

`policy7.parameters.org_id` and `parameters.created_by` are UUIDs — keep
consistent with auth7 (`SEED_ORG_ID`, deterministic_uuid('user','SYSTEM')).

## Alur

```bash
cp .env.seed.example .env.seed
make seed-demo-transform    # generate seed_*.sql
make seed-demo-apply        # psql -f seed_*.sql
# atau
make seed-demo
```

## File yang dihasilkan

```
seed/demo/
├── seed_001_categories.sql   # parameter_categories
└── seed_002_parameters.sql   # parameters (two-limit, rates, fees, regulatory, hours)
```

## Catatan two-limit pattern

```
Authorization Limit (auto-auth threshold)    Transaction Limit (max input)
        teller_authorization_limit                  teller_transfer_max
              Rp 25jt                                  Rp 100jt

Amount ≤ auth limit         → AUTO_AUTHORIZED
Auth limit < Amount ≤ trans → REQUIRES_AUTHORIZATION (supervisor / BM auth)
Amount > trans limit        → REJECTED

Supervisor max auth:  supervisor_auth_max      = Rp 100jt
BM max auth:          branch_manager_auth_max  = Rp 500jt
```

Lihat `docs/specs/02-api-detail.md` + `02-api-detail-samples.md` di repo
policy7 untuk semantik lengkap.
