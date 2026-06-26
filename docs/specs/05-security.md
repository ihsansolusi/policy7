# 05 — Security

policy7 adalah service **internal-only** — tanpa CORS / security-headers publik. Semua
proteksi pada lapis auth, M2M, dan signature.

## Multi-tenant isolation

Semua operasi di-scope `org_id` (UUID). Query store wajib memfilter `org_id`. Untuk
endpoint mutasi via workflow, org id diambil dari `X-Actor-OrgID` (`getActorOrgID`); untuk
panggilan langsung dari `X-Org-ID` / klaim token.

## Auth middleware (chain)

| Lapis | Endpoint | Mekanisme |
|---|---|---|
| `Auth(tokenMaker, serviceKeyValidator)` | `/v1`, `/admin/v1` | bearer JWT (auth7, diverifikasi via JWKS) **atau** `X-Service-Key` (BFF/M2M bypass) |
| `RequireDelegatedOrM2M()` | `/v1`, `/admin/v1` CRUD | token harus delegated (token-exchange) atau M2M; raw user token → 403 |
| `RequireM2M()` | `/admin/v1/.../wf-*` | hanya caller M2M (workflow7) |
| `VerifyAuditSignatureFromEnv()` | `/admin/v1/.../wf-*` | verifikasi signature audit (ActorEnvelope) |

`X_SERVICE_KEY_DISABLED` dapat menonaktifkan jalur service-key (mis. untuk hardening).

## Audit signature

Callback `wf-*` memverifikasi signature 7-field ActorEnvelope (lib7 ≥ v0.11.2, selaras
workflow7). `WORKFLOW7_AUDIT_SIGNING_KEY` di policy7 harus identik dengan workflow7. Versi
lib7 lama (verify 4-field) → 400 invalid signature; selalu samakan versi lib7 dengan peer.

## Audit forwarding

Setiap mutasi diteruskan ke audit7 sebagai system of record (lihat
[04-integration](04-integration.md)). HMAC/signature event mengikuti standar audit7.

## Env vars

| Var | Fungsi |
|---|---|
| `DATABASE_URL` | PostgreSQL DSN |
| `REDIS_URL` | Redis hot-cache |
| `NATS_URL` | events + cache-invalidation (kosong → no-op) |
| `PORT` | HTTP (default 8085) · `METRICS_PORT` Prometheus |
| `TOKEN_JWKS` | JWKS auth7 untuk verifikasi JWT |
| `JWT_SECRET` | token maker (jika non-JWKS) |
| `SERVICE_KEY` | nilai `X-Service-Key` yang diterima · `X_SERVICE_KEY_DISABLED` |
| `M2M_CLIENT_ID` / `M2M_CLIENT_SECRET` / `AUTH_TOKEN_ENDPOINT` | kredensial M2M outbound |
| `WORKFLOW7_AUDIT_SIGNING_KEY` | verifikasi signature callback wf-* |
| `AUDIT7_URL` | forward audit (`nats://…` durable, kosong → no-op) |
| `ENTERPRISE_URL` | sumber branch_scope poller |
| `ORG_ID` | default org untuk konteks tertentu |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | tracing |

> Tidak ada secret di config file — hanya referensi env var.
