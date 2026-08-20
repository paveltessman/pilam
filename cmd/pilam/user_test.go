package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/platform/config"
	"github.com/paveltessman/pilam/internal/platform/password"
	"github.com/paveltessman/pilam/internal/postgres"
)

// passwdPrefix is what the report writes the generated password behind.
const passwdPrefix = "password:"

// userDB gives the test the whole schema, and takes it back down afterwards.
//
// The command reads the public schema, the one the migrate test also works in.
// Both live in this package, so they run one after the other.
func userDB(t *testing.T) config.Config {
	t.Helper()

	url := os.Getenv(urlEnv)
	if url == "" {
		t.Fatalf("This test needs a database. Make sure db is running and %s is set", urlEnv)
	}

	cfg := config.Config{
		Database: config.Database{URL: url},
		Media:    config.Media{Dir: t.TempDir()},
		Timezone: time.UTC,
	}
	ctx := t.Context()
	count := migrationCount(t)

	// The test's context is already cancelled by the time cleanups run, hence a
	// fresh one here.
	t.Cleanup(func() { rollBack(context.WithoutCancel(ctx), cfg, count) })
	rollBack(ctx, cfg, count)

	if err := runMigrate(ctx, cfg, []string{"up"}); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	return cfg
}

// add runs the 'userAdd' command and returns what it printed.
func add(t *testing.T, cfg config.Config, args ...string) (string, error) {
	t.Helper()

	var out bytes.Buffer
	err := userAdd(t.Context(), cfg, &out, args)
	return out.String(), err
}

// printedPassword returns the one password the report carries, and fails the
// test when the report names none or more than one.
func printedPassword(t *testing.T, report string) string {
	t.Helper()

	var found []string
	for line := range strings.SplitSeq(report, "\n") {
		if _, value, ok := strings.Cut(line, passwdPrefix); ok {
			found = append(found, strings.TrimSpace(value))
		}
	}
	if len(found) != 1 {
		t.Fatalf("The report holds %d password lines, want 1:\n%s", len(found), report)
	}
	if found[0] == "" {
		t.Fatalf("The report holds an empty password:\n%s", report)
	}
	return found[0]
}

// stored loads the row the command wrote.
func stored(t *testing.T, cfg config.Config, email string) auth.User {
	t.Helper()

	db, err := postgres.Open(t.Context(), cfg.Database)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(db.Close)

	user, err := postgres.NewUsers(db).ByEmail(t.Context(), email)
	if err != nil {
		t.Fatalf("ByEmail %q: %v", email, err)
	}
	return user
}

func TestUserAddCreatesUser(t *testing.T) {
	cfg := userDB(t)

	report, err := add(t, cfg, "-email", " Ada@Example.COM ", "-name", "Ada Lovelace", "-root")
	if err != nil {
		t.Fatalf("user add: %v", err)
	}
	passwd := printedPassword(t, report)

	user := stored(t, cfg, "ada@example.com")
	switch {
	case user.FirstName != "Ada" || user.LastName != "Lovelace":
		t.Errorf("Incorrect name: %q %q", user.FirstName, user.LastName)
	case user.Role != auth.RootRole:
		t.Errorf("Incorrect role: want=%q, got=%q", auth.RootRole, user.Role)
	case !user.Active:
		t.Error("The new user is not active")
	case !user.PasswdExpired:
		t.Error("The new user does not have to change the password")
	}

	ok, _, err := password.Verify(user.PasswdHash, passwd)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !ok {
		t.Error("The printed password does not open the account")
	}

	if count := strings.Count(report, passwd); count != 1 {
		t.Errorf("The report prints the password %d times, want 1:\n%s", count, report)
	}
}

func TestUserAddWithoutRootFlagCreatesMember(t *testing.T) {
	cfg := userDB(t)

	if _, err := add(t, cfg, "-email", "grace@example.com", "-name", "Grace Hopper"); err != nil {
		t.Fatalf("user add: %v", err)
	}

	if role := stored(t, cfg, "grace@example.com").Role; role != auth.MemberRole {
		t.Errorf("Incorrect role: want=%q, got=%q", auth.MemberRole, role)
	}
}

func TestUserAddRefusesTakenEmail(t *testing.T) {
	cfg := userDB(t)

	if _, err := add(t, cfg, "-email", "ada@example.com", "-name", "Ada Lovelace"); err != nil {
		t.Fatalf("user add: %v", err)
	}

	report, err := add(t, cfg, "-email", "ADA@example.com", "-name", "Ada Byron")
	if !errors.Is(err, auth.ErrEmailTaken) {
		t.Errorf("Incorrect error for a taken email: %v", err)
	}
	if !strings.Contains(err.Error(), "ada@example.com") {
		t.Errorf("The message does not name the address: %v", err)
	}
	if report != "" {
		t.Errorf("A refused command still printed a password:\n%s", report)
	}
}

func TestUserAddChecksItsFlags(t *testing.T) {
	cfg := config.Config{
		Database: config.Database{URL: "postgres://pilam:pilam@127.0.0.1:1/pilam"},
		Timezone: time.UTC,
	}

	cases := map[string][]string{
		"no email":       {"-name", "Ada Lovelace"},
		"empty email":    {"-email", "   ", "-name", "Ada Lovelace"},
		"no name":        {"-email", "ada@example.com"},
		"one name part":  {"-email", "ada@example.com", "-name", "Ada"},
		"unknown flag":   {"-email", "ada@example.com", "-name", "Ada Lovelace", "-admin"},
		"empty flag set": {},
	}

	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			if err := userAdd(t.Context(), cfg, io.Discard, args); err == nil {
				t.Error("user add accepted the arguments")
			}
		})
	}
}

func TestUserRefusesUnknownAction(t *testing.T) {
	cfg := config.Config{Database: config.Database{URL: "postgres://pilam:pilam@127.0.0.1:1/pilam"}}

	for name, args := range map[string][]string{
		"no action":      {},
		"unknown action": {"remove", "-email", "ada@example.com"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := runUser(t.Context(), cfg, args); err == nil {
				t.Error("runUser accepted the arguments")
			}
		})
	}
}
