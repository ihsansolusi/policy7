# 06 — API Grouping (target contract)

Desain target API policy7, diturunkan dari data model ([02-data-model](02-data-model.md)),
bukan dari pemakaian historis. Untuk endpoint as-built lihat [03-api](03-api.md); rencana
migrasi ada di [ROADMAP](../ROADMAP.md).

## Prinsip: kategori data-driven ⇒ inquiry generik

Admin bisa menambah kategori baru (`parameter_categories` + `value_schema`) **tanpa deploy**.
Maka API konsumsi **tidak boleh** punya satu endpoint per konsep bisnis. Endpoint lama yang
hardcoded-per-kategori (`operational-hours`, `product-access`, `approval-thresholds`,
`rates/:product`, `fees/:product`, `regulatory/:type`) secara struktural tidak bisa melayani
kategori yang belum ada saat compile — itulah kenapa nyaris tak terpakai.

> **Aturan:** konsumsi di-key oleh `(category, name)` + konteks resolusi. Semantik
> per-kategori hidup di `value_schema` / `x-rules`, bukan di routing.

> **Validasi nyata:** auth7 #161 (`dd7b5fb`) — consumer baru pertama setelah review —
> mengkonsumsi kategori `operational_hours` lewat **generic** `GET
> /v1/params/operational_hours/{name}/effective` (Grup 2) + cache fetch-through +
> NATS invalidation (Grup 4), dan **menghindari** SDK + endpoint hardcoded. Yakni
> tim lain pun, diberi pilihan, memilih pola target. Lihat
> [04-integration](04-integration.md).

---

## Lima grup

### Grup 1 — Management / Authoring (`/admin/v1`)
Pemakai: **manusia** via bos7-enterprise. Plane menulis konfigurasi.

| Sub | Endpoint | Keterangan |
|---|---|---|
| Reads | `GET /params`, `/params/:id`, `/params/:id/history`, `GET /categories`, `/categories/:code` | list/detail/history + metadata kategori |
| Bulk | `POST /params/bulk-import` | import massal (error per-row), ops-only tanpa approval |
| Mutasi (approval) | `POST /params/wf-create`, `PUT /params/:id/wf-update`, `POST /params/:id/wf-delete` (+ varian `categories/*`) | dipanggil workflow7, M2M + audit-signature |

Auth: delegated JWT (token-exchange dari BFF) atau M2M; `wf-*` wajib M2M + audit-sig.
**Dihapus:** direct CRUD non-`wf` (`POST/PUT/DELETE /params` & `/categories`, `POST /params/query`)
— sudah digantikan jalur `wf-*`.

### Grup 2 — Inquiry / Runtime (`/v1`)
Pemakai: **aplikasi** (core7 services + bos7 webapps) saat ambil keputusan. Generik.
Konteks resolusi: `org_id` (header) + `branch_id` / `role_id` / `user_id` / `product`.
policy7 menjalankan fallback (Option C `BRANCH→BRANCH_TYPE→GLOBAL`, atau
`user→role→branch→global`) dan mengembalikan value efektif + tier yang match + versi.

| Operasi | Endpoint | Status |
|---|---|---|
| Resolve satu | `GET /v1/params/{category}/{name}/effective?branch_id=&role_id=&user_id=&product=` | ✅ sudah ada — kanonik |
| Resolve banyak (batch) | `POST /v1/params/resolve` | 🆕 |
| Snapshot kategori | `GET /v1/params?category={code}&product=…` (effective only) | 🆕 |
| Decision helper | `POST /v1/params/transaction_limit/validate` (two-limit) | ⚠️ pertahankan sbg semantik eksplisit |

