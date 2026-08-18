package main

import (
	"context"
	"database/sql"
	"io/fs"
	"os"
	"testing"

	"github.com/paveltessman/pilam/db"
	"github.com/paveltessman/pilam/internal/platform/config"
)

const urlEnv = "TEST_DATABASE_URL"

// migrationCount reports how many versions the binary carries, which is how
// many times `down` has to run to reach an empty database.
func migrationCount(t *testing.T) int {
	t.Helper()

	fsys, err := db.Migrations()
	if err != nil {
		t.Fatalf("Migrations: %v", err)
	}
	names, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		t.Fatalf("globbing: %v", err)
	}
	return len(names)
}

// rollBack takes the database to version 0 and removes goose's own table, so
// the run leaves the server as it found it. Errors are ignored: this runs as a
// cleanup, where the test has already reported whatever went wrong.
func rollBack(ctx context.Context, cfg config.Config, count int) {
	for range count {
		_ = runMigrate(ctx, cfg, []string{"down"})
	}

	sqlDB, err := sql.Open(migrateDriver, cfg.Database.URL)
	if err != nil {
		return
	}
	defer func() { _ = sqlDB.Close() }()

	_, _ = sqlDB.ExecContext(ctx, `DROP TABLE IF EXISTS goose_db_version`)
}

func TestMigrateUpAndDown(t *testing.T) {
	url := os.Getenv(urlEnv)
	if url == "" {
		t.Fatalf("This test needs a database. Make sure db is running and %s is set", urlEnv)
	}
	cfg := config.Config{Database: config.Database{URL: url}}
	ctx := t.Context()
	count := migrationCount(t)

	// The migrations create real tables now, so a run has to start from an
	// empty database and leave an empty one. Anything left over is what the
	// next run of `up` fails on. The test's context is already cancelled by the
	// time cleanups run, hence a fresh one here.
	t.Cleanup(func() { rollBack(context.WithoutCancel(ctx), cfg, count) })
	rollBack(ctx, cfg, count)

	// Twice. The first pass says the statements are valid. The second says the
	// rollback dropped every table the first pass created.
	for pass := range 2 {
		for _, action := range []string{"up", "status"} {
			if err := runMigrate(ctx, cfg, []string{action}); err != nil {
				t.Fatalf("pass %d: migrate %s: %v", pass, action, err)
			}
		}
		for range count {
			if err := runMigrate(ctx, cfg, []string{"down"}); err != nil {
				t.Fatalf("pass %d: migrate down: %v", pass, err)
			}
		}
	}
}

func TestMigrateRejectsAnUnknownAction(t *testing.T) {
	cfg := config.Config{Database: config.Database{URL: "postgres://pilam:pilam@127.0.0.1:1/pilam"}}
	if err := runMigrate(t.Context(), cfg, []string{"sideways"}); err == nil {
		t.Error("runMigrate accepted an unknown action")
	}
}
