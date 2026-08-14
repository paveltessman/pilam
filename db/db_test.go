package db_test

import (
	"io/fs"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/db"
)

var migrationName = regexp.MustCompile(`^(\d{5})_[a-z0-9_]+\.sql$`)

func migrations(t *testing.T) []string {
	t.Helper()

	fsys, err := db.Migrations()
	if err != nil {
		t.Fatalf("Migrations: %v", err)
	}
	names, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		t.Fatalf("globbing: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("no migrations are embedded: goose would have nothing to apply")
	}
	return names
}

func TestMigrationsAreNamedInSequence(t *testing.T) {
	previous := 0
	for _, name := range migrations(t) {
		match := migrationName.FindStringSubmatch(name)
		if match == nil {
			t.Errorf("%s: want NNNNN_lower_snake_case.sql", name)
			continue
		}
		version, err := strconv.Atoi(match[1])
		if err != nil {
			t.Errorf("%s: unreadable version: %v", name, err)
			continue
		}
		if version <= previous {
			t.Errorf("%s: version %d does not follow %d", name, version, previous)
		}
		previous = version
	}
}

func TestEveryMigrationCanBeRolledBack(t *testing.T) {
	fsys, err := db.Migrations()
	if err != nil {
		t.Fatalf("Migrations: %v", err)
	}

	for _, name := range migrations(t) {
		content, err := fs.ReadFile(fsys, name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, annotation := range []string{"-- +goose Up", "-- +goose Down"} {
			if !strings.Contains(string(content), annotation) {
				t.Errorf("%s: missing %q", name, annotation)
			}
		}
	}
}
