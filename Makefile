.PHONY: setup db-up db-down migrate-up migrate-down run test

DB_URL ?= postgres://policy7:policy7secret@localhost:5436/policy7?sslmode=disable

setup:
	go mod tidy

db-up:
	docker compose up -d postgres redis

db-down:
	docker compose down

migrate-up:
	migrate -path migrations -database "$(DB_URL)" -verbose up

migrate-down:
	migrate -path migrations -database "$(DB_URL)" -verbose down

run:
	go run cmd/server/main.go

test:
	go test -v ./...
