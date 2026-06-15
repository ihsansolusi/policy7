.PHONY: setup db-up db-down migrate-gen-reset migrate-gen-add migrate-up migrate-down seed-up seed-down run test

# ─── Variabel ──────────────────────────────────────────────────────────────────
DEF_FILE     := ../../appdefs/policy7/src/defappconfig/data_model.def
MIGRATIONS   := migrations
SEED_DIR     := migrations-seed
GEN_SCRIPT   := ../../scripts/deflang/gen_migrations.py
DB_URL       ?= postgres://policy7:policy7secret@localhost:5432/policy7?sslmode=disable
SEED_PROFILE ?= demo

setup:
	go mod tidy

db-up:
	docker compose up -d postgres redis

db-down:
	docker compose down

# ─── Generate: Reset dari awal (dev / pre-production) ─────────────────────────
# Hapus semua migration di migrations/ lalu regenerate dari DEF.
# Gunakan HANYA di development / pre-production (bukan server yang sudah running).
migrate-gen-reset:
	@echo ""
	@echo "PERINGATAN: Semua file di $(MIGRATIONS)/*.sql akan dihapus dan dibuat ulang dari DEF."
	@echo "Gunakan HANYA di development / pre-production."
	@echo ""
	@printf "Lanjutkan? [y/N] " && read ans && [ "$$ans" = "y" ] || (echo "Dibatalkan."; exit 0)
	rm -f $(MIGRATIONS)/*.sql
	python3 $(GEN_SCRIPT) \
		--def $(DEF_FILE) \
		--out $(MIGRATIONS) \
		--module policy7 \
		--date $$(date +%Y%m%d) \
		--start 1
	@echo ""
	@echo "Selesai. Jalankan: make migrate-up"

# ─── Generate: Tambah migration baru (post-production incremental) ─────────────
migrate-gen-add:
	@[ -n "$(NAME)" ] || \
		(echo "Error: NAME wajib diisi."; \
		 echo "Usage: make migrate-gen-add NAME=add_column_to_parameters"; exit 1)
	@DATE=$$(date +%Y%m%d); \
	LAST=$$(ls $(MIGRATIONS)/$${DATE}*.up.sql 2>/dev/null \
		| sed "s|$(MIGRATIONS)/$${DATE}||;s|_.*||" | sort -n | tail -1); \
	if [ -z "$$LAST" ]; then SEQ=1; else SEQ=$$(( 10#$$LAST + 1 )); fi; \
	FILE="$(MIGRATIONS)/$${DATE}$$(printf '%06d' $$SEQ)_$(NAME)"; \
	printf -- "-- Migration: $(NAME)\n\n" > "$${FILE}.up.sql"; \
	printf -- "-- Rollback: $(NAME)\n\n" > "$${FILE}.down.sql"; \
	echo "File dibuat: $${FILE}.up.sql / .down.sql"

# ─── Jalankan Migration: SCHEMA (migrations/) ─────────────────────────────────
migrate-up:
	migrate -path $(MIGRATIONS) -database "$(DB_URL)" -verbose up

migrate-down:
	migrate -path $(MIGRATIONS) -database "$(DB_URL)" -verbose down

# ─── Jalankan Migration: SEED (migrations-seed/<profile>/) ────────────────────
# Sama seperti schema migration tapi folder berbeda + tracking table terpisah
# per-profile (seed_<profile>_migrations) supaya demo/prod tidak bertabrakan.
# Override profile: make seed-up SEED_PROFILE=prod
seed-up:
	migrate -path $(SEED_DIR)/$(SEED_PROFILE) \
		-database "$(DB_URL)&x-migrations-table=seed_$(SEED_PROFILE)_migrations" -verbose up

seed-down:
	migrate -path $(SEED_DIR)/$(SEED_PROFILE) \
		-database "$(DB_URL)&x-migrations-table=seed_$(SEED_PROFILE)_migrations" -verbose down

run:
	go run cmd/server/main.go

test:
	go test -v ./...
