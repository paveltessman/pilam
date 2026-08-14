// Package postgres is the only package in the application that knows SQL.
//
// It owns the connection pool, the transaction runner, and every repository
// implementation, one file per domain.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/paveltessman/pilam/internal/platform/config"
	"github.com/paveltessman/pilam/internal/platform/logging"
)

const (
	connectTimeout  = 5 * time.Second
	rollbackTimeout = 5 * time.Second
)

// dbtx is what a query runs against: either the pool, or a transaction.
type dbtx interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

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

// InTx runs fn inside a transaction, committing when it returns nil and rolling
// back when it returns an error or panics.
//
// fn must not use the transaction from more than one goroutine: pgx.Tx is not
// safe for concurrent use.
func (db *DB) InTx(ctx context.Context, fn func(tx dbtx) error) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin: %w", err)
	}

	// Rollback on the way out covers both the error paths below and a panic
	// unwinding through fn, and is a no-op once the transaction has committed.
	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackTimeout)
		defer cancel()
		if err := tx.Rollback(rollbackCtx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			logging.FromContext(ctx).Error("rolling back transaction", "err", err)
		}
	}()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: commit: %w", err)
	}
	return nil
}
