package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/platform/clock"
	"github.com/paveltessman/pilam/internal/platform/config"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/password"
	"github.com/paveltessman/pilam/internal/postgres"
	"github.com/paveltessman/pilam/internal/seed"
)

// How much of the dataset the tests below load. The whole roster hashes a
// password per row, and the whole season writes 140 models, and these tests only
// read what one row looks like.
const (
	seedCount  = "3"
	modelCount = "6"
)

// loadSeed runs the seed command and returns the report it printed.
func loadSeed(t *testing.T, cfg config.Config, args ...string) (string, error) {
	t.Helper()

	var out bytes.Buffer
	err := seedAll(t.Context(), cfg, &out, args)
	return out.String(), err
}

func TestSeedWritesUsersThatCanLogIn(t *testing.T) {
	cfg := userDB(t)

	report, err := loadSeed(t, cfg, "-users", seedCount, "-models", modelCount, "-domain", "example.test")
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

	if _, err := loadSeed(t, cfg, "-users", seedCount, "-models", modelCount); err != nil {
		t.Fatalf("The first run failed: %v", err)
	}
	first := listUsers(t, cfg)
	firstModels := listModels(t, cfg)

	report, err := loadSeed(t, cfg, "-users", seedCount, "-models", modelCount)
	if err != nil {
		t.Fatalf("The second run failed: %v", err)
	}
	if !strings.Contains(report, "created:  0") {
		t.Errorf("The second run wrote users again:\n%s", report)
	}
	if !strings.Contains(report, "models:  0 created, "+modelCount+" already there") {
		t.Errorf("The second run wrote models again:\n%s", report)
	}

	secondModels := listModels(t, cfg)
	if len(secondModels) != len(firstModels) {
		t.Fatalf("The second run left %d models, want %d", len(secondModels), len(firstModels))
	}
	for i, model := range secondModels {
		if model.ID != firstModels[i].ID {
			t.Errorf("The second run moved model %s", firstModels[i].Article)
		}
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
		"unknown flag":      {"-employees", seedCount},
		"a bad user count":  {"-users", "many"},
		"a bad model count": {"-models", "many"},
		"no connection":     {"-users", seedCount},
	}

	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := loadSeed(t, cfg, args...); err == nil {
				t.Error("seed accepted the arguments")
			}
		})
	}
}

func TestSeedWritesTheCatalog(t *testing.T) {
	cfg := userDB(t)

	report, err := loadSeed(t, cfg, "-users", seedCount, "-models", modelCount)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if !strings.Contains(report, "seasons: 1 created") {
		t.Errorf("The report names no season:\n%s", report)
	}

	svc := catalogOf(t, cfg)
	ctx := t.Context()

	seasons, err := svc.ListSeasons(ctx)
	if err != nil {
		t.Fatalf("ListSeasons: %v", err)
	}
	if len(seasons) != 1 {
		t.Fatalf("The database holds %d seasons, want 1", len(seasons))
	}

	drops, err := svc.ListDrops(ctx, seasons[0].ID)
	if err != nil {
		t.Fatalf("ListDrops: %v", err)
	}
	if len(drops) != 3 {
		t.Fatalf("The season holds %d drops, want 3", len(drops))
	}
	for i := 1; i < len(drops); i++ {
		if !drops[i-1].TargetDate.Before(drops[i].TargetDate) {
			t.Errorf("The drops are not spread across the season: %v", drops)
		}
	}

	models := listModels(t, cfg)
	if len(models) != 6 {
		t.Fatalf("The database holds %d models, want 6", len(models))
	}

	photos := 0
	for _, model := range models {
		strip, err := svc.ListPhotos(ctx, model.ID)
		if err != nil {
			t.Fatalf("ListPhotos: %v", err)
		}
		photos += len(strip)
	}
	if photos == 0 {
		t.Error("No seeded model carries a photo")
	}
}

// catalogOf opens the catalog service over the test database.
func catalogOf(t *testing.T, cfg config.Config) *catalog.Service {
	t.Helper()

	db, err := postgres.Open(t.Context(), cfg.Database)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(db.Close)

	idGen := ids.NewDeterministic(seed.IDSeed)
	trail := audit.NewTrail(postgres.NewAudit(db), clock.New(time.UTC), idGen)
	return catalog.NewService(catalog.Stores{
		Seasons: postgres.NewSeasons(db),
		Drops:   postgres.NewDrops(db),
		Models:  postgres.NewModels(db),
		Photos:  postgres.NewPhotos(db),
		Atomic:  db,
	}, idGen, trail)
}

// listModels returns every model in the database, ordered by article.
func listModels(t *testing.T, cfg config.Config) []catalog.Model {
	t.Helper()

	models, err := catalogOf(t, cfg).ListModels(t.Context(), catalog.ModelListParams{})
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	return models
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
