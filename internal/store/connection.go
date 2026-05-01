package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewDBPool initializes a new PostgreSQL connection pool
func NewDBPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	const op = "store.NewDBPool"

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to parse config: %w", op, err)
	}

	// Register jsonb with standard library json marshaler so that pgx correctly
	// encodes/decodes jsonb columns as plain JSON bytes (not binary bytea).
	// Without this, pgx binary protocol sends jsonb with a version-byte prefix
	// that causes json.RawMessage fields to be base64-encoded in JSON responses.
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		conn.TypeMap().RegisterType(&pgtype.Type{
			Name:  "jsonb",
			OID:   pgtype.JSONBOID,
			Codec: &pgtype.JSONBCodec{Marshal: json.Marshal, Unmarshal: json.Unmarshal},
		})
		return nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to create pool: %w", op, err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("%s: failed to ping database: %w", op, err)
	}

	return pool, nil
}
