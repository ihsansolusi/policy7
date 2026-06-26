# 00 — Overview & Boundary

## Masalah

Core7 butuh parameter bisnis yang bisa diubah **tanpa redeploy**. Tanpa service terpusat,
parameter (limit, threshold, bunga, biaya, ambang regulator) tersebar di tiap service →
duplikasi, inkonsistensi, sulit di-audit, dan setiap perubahan butuh deploy.

## Solusi

Policy7 = *centralized business-policy & parameter service*. Satu sumber kebenaran untuk
seluruh parameter bisnis, dikonsumsi service lain via REST (real-time, versioned, auditable).

## Boundary (terkunci)

| Pertanyaan | Pemilik | Output |
|---|---|---|
| "Boleh atau tidak?" | **auth7** (ABAC/Rego) | YES / NO |
| "Berapa batasnya? / Berapa nilainya?" | **policy7** | numeric / threshold / object |
| "Siapa yang harus approve?" | **workflow7** | approval flow |

Aturan pemisahan:

- policy7 adalah **single source of truth** untuk policy/parameter pada steady-state runtime.
- policy7 **tidak** memiliki user identity, role/permission definition, atau session — itu
  domain auth7. auth7 hanya mengkonsumsi policy7 sebagai **input data ABAC** (mis. jam
  operasional, product access), bukan kepemilikan permission.
- **bos7-enterprise** adalah admin UI utama; ia consumer dari `/admin/v1/*` policy7.
- Mutasi parameter dari UI berjalan **lewat approval workflow7**, lalu workflow7 memanggil
  callback `wf-*` policy7 (lihat [04-integration](04-integration.md)).

## Scope Parameter (in scope)

| Kategori (`category`) | Contoh |
|---|---|
| `transaction_limit` | limit per-role / per-customer, two-limit (auth + transaction) |
| `approval_threshold` | ambang nominal butuh approval |
| `operational_hours` | jam kerja per-role |
| `product_access` | filter produk per-role |
| `rate` | bunga per-tenor |
| `fee` | biaya per-produk/channel |
| `regulatory` | ambang CTR/STR |

Kategori bukan hardcode — disimpan di tabel `parameter_categories` dengan `value_schema`
(JSON Schema + ekstensi `x-ui`/`x-rules`) sehingga admin bisa mendefinisikan kategori &
bentuk value baru tanpa ubah kode. Lihat [02-data-model](02-data-model.md).

## Out of scope

- ABAC boolean rules → auth7 (OPA/Rego).
- Role & permission definition, user/session lifecycle → auth7.
- Approval-flow orchestration → workflow7.
- Real-time scoring / risk engine → potensi v2.

## Stack

Go 1.22+ · Gin · PostgreSQL 16 (pgx + sqlc) · Redis · NATS · golang-migrate ·
zerolog + OpenTelemetry. Internal-only service (tanpa CORS/security-headers publik),
default port **8085**.
