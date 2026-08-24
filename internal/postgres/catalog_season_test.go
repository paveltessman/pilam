package postgres

import (
	"errors"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/platform/date"
)

func TestSeasonsRoundTripEveryField(t *testing.T) {
	t.Parallel()

	store := catalogDB(t)
	want := store.season(t, "S1", "2026-11-01")

	got, err := store.seasons.ByID(t.Context(), want.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}

	if got != want {
		t.Errorf("ByID = %+v, want %+v", got, want)
	}
}

// A date column is a calendar day. pgx must hand it back as UTC midnight.
func TestSeasonsReadTheStartDateAsUTCMidnight(t *testing.T) {
	t.Parallel()

	store := catalogDB(t)
	want := store.season(t, "S1", "2026-11-01")

	got, err := store.seasons.ByID(t.Context(), want.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}

	if !got.StartDate.Equal(date.Of(2026, time.November, 1)) {
		t.Errorf("StartDate = %v, want 2026-11-01 at UTC midnight", got.StartDate)
	}
	if got.StartDate.Location() != time.UTC {
		t.Errorf("StartDate zone = %v, want UTC", got.StartDate.Location())
	}
}

func TestSeasonsReportMissingRowAsErrNoSeason(t *testing.T) {
	t.Parallel()

	store := catalogDB(t)

	if _, err := store.seasons.ByID(t.Context(), gen.New()); !errors.Is(err, catalog.ErrNoSeason) {
		t.Errorf("ByID error = %v, want %v", err, catalog.ErrNoSeason)
	}

	missing := catalog.Season{ID: gen.New(), Name: "S1", StartDate: date.MustParse("2026-11-01")}
	if err := store.seasons.Update(t.Context(), missing); !errors.Is(err, catalog.ErrNoSeason) {
		t.Errorf("Update error = %v, want %v", err, catalog.ErrNoSeason)
	}
}

func TestSeasonsRefuseATakenNameWhateverTheCase(t *testing.T) {
	t.Parallel()

	store := catalogDB(t)
	store.season(t, "S1", "2026-11-01")

	for _, name := range []string{"S1", "s1"} {
		taken := catalog.Season{ID: gen.New(), Name: name, StartDate: date.MustParse("2027-05-01"), Active: true}
		if err := store.seasons.Create(t.Context(), taken); !errors.Is(err, catalog.ErrSeasonNameTaken) {
			t.Errorf("Create %q error = %v, want %v", name, err, catalog.ErrSeasonNameTaken)
		}
	}
}

func TestSeasonsUpdateWritesEveryFieldAndStampsTheRow(t *testing.T) {
	t.Parallel()

	store := catalogDB(t)
	want := store.season(t, "S1", "2026-11-01")

	_, err := store.seasons.ByID(t.Context(), want.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}

	want.Name = "S1 Main"
	want.StartDate = date.MustParse("2026-12-01")
	want.Active = false
	if err := store.seasons.Update(t.Context(), want); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := store.seasons.ByID(t.Context(), want.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}

	if got != want {
		t.Errorf("ByID = %+v, want %+v", got, want)
	}
}

func TestSeasonsListOrdersByStartDate(t *testing.T) {
	t.Parallel()

	store := catalogDB(t)
	store.season(t, "A1", "2027-05-01")
	store.season(t, "S1", "2026-11-01")

	got, err := store.seasons.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 || got[0].Name != "S1" || got[1].Name != "A1" {
		t.Errorf("List = %+v, want S1 then A1", got)
	}
}
