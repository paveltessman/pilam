package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/platform/config"
)

// The transaction runner cannot be checked without a server: commit and rollback
// are the server's behaviour, not ours.
// Tests that need one read TEST_DATABASE_URL and skip when it is unset,
// so `go test ./...` stays green on a machine with no stack running.
const urlEnv = "TEST_DATABASE_URL"

// openTestDB connects, and gives the test a scratch table to write to.
func openTestDB(t *testing.T) *DB {
	t.Helper()

	url := os.Getenv(urlEnv)
	if url == "" {
		t.Skipf("%s is unset: skipping the tests that need a database", urlEnv)
	}

	ctx := t.Context()
	db, err := Open(ctx, config.Database{URL: url})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(db.Close)

	exec(t, ctx, db, `DROP TABLE IF EXISTS intx_test`)
	exec(t, ctx, db, `CREATE TABLE intx_test (note text not null)`)

	// The test's own context is cancelled before its cleanups run, so the one
	// statement that has to outlive the test gets a context of its own.
	t.Cleanup(func() { exec(t, context.WithoutCancel(ctx), db, `DROP TABLE IF EXISTS intx_test`) })

	return db
}

// exec runs one statement.
func exec(t *testing.T, ctx context.Context, db *DB, sql string) {
	t.Helper()
	err := db.InTx(ctx, func(tx dbtx) error {
		_, err := tx.Exec(ctx, sql)
		return err
	})
	if err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

// notes returns everything committed to the scratch table.
func notes(t *testing.T, db *DB) []string {
	t.Helper()

	var found []string
	err := db.InTx(t.Context(), func(tx dbtx) error {
		rows, err := tx.Query(t.Context(), `SELECT note FROM intx_test ORDER BY note`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var note string
			if err := rows.Scan(&note); err != nil {
				return err
			}
			found = append(found, note)
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatalf("reading notes: %v", err)
	}
	return found
}

func TestOpenRejectsAnUnreachableServer(t *testing.T) {
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
	db := openTestDB(t)
	if err := db.Ping(t.Context()); err != nil {
		t.Errorf("Ping: %v", err)
	}
}

func TestInTxCommitsWhenTheFunctionReturnsNil(t *testing.T) {
	db := openTestDB(t)

	err := db.InTx(t.Context(), func(tx dbtx) error {
		_, err := tx.Exec(t.Context(), `INSERT INTO intx_test (note) VALUES ('kept')`)
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
	db := openTestDB(t)

	sentinel := errors.New("the second write failed")
	err := db.InTx(t.Context(), func(tx dbtx) error {
		if _, err := tx.Exec(t.Context(), `INSERT INTO intx_test (note) VALUES ('discarded')`); err != nil {
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
	db := openTestDB(t)

	func() {
		defer func() {
			if recover() == nil {
				t.Error("the panic did not propagate out of InTx")
			}
		}()
		_ = db.InTx(t.Context(), func(tx dbtx) error {
			if _, err := tx.Exec(t.Context(), `INSERT INTO intx_test (note) VALUES ('discarded')`); err != nil {
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
	db := openTestDB(t)

	ctx, cancel := context.WithCancel(t.Context())
	err := db.InTx(ctx, func(tx dbtx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO intx_test (note) VALUES ('discarded')`); err != nil {
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