**`POST /v1/params/resolve`** (batch — satu decision sering butuh banyak param):
```json
// request
{ "context": { "branch_id": "…", "role_id": "teller", "user_id": "…", "product": "transfer" },
  "keys": [ {"category":"transaction_limit","name":"teller_transfer_max"},
            {"category":"fee","name":"interbank_transfer"} ] }
// response
{ "results": [
  { "category":"transaction_limit","name":"teller_transfer_max",
    "value": {…}, "matched_scope":"role", "version": 3, "effective_from":"…" },
  { "category":"fee","name":"interbank_transfer","value": null, "matched_scope": null } ] }
```

**`GET /v1/params?category=…`** — semua param efektif dalam satu kategori untuk org/scope;
dipakai cache-warm atau "ambil semua rate". Menggantikan `rates/:product`, `fees/:product`,
`product-access`, dst.

**Decision helper** = satu-satunya logika yang boleh sadar-kategori, karena membandingkan
**input runtime** (mis. nominal transaksi) dengan value tersimpan — tak bisa diekspresikan
`x-rules` (yang hanya memvalidasi bentuk value saat authoring). Dipertahankan untuk
`transaction_limit` (two-limit: `amount ≤ auth → AUTO`, `≤ trans → REQUIRES`, `> trans →
REJECTED`) karena sentralisasinya bernilai tinggi. `authorization_limit/check` &
`regulatory/check` di-fold ke pola yang sama atau didorong ke caller (resolve + bandingkan
sendiri).

### Grup 3 — Discovery / Schema
Pemakai: form dinamis bos7-enterprise + tooling/consumer yang menafsirkan value generik.

| Endpoint | Untuk |
|---|---|
| `GET /categories` + `/categories/:code` (baca `value_schema` + `x-ui`/`x-rules`) | render form dinamis; introspeksi bentuk value |

Saat ini di `/admin/v1/categories`. Bila ada consumer non-admin yang perlu bentuk value,
expose **read** value_schema juga di `/v1`. `/v1/contracts/*` lama adalah versi gagal dari
ide ini (terkopling facade) → dibuang.

### Grup 4 — Events / Subscription (NATS)
Plane **push**, bukan REST — sudah hidup & dipakai. Subjects
`policy7.params.created|updated|deleted`. Consumer (auth7, rencana workflow7) subscribe untuk
**invalidasi cache** lokal setelah param berubah → konsumsi ringan (cache + invalidate on
event, bukan polling tiap decision). Lihat [01-architecture](01-architecture.md) §NATS.

### Grup 5 — Ops
`GET /health` (liveness, tanpa auth) · `/metrics` (Prometheus, port `METRICS_PORT`).

---

## Peta transisi (as-built → target)

| As-built | Target |
|---|---|
| `GET /v1/params/:category/:name/effective` | **tetap** (Grup 2 kanonik) |
| `GET /v1/params/:category/:name` | hapus → `/effective` |
| `GET /v1/params/{operational-hours,product-access,approval-thresholds}` | hapus → `resolve` / `snapshot` (Grup 2) |
| `GET /v1/params/rates/:product`, `/fees/:product` | hapus → `snapshot(category=rate\|fee)` |
| `GET /v1/params/regulatory/:type`, `POST …/check` | hapus → `resolve` + decision caller-side |
| `POST /v1/params/authorization_limit/check` | hapus / fold ke decision helper |
| `POST /v1/params/transaction_limit/validate` | **tetap** (Grup 2 decision helper) |
| `GET /v1/contracts/*` | hapus (facade retired) |
| `POST/PUT/DELETE /admin/v1/params`, `/categories`, `POST /params/query` | hapus → `wf-*` (Grup 1) |
| `GET /admin/v1/{params,categories}` (+ `:id/history`, `bulk-import`, `wf-*`) | **tetap** (Grup 1) |
| `pkg/client` Go SDK | hapus (0 importer) atau align ke Grup 2 (`Resolve`/`BatchResolve`) bila dipakai nanti |

Hasil: dari 30+ endpoint → kontrak inti kecil — **authoring (Grup 1) + resolve/snapshot/1
decision (Grup 2) + discovery (Grup 3) + events (Grup 4) + ops (Grup 5)**.
