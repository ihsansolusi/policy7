# Policy7 — Technical Specs

Spesifikasi teknis policy7. Mendeskripsikan service **sebagaimana yang sudah berjalan**
(bukan rencana). Untuk daftar pekerjaan yang belum diimplementasi, lihat
[`../ROADMAP.md`](../ROADMAP.md).

| # | Dokumen | Isi |
|---|---|---|
| 00 | [overview](00-overview.md) | Tujuan, boundary terhadap auth7/workflow7, scope parameter |
| 01 | [architecture](01-architecture.md) | Clean architecture, hybrid store (PostgreSQL + Redis + NATS), resolution engine |
| 02 | [data-model](02-data-model.md) | 4 entitas, scope/inheritance, two-limit, versioning, DEF→migration workflow |
| 03 | [api](03-api.md) | API as-built `/v1` + `/admin/v1` + workflow callbacks (dengan status aktif/deprecate) |
| 04 | [integration](04-integration.md) | auth7, enterprise (+BFF), workflow7 approval, notif7, audit7, Go client |
| 05 | [security](05-security.md) | Multi-tenant org scoping, delegated JWT / M2M, audit signature, env vars |
| 06 | [api-grouping](06-api-grouping.md) | **Kontrak target**: 5 grup API + inquiry generik (data-driven categories) + peta transisi |

> Migration & DEF model yang otoritatif berada di `appdefs/policy7` (devroot), bukan
> submodule ini. Spec ini mengacu ke sana di [02-data-model](02-data-model.md).
