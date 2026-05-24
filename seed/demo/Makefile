# Makefile for policy7 demo seed.
# Run from this directory: `make seed-demo`
#
# DATABASE_URL must point at the policy7 Postgres target.

.PHONY: help seed-demo seed-demo-transform seed-demo-apply seed-clean

help:
	@echo "Targets:"
	@echo "  seed-demo-transform  Generate seed_*.sql (no Oracle access needed)"
	@echo "  seed-demo-apply      psql -f seed_*.sql against \$$DATABASE_URL"
	@echo "  seed-demo            transform + apply"
	@echo "  seed-clean           TRUNCATE seeded tables in reverse FK order"

seed-demo-transform:
	python3 transform.py

seed-demo-apply:
	@for f in seed_001_categories.sql seed_002_parameters.sql; do \
		echo "[seed] applying $$f"; \
		psql "$$DATABASE_URL" -v ON_ERROR_STOP=1 -f $$f ; \
	done

seed-demo: seed-demo-transform seed-demo-apply

seed-clean:
	psql "$$DATABASE_URL" -c "TRUNCATE parameter_history, parameters, parameter_categories RESTART IDENTITY CASCADE;"
