package postgres

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	// Registers the "pgx" database/sql driver goose runs the migrations over.
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	pilamdb "github.com/paveltessman/pilam/db"
	"github.com/paveltessman/pilam/internal/platform/config"
)

const urlEnv = "TEST_DATABASE_URL"

const migrateDriver = "pgx"

const schemaPrefix = "pilam_test_"

// Postgres truncates an identifier at 63 bytes. The test name gets what is left
// after the prefix.
const maxSchemaName = 63

// openTestDB gives the test the whole schema, in a Postgres schema of its own.
//
// One schema per test keeps the packages apart: `go test ./...` runs them at the
// same time, and the migrate test in cmd/pilam takes the public schema down to
// empty and back.
func openTestDB(t *testing.T) *DB {
	t.Helper()

	raw := os.Getenv(urlEnv)
	if raw == "" {
		t.Fatalf("This test needs a database. Make sure db is running and %s is set", urlEnv)
	}

	schema := schemaName(t)
	scoped := searchPath(t, raw, schema)

	// The test's own context is cancelled before its cleanups run, so the drop
	// gets a context of its own.
	dropCtx := context.WithoutCancel(t.Context())
	t.Cleanup(func() { dropSchema(dropCtx, scoped, schema) })
	migrateSchema(t, scoped, schema)

	db, err := Open(t.Context(), config.Database{URL: scoped})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// Registered after the drop, so it runs before it: the pool closes first.
	t.Cleanup(db.Close)

	return db
}

// schemaName turns the test name into an identifier. It is the same on every
// run, so a schema left behind by a crash names the test that made it.
func schemaName(t *testing.T) string {
	t.Helper()

	readable := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		default:
			return '_'
		}
	}, t.Name())

	name := schemaPrefix + readable
	if len(name) > maxSchemaName {
		name = name[:maxSchemaName]
	}
	return name
}

// searchPath returns raw with the connection pinned to schema.
func searchPath(t *testing.T, raw, schema string) string {
	t.Helper()

	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parsing %s: %v", urlEnv, err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()

	return parsed.String()
}

// migrateSchema creates the schema and applies every embedded migration to it.
// A schema left behind by an earlier run is dropped first.
func migrateSchema(t *testing.T, dsn, schema string) {
	t.Helper()

	sqlDB, err := sql.Open(migrateDriver, dsn)
	if err != nil {
		t.Fatalf("opening %s: %v", schema, err)
	}
	defer func() { _ = sqlDB.Close() }()

	ctx := t.Context()
	quoted := pgx.Identifier{schema}.Sanitize()
	for _, statement := range []string{
		`DROP SCHEMA IF EXISTS ` + quoted + ` CASCADE`,
		`CREATE SCHEMA ` + quoted,
	} {
		if _, err := sqlDB.ExecContext(ctx, statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}

	migrations, err := pilamdb.Migrations()
	if err != nil {
		t.Fatalf("Migrations: %v", err)
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations,
		goose.WithLogger(goose.NopLogger()),
	)
	if err != nil {
		t.Fatalf("goose provider: %v", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("migrating %s: %v", schema, err)
	}
}

// dropSchema removes the schema and everything the test put in it. A failure
// here is reported, not fatal: the test itself is already over.
func dropSchema(ctx context.Context, dsn, schema string) {
	sqlDB, err := sql.Open(migrateDriver, dsn)
	if err != nil {
		return
	}
	defer func() { _ = sqlDB.Close() }()

	_, _ = sqlDB.ExecContext(ctx, `DROP SCHEMA IF EXISTS `+pgx.Identifier{schema}.Sanitize()+` CASCADE`)
}
