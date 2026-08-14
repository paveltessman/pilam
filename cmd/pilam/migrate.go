package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"text/tabwriter"
	"time"

	// Registers the "pgx" database/sql driver used below.
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/paveltessman/pilam/db"
	"github.com/paveltessman/pilam/internal/platform/config"
)

const migrateDriver = "pgx"

// The actions migrate accepts. Bare `migrate` means `migrate up`.
var migrateActions = map[string]func(context.Context, *goose.Provider) error{
	"up":      migrateUp,
	"down":    migrateDown,
	"status":  migrateStatus,
	"version": migrateVersion,
}

func runMigrate(ctx context.Context, cfg config.Config, args []string) error {
	action := "up"
	if len(args) > 0 {
		action = args[0]
	}
	run, ok := migrateActions[action]
	if !ok {
		return fmt.Errorf("migrate: unknown action %q, want up, down, status or version", action)
	}

	migrations, err := db.Migrations()
	if err != nil {
		return err
	}

	sqlDB, err := sql.Open(migrateDriver, cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("migrate: can't open database: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()

	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations,
		// goose logs each applied migration itself; routing that through the
		// configured logger keeps one format in the output of one process.
		goose.WithSlog(slog.Default()),
	)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	return run(ctx, provider)
}

// migrateVersion reports the version the database is currently at.
func migrateVersion(ctx context.Context, provider *goose.Provider) error {
	version, err := provider.GetDBVersion(ctx)
	if err != nil {
		return fmt.Errorf("migrate version: %w", err)
	}
	slog.Info("schema version", "version", version)
	return nil
}

// migrateUp applies every pending migration.
func migrateUp(ctx context.Context, provider *goose.Provider) error {
	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	if len(results) == 0 {
		slog.Info("schema is up to date")
		return nil
	}
	for _, r := range results {
		slog.Info("applied", "migration", r.String())
	}
	return nil
}

// migrateDown rolls back exactly one migration.
func migrateDown(ctx context.Context, provider *goose.Provider) error {
	result, err := provider.Down(ctx)
	if err != nil {
		if errors.Is(err, goose.ErrNoNextVersion) {
			slog.Info("nothing to roll back")
			return nil
		}
		return fmt.Errorf("unable to migrate down: %w", err)
	}
	slog.Info("rolled back", "migration", result.String())
	return nil
}

// migrateStatus prints what is applied and what is pending.
func migrateStatus(ctx context.Context, provider *goose.Provider) error {
	statuses, err := provider.Status(ctx)
	if err != nil {
		return fmt.Errorf("migrate status: %w", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Writes go to a buffer; Flush below is where a failure actually surfaces.
	_, _ = fmt.Fprintln(w, "VERSION\tSTATE\tAPPLIED AT\tSOURCE")
	for _, s := range statuses {
		appliedAt := "-"
		if !s.AppliedAt.IsZero() {
			appliedAt = s.AppliedAt.Format(time.RFC3339)
		}
		// Path is the bare file name: the embedded filesystem is rooted at the
		// migrations directory.
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", s.Source.Version, s.State, appliedAt, s.Source.Path)
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("migrate status: %w", err)
	}
	return nil
}
