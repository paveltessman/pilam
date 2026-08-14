// Package db carries the SQL sources into the binary: the goose migrations
// here, and the sqlc query files next to them.
//
// The migrations are embedded rather than read from disk so that `pilam
// migrate` works from the same single binary that serves traffic, with no
// assumption that the source tree is present. There is no Go code here beyond
// the embed: everything that runs SQL lives in internal/postgres.
package db

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed migrations/*.sql
var migrations embed.FS

// the -dir the goose CLI is pointed at when creating a new one. See `make migration`.
const MigrationsDir = "migrations"

func Migrations() (fs.FS, error) {
	sub, err := fs.Sub(migrations, MigrationsDir)
	if err != nil {
		return nil, fmt.Errorf("db: rooting migrations at %s: %w", MigrationsDir, err)
	}
	return sub, nil
}
