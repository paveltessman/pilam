package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/paveltessman/pilam/internal/platform/logging"
	"github.com/paveltessman/pilam/internal/postgres/internal/sqlc"
)

const rollbackTimeout = 5 * time.Second

type txKey struct{}

// InTx runs fn inside a transaction, committing when it returns nil and rolling
// back when it returns an error or panics.
//
// The transaction travels in the context fn receives, so every repository call
// under fn joins it without carrying a handle. A caller that is already inside
// a transaction joins that one, and the outermost InTx alone commits it.
//
// fn must not use the transaction from more than one goroutine: pgx.Tx is not
// safe for concurrent use.
func (db *DB) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := txFromContext(ctx); ok {
		return fn(ctx)
	}

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

	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: commit: %w", err)
	}
	return nil
}

// txFromContext returns the transaction ctx carries, and whether it carries one.
func txFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	return tx, ok
}

// conn returns what the next statement runs against: the transaction ctx
// carries, or the pool when the call stands on its own. DBTX is the generated
// interface over exactly that pair.
func (db *DB) conn(ctx context.Context) sqlc.DBTX {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}
	return db.pool
}

// queries returns the generated queries, bound to the connection ctx names.
func (db *DB) queries(ctx context.Context) *sqlc.Queries { return sqlc.New(db.conn(ctx)) }
