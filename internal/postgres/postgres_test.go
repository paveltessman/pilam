package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/platform/config"
)

// scratchDB is openTestDB plus the table the transaction tests write to.
func scratchDB(t *testing.T) *DB {
	t.Helper()

	db := openTestDB(t)
	exec(t, t.Context(), db, `CREATE TABLE intx_test (note text not null)`)
	return db
}

// exec runs one statement outside a transaction.
func exec(t *testing.T, ctx context.Context, db *DB, sql string) {
	t.Helper()
	if _, err := db.conn(ctx).Exec(ctx, sql); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

// notes returns everything committed to the scratch table.
func notes(t *testing.T, db *DB) []string {
	t.Helper()

	ctx := t.Context()
	rows, err := db.conn(ctx).Query(ctx, `SELECT note FROM intx_test ORDER BY note`)
	if err != nil {
		t.Fatalf("reading notes: %v", err)
	}
	defer rows.Close()

	var found []string
	for rows.Next() {
		var note string
		if err := rows.Scan(&note); err != nil {
			t.Fatalf("reading notes: %v", err)
		}
		found = append(found, note)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading notes: %v", err)
	}
	return found
}

func TestOpenRejectsAnUnreachableServer(t *testing.T) {
	t.Parallel()

	// pgxpool keeps retrying a refused connection until the context runs out,
	// so a bad address costs Open its whole connect timeout. Deadline of the
	// caller's own context is the shorter of the two; the test uses that rather
	// than sitting out the boot-time one.
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	_, err := Open(ctx, config.Database{URL: "postgres://pilam:pilam@127.0.0.1:1/pilam"})
	if err == nil {
		t.Fatal("Open succeeded against a closed port, want an error")
	}
}

func TestPing(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	if err := db.Ping(t.Context()); err != nil {
		t.Errorf("Ping: %v", err)
	}
}

func TestInTxCommitsWhenTheFunctionReturnsNil(t *testing.T) {
	t.Parallel()

	db := scratchDB(t)

	err := db.InTx(t.Context(), func(ctx context.Context) error {
		_, err := db.conn(ctx).Exec(ctx, `INSERT INTO intx_test (note) VALUES ('kept')`)
		return err
	})
	if err != nil {
		t.Fatalf("InTx: %v", err)
	}

	if got := notes(t, db); len(got) != 1 || got[0] != "kept" {
		t.Errorf("rows = %v, want [kept]", got)
	}
}

// The audit trail depends on this: a write and the entry describing it either
// both land or neither does.
func TestInTxRollsBackWhenTheFunctionFails(t *testing.T) {
	t.Parallel()

	db := scratchDB(t)

	sentinel := errors.New("the second write failed")
	err := db.InTx(t.Context(), func(ctx context.Context) error {
		if _, err := db.conn(ctx).Exec(ctx, `INSERT INTO intx_test (note) VALUES ('discarded')`); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("InTx error = %v, want %v", err, sentinel)
	}

	if got := notes(t, db); len(got) != 0 {
		t.Errorf("rows = %v, want none: the transaction should have rolled back", got)
	}
}

func TestInTxRollsBackWhenTheFunctionPanics(t *testing.T) {
	t.Parallel()

	db := scratchDB(t)

	func() {
		defer func() {
			if recover() == nil {
				t.Error("the panic did not propagate out of InTx")
			}
		}()
		_ = db.InTx(t.Context(), func(ctx context.Context) error {
			if _, err := db.conn(ctx).Exec(ctx, `INSERT INTO intx_test (note) VALUES ('discarded')`); err != nil {
				return err
			}
			panic("something went wrong mid-use-case")
		})
	}()

	if got := notes(t, db); len(got) != 0 {
		t.Errorf("rows = %v, want none: a panic must not commit", got)
	}
}

// A cancelled request is the ordinary way a transaction is abandoned, and the
// rollback has to happen anyway. Without it the write would sit on the
// connection until the server noticed the client had gone.
func TestInTxRollsBackWhenTheContextIsCancelled(t *testing.T) {
	t.Parallel()

	db := scratchDB(t)

	outer, cancel := context.WithCancel(t.Context())
	err := db.InTx(outer, func(ctx context.Context) error {
		if _, err := db.conn(ctx).Exec(ctx, `INSERT INTO intx_test (note) VALUES ('discarded')`); err != nil {
			return err
		}
		cancel()
		return ctx.Err()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("InTx error = %v, want %v", err, context.Canceled)
	}

	if got := notes(t, db); len(got) != 0 {
		t.Errorf("rows = %v, want none", got)
	}
}

// A nested call joins the transaction already in the context. Without that, an
// audit entry written inside a user write would sit in a second transaction and
// could land on its own.
func TestInTxJoinsTheTransactionTheContextCarries(t *testing.T) {
	t.Parallel()

	db := scratchDB(t)

	sentinel := errors.New("the outer work failed")
	err := db.InTx(t.Context(), func(ctx context.Context) error {
		inner := db.InTx(ctx, func(ctx context.Context) error {
			_, err := db.conn(ctx).Exec(ctx, `INSERT INTO intx_test (note) VALUES ('discarded')`)
			return err
		})
		if inner != nil {
			return inner
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("InTx error = %v, want %v", err, sentinel)
	}

	// The inner call returned nil, but it did not commit: the outer rollback
	// took its write with it.
	if got := notes(t, db); len(got) != 0 {
		t.Errorf("rows = %v, want none: the inner call committed on its own", got)
	}
}
