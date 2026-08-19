package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/platform/config"
	"github.com/paveltessman/pilam/internal/platform/password"
	"github.com/paveltessman/pilam/internal/postgres"
	"github.com/paveltessman/pilam/internal/seed"
)

// seedCount is how many employees the tests below load. The whole roster hashes
// a password per row, and these tests only read what one row looks like.
const seedCount = "3"

// loadSeed runs the seed command and returns the report it printed.
func loadSeed(t *testing.T, cfg config.Config, args ...string) (string, error) {
	t.Helper()

	var out bytes.Buffer
	err := seedAll(t.Context(), cfg, &out, args)
	return out.String(), err
}

func TestSeedWritesUsersThatCanLogIn(t *testing.T) {
	cfg := userDB(t)

	report, err := loadSeed(t, cfg, "-users", seedCount, "-domain", "example.test")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if !strings.Contains(report, seed.DefaultPasswd) {
		t.Errorf("The report does not name the password:\n%s", report)
	}

	users := listUsers(t, cfg)
	if len(users) != 3 {
		t.Fatalf("The database holds %d users, want 3", len(users))
	}

	for _, user := range users {
		if !strings.HasSuffix(user.Email, "@example.test") {
			t.Errorf("Incorrect address: %q", user.Email)
		}
		if user.PasswdExpired {
			t.Errorf("%s has to change the password before reaching the board", user.Email)
		}

		ok, _, err := password.Verify(user.PasswdHash, seed.DefaultPasswd)
		if err != nil {
			t.Fatalf("Verify: %v", err)
		}
		if !ok {
			t.Errorf("The reported password does not open %s", user.Email)
		}
	}
}

func TestSeedRunsTwiceOverTheSameDatabase(t *testing.T) {
	cfg := userDB(t)

	if _, err := loadSeed(t, cfg, "-users", seedCount); err != nil {
		t.Fatalf("The first run failed: %v", err)
	}
	first := listUsers(t, cfg)

	report, err := loadSeed(t, cfg, "-users", seedCount)
	if err != nil {
		t.Fatalf("The second run failed: %v", err)
	}
	if !strings.Contains(report, "created:  0") {
		t.Errorf("The second run wrote users again:\n%s", report)
	}

	second := listUsers(t, cfg)
	if len(second) != len(first) {
		t.Fatalf("The second run left %d users, want %d", len(second), len(first))
	}
	for i, user := range second {
		if user.ID != first[i].ID || user.Email != first[i].Email {
			t.Errorf("The second run moved a row: want=%s %s, got=%s %s",
				first[i].ID, first[i].Email, user.ID, user.Email)
		}
	}
}

func TestSeedRefusesFlagsItCantWorkFrom(t *testing.T) {
	cfg := config.Config{
		Database: config.Database{URL: "postgres://pilam:pilam@127.0.0.1:1/pilam"},
		Timezone: time.UTC,
	}

	cases := map[string][]string{
		"unknown flag":  {"-employees", seedCount},
		"a bad count":   {"-users", "many"},
		"no connection": {"-users", seedCount},
	}

	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := loadSeed(t, cfg, args...); err == nil {
				t.Error("seed accepted the arguments")
			}
		})
	}
}

// listUsers returns every user in the database, ordered by email.
func listUsers(t *testing.T, cfg config.Config) []auth.User {
	t.Helper()

	db, err := postgres.Open(t.Context(), cfg.Database)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(db.Close)

	users, err := postgres.NewUsers(db).List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	return users
}
