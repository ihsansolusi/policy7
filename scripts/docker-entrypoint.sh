#!/bin/sh
# docker-entrypoint.sh — Ensure DB + user, run migrations, start policy7

set -e

if [ -z "${DATABASE_URL##*\?*}" ]; then
  DATABASE_URL="${DATABASE_URL}&sslmode=disable"
else
  DATABASE_URL="${DATABASE_URL}?sslmode=disable"
fi

if [ -n "$DATABASE_ADMIN_URL" ]; then
  if [ -z "${DATABASE_ADMIN_URL##*\?*}" ]; then
    DATABASE_ADMIN_URL="${DATABASE_ADMIN_URL}&sslmode=disable"
  else
    DATABASE_ADMIN_URL="${DATABASE_ADMIN_URL}?sslmode=disable"
  fi

  echo "→ Ensuring database 'policy7' exists..."
  psql "$DATABASE_ADMIN_URL" -tc "SELECT 1 FROM pg_database WHERE datname='policy7'" \
    | grep -q 1 || psql "$DATABASE_ADMIN_URL" -c "CREATE DATABASE policy7;"

  echo "→ Ensuring user 'policy7' exists..."
  psql "$DATABASE_ADMIN_URL" -tc "SELECT 1 FROM pg_roles WHERE rolname='policy7'" \
    | grep -q 1 || psql "$DATABASE_ADMIN_URL" -c "CREATE USER policy7 WITH PASSWORD 'policy7secret';"
  psql "$DATABASE_ADMIN_URL" -c "ALTER USER policy7 WITH PASSWORD 'policy7secret';" 2>/dev/null || true
  psql "$DATABASE_ADMIN_URL" -c "GRANT ALL PRIVILEGES ON DATABASE policy7 TO policy7;" 2>/dev/null || true

  P7_DB_URL="${DATABASE_ADMIN_URL%/postgres*}/policy7?sslmode=disable"
  psql "$P7_DB_URL" -c "GRANT CREATE ON SCHEMA public TO policy7;" 2>/dev/null || true
  psql "$P7_DB_URL" -c "GRANT USAGE ON SCHEMA public TO policy7;" 2>/dev/null || true
  psql "$P7_DB_URL" -c "ALTER DATABASE policy7 SET search_path TO public;" 2>/dev/null || true
  psql "$P7_DB_URL" -c "ALTER ROLE policy7 SET search_path TO public;" 2>/dev/null || true

  DIRTY=$(psql "$P7_DB_URL" -tAc "SELECT COUNT(*) FROM schema_migrations WHERE dirty=true" 2>/dev/null || echo "0")
  if [ "$DIRTY" != "0" ] && [ -n "$DIRTY" ]; then
    echo "→ Repairing dirty migration state..."
    psql "$P7_DB_URL" -c "DELETE FROM schema_migrations WHERE dirty=true;" 2>/dev/null || true
  fi

  echo "→ Database ready."
fi

echo "→ Running migrations..."
migrate -path migrations -database "$DATABASE_URL" -verbose up
echo "→ Migrations done."

if [ -n "$DATABASE_URL" ] && [ -f scripts/seed-data.sql ]; then
  echo "→ Seeding initial data..."
  psql "${DATABASE_URL}" -f scripts/seed-data.sql 2>&1 || true
  echo "→ Seed done."
fi

exec ./policy7
