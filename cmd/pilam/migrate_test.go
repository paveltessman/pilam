package main

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/paveltessman/pilam/internal/platform/config"
)

const urlEnv = "TEST_DATABASE_URL"

func TestMigrateUpAndDown(t *testing.T) {
	url := os.Getenv(urlEnv)
	if url == "" {
		t.Fatalf("This test needs a database. Make sure db is running and %s is set", urlEnv)
	}
	cfg := config.Config{Database: config.Database{URL: url}}
	ctx := t.Context()

	// goose keeps its bookkeeping in a table of its own; a later run of this
	// test has to start from nothing. The test's context is already cancelled
	// by the time cleanups run, hence a fresh one here.
	t.Cleanup(func() {
		ctx := context.WithoutCancel(ctx)

		sqlDB, err := sql.Open(migrateDriver, cfg.Database.URL)
		if err != nil {
			t.Fatalf("cleanup: %v", err)
		}
		defer func() { _ = sqlDB.Close() }()

		if _, err := sqlDB.ExecContext(ctx, `DROP TABLE IF EXISTS goose_db_version`); err != nil {
			t.Fatalf("cleanup: %v", err)
		}
	})

	for _, action := range []string{"up", "status", "down", "up"} {
		if err := runMigrate(ctx, cfg, []string{action}); err != nil {
			t.Fatalf("migrate %s: %v", action, err)
		}
	}
}

func TestMigrateRejectsAnUnknownAction(t *testing.T) {
	cfg := config.Config{Database: config.Database{URL: "postgres://pilam:pilam@127.0.0.1:1/pilam"}}
	if err := runMigrate(t.Context(), cfg, []string{"sideways"}); err == nil {
		t.Error("runMigrate accepted an unknown action")
	}
}
