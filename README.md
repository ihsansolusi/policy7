# Policy7

Policy7 is a centralized business policy and parameter service for the Core7 ecosystem. It manages all configuration that can change without requiring application redeployment.

## Key Features

- **Multi-tenant isolation**: Strict enforcement of `X-Org-ID` for all operations.
- **Parameter Inheritance**: Evaluates parameters following a resolution fallback: `user` -> `role` -> `branch` -> `global`.
- **Two-Limit Pattern**: Differentiates between Authorization Limits (requires approval) and Transaction Limits (hard ceiling).
- **Audit Trails & Versioning**: All modifications to parameters are versioned and audited seamlessly in `parameter_history`.
- **Hybrid Data Store**: Relies on PostgreSQL 16 (accessed via strictly typed `sqlc`) and Redis for high-speed hot-caching.
- **Event Streaming**: NATS integration for async cache invalidation and telemetry broadcasting.

## Managed Configurations

- Transaction limits (employee & customer)
- Approval thresholds
- Operational hours
- Interest rates & fees
- Regulatory thresholds (CTR/STR)
- Product access rules

## Getting Started

### Prerequisites
- Go 1.22+
- PostgreSQL 16
- Redis 7
- NATS Server

### Local Development Setup

```bash
# Set up environment variables
cp configs/.env.example .env

# Download dependencies
make setup

# Spin up infrastructure (PostgreSQL & Redis)
make db-up

# Run database migrations
make migrate-up

# Start the application on port 8085
make run
```

### Running Tests

```bash
make test
```

## Architecture

Policy7 embraces Clean Architecture (domain, service, store, api layers) integrated with a Hybrid Messaging Strategy. See `docs/specs/` for architectural references and decision records.

## Integration Points

- **Auth7**: Rego policies query Policy7 for operational hours and product access dynamically.
- **Core7-Enterprise**: Validates transactions and applies correct fees directly sourced from Policy7.
- **Workflow7**: Checks Policy7 for auto-approval thresholds and routing limits.
- **Notif7**: Subscribes to regulatory flag events published via NATS.
