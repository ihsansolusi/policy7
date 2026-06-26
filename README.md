# Policy7

**Centralized business-policy & parameter service** untuk ekosistem Core7.

Policy7 menyimpan seluruh **parameter bisnis** yang bisa berubah tanpa redeploy aplikasi —
transaction limit, approval threshold, jam operasional, bunga & biaya, ambang regulator
(CTR/STR), dan aturan akses produk. Semua service lain (auth7, enterprise, workflow7,
notif7) mengkonsumsinya lewat REST sehingga parameter tidak lagi tersebar & terduplikasi
di tiap service.

> **Batas tegas:** policy7 menjawab *"berapa batasnya?"* / *"berapa nilainya?"*.
> Keputusan boolean *"boleh atau tidak?"* tetap milik **auth7** (ABAC/Rego),
> orkestrasi *"siapa yang approve?"* tetap milik **workflow7**.

---

## Status

Service sudah **terimplementasi & terintegrasi** (runtime API, admin CRUD, versioning,
NATS events, workflow7 approval callbacks, audit7 forwarding). Detail apa yang sudah jalan
vs yang masih backlog ada di [`docs/ROADMAP.md`](docs/ROADMAP.md).

## Yang Dikelola

| Kategori | Contoh |
|---|---|
| Transaction limit (employee & customer) | Teller max Rp 100jt/transaksi |
| Authorization / approval threshold | Auto-auth ≤ Rp 25jt, di atasnya butuh approval |
| Operational hours | Teller 08:00–16:00 WIB |
| Interest rates & fees | Deposito 12 bln 4.5% p.a.; transfer Rp 6.500 |
| Regulatory thresholds | Lapor CTR jika transaksi > Rp 100jt |
| Product access rules | Role mana boleh akses produk mana |

## Arsitektur Singkat

Clean architecture Go (`api → service → store → domain`) di atas **hybrid data store**:
PostgreSQL 16 (pgx + sqlc) sebagai master, Redis untuk hot-cache, NATS untuk event &
cache-invalidation. Lihat [`docs/specs/01-architecture.md`](docs/specs/01-architecture.md).

```
cmd/server       → entrypoint (Gin)
internal/api     → REST handlers + middleware (auth, M2M, audit-signature)
internal/service → business logic (resolution, versioning, NATS, branch-scope poller)
internal/store   → PostgreSQL (sqlc) + Redis
internal/domain  → entities, value_schema validator, errors
(no SDK)         → konsumen panggil REST /v1 langsung (thin client; lihat 04-integration)
```

## Quick Start

Prasyarat: Go 1.22+, PostgreSQL 16, Redis 7, NATS (opsional di dev).

```bash
cp .env.example .env          # set DATABASE_URL, REDIS_URL, PORT, dst.
make setup                    # go mod tidy
make db-up                    # docker compose: postgres + redis
make migrate-up               # apply schema (migrations/)
make seed-up                  # seed demo (migrations-seed/demo) — opsional
make run                      # jalan di :8085
make test
```

> **Migration files di-generate dari DEF**, bukan ditulis tangan. Sumbernya
> `appdefs/policy7/src/defappconfig/data_model.def` di devroot. Regenerate:
> `cd ../../appdefs/policy7 && make migrate-gen-reset`. Lihat
> [`docs/specs/02-data-model.md`](docs/specs/02-data-model.md).

## API

- **`/v1/*`** — consumer inquiry (generik): `…/effective` (resolve satu), `resolve` (batch),
  `?category=` (snapshot), `transaction_limit/validate` (decision helper). Butuh delegated JWT
  atau M2M.
- **`/admin/v1/*`** — admin reads parameter & kategori + history + bulk-import. Dipakai
  Policy Management UI di bos7-enterprise.
- **`/admin/v1/.../wf-*`** — callback approval dari workflow7 (M2M + audit signature); satu-satunya
  jalur mutasi.

Daftar lengkap: [`docs/specs/03-api.md`](docs/specs/03-api.md).

## Integrasi

| Service | Pola | Use case |
|---|---|---|
| **auth7** | REST (ABAC input) | operational hours, product access untuk Rego |
| **core7-enterprise** | REST + BFF (token-exchange) | validasi two-limit, rates & fees, admin UI |
| **workflow7** | REST callback (M2M) | mutasi parameter via approval (`policy-param-*-v1`) |
| **notif7** | NATS | alert ambang regulator |
| **audit7** | NATS ingest | system of record untuk semua mutasi |

Detail: [`docs/specs/04-integration.md`](docs/specs/04-integration.md),
keamanan: [`docs/specs/05-security.md`](docs/specs/05-security.md).

## Dokumentasi

- [`docs/specs/`](docs/specs/) — spesifikasi teknis (overview, arsitektur, data model, API
  as-built, integrasi, security, **kontrak API target**). Index:
  [`docs/specs/README.md`](docs/specs/README.md).
- [`docs/ROADMAP.md`](docs/ROADMAP.md) — yang sudah diimplementasi vs yang masih backlog.

> Panduan kerja AI/Claude memakai `CLAUDE.md` di root devroot, bukan di submodule ini.
