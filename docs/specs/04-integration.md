# 04 — Integration

policy7 adalah produsen parameter; service lain mengkonsumsinya. Semua URL peer lewat
env var (lihat [05-security](05-security.md)).

## auth7 — ABAC input (reference Grup-2 consumer)

auth7 mengkonsumsi parameter policy7 sebagai **input ABAC** (rule allow/deny tetap di auth7).
Sejak auth7 #161 (`dd7b5fb`), time-based ABAC mengambil kategori `operational_hours` lewat
**inquiry generik** — `GET /v1/params/operational_hours/{name}/effective` (Grup 2 kanonik,
resolusi `user→role→branch→global` di sisi policy7) — bukan endpoint hardcoded
`/v1/params/operational-hours`. Pola konsumsinya = **cache lokal (opacache) fetch-through saat
miss + invalidasi via NATS** `policy7.params.*` (Grup 4). auth7 sengaja **tidak** memakai
`pkg/client` SDK (menulis thin fetcher sendiri). Ini contoh acuan pemakaian Grup 2 + Grup 4
([06-api-grouping](06-api-grouping.md)).

policy7 juga memvalidasi token JWT auth7 via JWKS (`TOKEN_JWKS`).

## core7-enterprise — validasi + admin UI

- Backend enterprise query policy7 untuk validasi transaksi (two-limit) dan rates/fees.
- **bos7-enterprise** = admin UI Policy Management (schema-driven, data-driven category via
  `value_schema`, two-limit, versioning, simulator). BFF-nya memanggil `/admin/v1/*`.
- **Token-exchange (RFC 8693):** BFF menukar token user menjadi token policy7 yang
  ter-delegasi sebelum memanggil read endpoint (`List`, `GetByID`); raw user token ditolak
  `RequireDelegatedOrM2M` (403). Pola sama dengan audittrail.

## workflow7 — mutasi via approval

Mutasi parameter **tidak** langsung; semua lewat approval:

```
bos7-enterprise BFF → workflow7 (flow policy-param-create|update|delete-v1)
                    → [approval] → policy7 /admin/v1/params/wf-*  (M2M + audit signature)
                    → versioning + parameter_history(change_reason) → audit7
```

workflow7 mengirim `X-Actor-OrgID` (bukan `X-Org-ID`) — handler `wf-*` membaca itu lewat
`getActorOrgID`. Prasyarat env:
- workflow7: `POLICY7_CORE_BASE_URL=http://localhost:8085`.
- policy7: `TOKEN_JWKS=http://localhost:8083/.well-known/jwks.json`,
  `WORKFLOW7_AUDIT_SIGNING_KEY` (match workflow7).

## notif7 — regulatory alerts

notif7 subscribe event NATS untuk alert ambang regulator (CTR/STR).

## audit7 — system of record

Setiap mutasi di-forward ke audit7 (`internal/api/audit_forward.go`, `audit7client`).
`AUDIT7_URL` `nats://…` mem-publish durable ke JetStream ingest
(`audit7.ingest.policy7`); kosong/placeholder → no-op aman. Lihat memory audit7 event
integration.

## branch_scope poller

`internal/service/branchscope/poller.go` mensinkron tabel `branch_scope` (id, branch_type)
dari enterprise (`ENTERPRISE_URL`). Dipakai resolution Option C tier `branch_type`.

## NATS events

`policy7.params.created|updated|deleted` di-publish saat mutasi; semua instance policy7
subscribe `policy7.params.>` untuk invalidasi cache. `policy7.health` request-reply.

## Go client SDK

```go
import "github.com/ihsansolusi/policy7/pkg/client"

c := client.NewClient(baseURL, apiKey, serviceID)
res, _ := c.ValidateTransaction(ctx, req)
```
