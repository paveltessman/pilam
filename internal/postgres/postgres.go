// Package postgres is the only package in the application that knows SQL.
//
// It owns the connection pool, the transaction runner, and every repository
// implementation, one file per domain.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/paveltessman/pilam/internal/platform/config"
)

const connectTimeout = 5 * time.Second

// DB is the application's handle on Postgres.
//
// Safe for concurrent use: the pool it wraps is.
type DB struct {
	pool *pgxpool.Pool
}

// Open connects to the database named in cfg and verifies that it answers.
//
// pgxpool connects lazily, which would defer a wrong password or an unreachable
// host to the first request. Open pays for a round trip at boot
// instead, so that a misconfiguration is a failure on startup.
func Open(ctx context.Context, cfg config.Database) (*DB, error) {
	pool, err := pgxpool.New(ctx, cfg.URL)
	if err != nil {
		// The URL's shape is validated in config, so reaching here means
		// something config does not check, such as an unknown parameter.
		return nil, fmt.Errorf("postgres: can't open a db connection pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: can't connect to db: %w", err)
	}

	return &DB{pool: pool}, nil
}

func (db *DB) Close() { db.pool.Close() }

func (db *DB) Ping(ctx context.Context) error {
	if err := db.pool.Ping(ctx); err != nil {
		return fmt.Errorf("postgres: ping: %w", err)
	}
	return nil
}
