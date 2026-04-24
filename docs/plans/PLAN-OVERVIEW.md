# Policy7 — Plan Overview

> **Status**: Brainstorming → Planning
> **Target**: v1.0 Foundation

---

## Plan 01 — Foundation
- [ ] Scaffold Go project (cmd/, internal/, domain/)
- [ ] PostgreSQL schema (parameters, parameter_history)
- [ ] Basic REST API (GET /v1/params/*)
- [ ] Docker compose (postgres + redis)
- [ ] Unit tests

## Plan 02 — Admin API
- [ ] CRUD parameters (/admin/v1/params)
- [ ] Versioning logic
- [ ] Audit trail (parameter_history)
- [ ] Effective date scheduling

## Plan 03 — Parameter Categories
- [ ] Transaction limits (employee & customer)
- [ ] Approval thresholds
- [ ] Operational hours
- [ ] Product access rules

## Plan 04 — Rates & Fees
- [ ] Interest rates per product/tenor
- [ ] Fee & tarif per product/channel
- [ ] Regulatory thresholds (CTR/STR)

## Plan 05 — Integration
- [ ] Auth7 OPA integration (OPA query policy7)
- [ ] Core7 enterprise integration
- [ ] Workflow7 integration
- [ ] notif7 integration (regulatory alerts)

## Plan 06 — Performance & Caching
- [ ] Redis hot cache
- [ ] Parameter change pub/sub
- [ ] Load testing

---

*Diperbarui: 2026-04-24*