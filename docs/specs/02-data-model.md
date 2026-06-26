# 02 — Data Model

policy7 memiliki datanya sendiri (tidak ada skema legacy `ibpolicy`). Empat tabel:

| Tabel | Peran |
|---|---|
| `parameters` | master parameter, versioned, JSONB value |
| `parameter_history` | audit trail tiap perubahan (before/after + change_reason) |
| `parameter_categories` | metadata kategori + `value_schema` (JSON Schema + `x-ui`/`x-rules`) |
| `branch_scope` | proyeksi branch dari enterprise (id, branch_type) untuk Option C |

## `parameters`

Kolom utama (UUID PK, semua difilter `org_id` UUID):

| Kolom | Catatan |
|---|---|
| `category`, `name` | identitas parameter |
| `applies_to` | `global` / `role` / `branch` / `branch_type` / `customer_type` / `product` |
| `applies_to_id` | id target scope; **NULL hanya bila** `applies_to='global'` |
| `product` | dimensi orthogonal sekunder (mis. role=teller **dan** product=transfer) |
| `value` | JSONB (number / object / array) — bentuknya divalidasi `value_schema` |
| `unit`, `scope` | mis. `IDR`, `per_transaction` |
| `effective_from` / `effective_until` | jadwal aktif (DEFAULT NOW / nullable) |
| `version`, `is_active` | versioning; satu versi aktif per kombinasi scope |
| audit cols | `created_by/at`, `updated_by/at` |

**Constraint (enforced di DB):**

- `chk_parameters_effective_range` — `effective_until` > `effective_from`.
- `chk_parameters_scope_id` — `applies_to_id` wajib NULL ⟺ `applies_to='global'`.
- `chk_parameters_product_scope` — jika `applies_to='product'`, kolom `product` harus NULL
  (kode produk hidup di `applies_to_id`; anti-redundansi).
- **Functional unique index** — satu versi aktif per
  `(org_id, category, name, applies_to, COALESCE(applies_to_id,''), COALESCE(product,''))`
  `WHERE is_active` (`CompositeUnique` dengan `coalesce:true`).

## Versioning

Update = record baru `version++`, versi lama `is_active=false`; satu baris ditulis ke
`parameter_history` (`previous_value`, `new_value`, `change_metadata`, `change_reason`,
`changed_by`). Query default mengambil versi aktif terbaru; history tersedia via
`GET /admin/v1/params/:id/history`.

## Two-limit pattern

`transaction_limit` membawa **dua** batas dalam satu value:

```json
{ "transaction_limit": 100000000, "authorization_limit": 25000000, "currency": "IDR", "scope": "per_transaction" }
```

Decision flow (`POST /v1/params/transaction_limit/validate`):

```
amount ≤ authorization_limit      → AUTO_AUTHORIZED
authorization_limit < amount ≤ transaction_limit → REQUIRES_AUTHORIZATION
amount > transaction_limit        → REJECTED
```

## `value_schema` (data-driven categories)

`parameter_categories.value_schema` menyimpan JSON Schema bentuk `value` plus ekstensi:

- **`x-ui`** — petunjuk render (currency precision, two-limit grouping, select/lookup,
  detail-rows). **Diabaikan** validator backend (`internal/domain/value_schema.go`); hanya
  dipakai FE renderer.
- **`x-rules`** — aturan cross-field (`lte/gte/lt/gt/eq/required-if`) yang **divalidasi
  backend** (map ke HTTP 422). Validasi juga dijalankan pre-submit di FE agar value invalid
  tidak memicu workflow.

Ini yang membuat admin bisa menambah kategori & bentuk value baru tanpa ubah kode.

## DEF → migration workflow

> **Otoritatif di devroot, bukan di submodule ini.** DEF model =
> `appdefs/policy7/src/defappconfig/data_model.def` (+ `dm_common.def`). Migration di
> `migrations/` adalah hasil generate dari DEF, bukan tulis tangan.

```bash
cd ../../appdefs/policy7 && make migrate-gen-reset   # regenerate seluruh set
```

Generator: `appdefs/scripts/gen_migrations.py` (devroot). Fitur yang dipakai policy7:
`UuidF/JsonbF/DateF/IntF/BoolF`, `CheckConstraint`, `CompositeUnique{coalesce,where,name}`,
`Index({...})`. Migration lama pra-rebaseline disimpan di `migrations/.archive/`.

**Catatan:** policy7 dulu *deployed-first* (migration tulis tangan dijadikan acuan), kini
sudah di-rebaseline ke set DEF-generated (4 tabel, `20260615*`) dan sudah di-`migrate up`
ke DB nyata. Stub `defappconfig/data_model.def` di submodule sudah dihapus (canonical di
appdefs).

## Seed

`migrations-seed/<profile>/` (golang-migrate) — `make seed-up SEED_PROFILE=demo|prod`.
Tracking table per-profile (`seed_<profile>_migrations`) supaya demo/prod tak bentrok.

- `prod/` — 7 kategori dasar (metadata saja; nilai parameter dikelola via Admin API per org).
- `demo/` — kategori + `value_schema` (000003) + sample params two-limit & inheritance
  Level 1–4 (000002 + 000004).

Seed di-tulis tangan (statik, hand-curated) — policy7 tidak punya sumber eksternal untuk
di-transform, jadi tidak ada generator script (beda dengan auth7/enterprise yang men-transform
`ibankdb_medium`).
