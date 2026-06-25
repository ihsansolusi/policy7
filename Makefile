.PHONY: setup db-up db-down migrate-up migrate-down seed-up seed-down run test

# ─── Variabel ──────────────────────────────────────────────────────────────────
# GENERATE MIGRATION FILES (DEF → SQL): cd ../../appdefs/policy7 && make migrate-gen-reset
MIGRATIONS   := migrations
SEED_DIR     := migrations-seed
DB_URL       ?= postgres://policy7:policy7secret@localhost:5432/policy7?sslmode=disable
SEED_PROFILE ?= demo

setup:
	go mod tidy

db-up:
	docker compose up -d postgres redis

db-down:
	docker compose down

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
