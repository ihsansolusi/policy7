# CLAUDE.md — Policy7

Panduan konteks untuk Claude AI saat bekerja di repository `policy7`.

---

## Identitas Proyek

- **Proyek**: Policy7 — Business policy & parameter service untuk ekosistem Core7
- **Repo**: `github.com/ihsansolusi/policy7` (branch: `main`)
- **Submodule di**: `/home/galih/Works/projects/banks/core7-devroot/supported-apps/policy7`
- **GitHub Org**: `ihsansolusi`

---

## Tujuan Policy7

Policy7 adalah service terpisah yang menyimpan **semua parameter bisnis** yang bisa berubah tanpa deploy aplikasi. Ini meliputi:

- Transaction limits (employee & customer)
- Approval thresholds
- Operational hours
- Interest rates & fees
- Regulatory thresholds (CTR/STR)
- Product access rules
- Business rules/scoring thresholds

---

## Hubungan dengan Auth7 & Core7

```
auth7:      "BOLEHKAH user ini akses resource ini?" → YES/NO
policy7:    "BOLEHKAH seberapa? BERAPA batasnya?" → numeric/threshold
workflow7:  "SIAPA yang harus approve?" → approval flow
```

Auth7 menyediakan **role & permission**.
Policy7 menyediakan **limit & parameter** per role/customer/product.
Core7 services query **keduanya** untuk decision lengkap.

---

## Struktur Repo

```
policy7/
├── cmd/
│   └── server/
│       └── main.go              # Entry point
├── internal/
│   ├── api/                     # REST handlers (Gin)
│   ├── service/                 # Business logic
│   ├── store/                   # Database access (pgx + sqlc)
│   └── domain/                  # Entities, errors
├── docs/
│   ├── specs/                   # Specs (00-overview, dll)
│   └── plans/                   # Implementation plans
├── migrations/                  # golang-migrate
└── scripts/                     # DB operations
```

---

## Teknologi Stack

| Komponen | Teknologi |
|---|---|
| Language | Go 1.22+ |
| Framework | Gin (REST) |
| Database | PostgreSQL 16 (pgx + sqlc) |
| Cache | Redis (optional, untuk hot params) |
| Migrations | golang-migrate |

---

## Aturan Kode

- Setiap Go method: `const op = "package.Type.Method"`
- Error wrapping: `fmt.Errorf("%s: %w", op, err)`
- Multi-tenant: semua query wajib filter `org_id`
- No secrets in config files — hanya `"${ENV_VAR}"`

---

## Referensi

- Auth7 Specs: `../auth7/docs/specs/` (di devroot)
- Auth7 Spec 04: Authorization (referensi policy7): `../auth7/docs/specs/04-authorization.md`

---

*Diperbarui: 2026-04-24*
