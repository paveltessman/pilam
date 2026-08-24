package postgres

import (
	"errors"
	"testing"

	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/platform/date"
)

func TestDropsRoundTripEveryField(t *testing.T) {
	t.Parallel()

	store := catalogDB(t)
	season := store.season(t, "S1", "2026-11-01")
	want := store.drop(t, season, "Drop 1", "2027-02-15")

	got, err := store.drops.ByID(t.Context(), want.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}

	if got != want {
		t.Errorf("ByID = %+v, want %+v", got, want)
	}
}

func TestDropsHoldOneNamePerSeason(t *testing.T) {
	t.Parallel()

	store := catalogDB(t)
	first := store.season(t, "S1", "2026-11-01")
	second := store.season(t, "S2", "2027-05-01")
	store.drop(t, first, "Drop 1", "2027-02-15")

	taken := catalog.Drop{
		ID:         gen.New(),
		SeasonID:   first.ID,
		Name:       "drop 1",
		TargetDate: date.MustParse("2027-03-15"),
		Active:     true,
	}
	if err := store.drops.Create(t.Context(), taken); !errors.Is(err, catalog.ErrDropNameTaken) {
		t.Errorf("Create error = %v, want %v", err, catalog.ErrDropNameTaken)
	}

	// The same name in another season is a different drop.
	store.drop(t, second, "Drop 1", "2027-08-15")
}

// The drop keeps the season it was created in.
func TestDropsUpdateLeavesTheSeasonAsItStands(t *testing.T) {
	t.Parallel()

	store := catalogDB(t)
	first := store.season(t, "S1", "2026-11-01")
	second := store.season(t, "S2", "2027-05-01")
	drop := store.drop(t, first, "Drop 1", "2027-02-15")

	drop.SeasonID = second.ID
	drop.Name = "Drop 1a"
	if err := store.drops.Update(t.Context(), drop); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := store.drops.ByID(t.Context(), drop.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if got.SeasonID != first.ID {
		t.Errorf("SeasonID = %s, want the season it was created in, %s", got.SeasonID, first.ID)
	}
	if got.Name != "Drop 1a" {
		t.Errorf("Name = %q, want %q", got.Name, "Drop 1a")
	}
}

func TestDropsListBySeasonOrdersByTargetDate(t *testing.T) {
	t.Parallel()

	store := catalogDB(t)
	season := store.season(t, "S1", "2026-11-01")
	other := store.season(t, "S2", "2027-05-01")

	store.drop(t, season, "Drop 2", "2027-04-15")
	store.drop(t, season, "Drop 1", "2027-02-15")
	store.drop(t, other, "Drop 1", "2027-08-15")

	got, err := store.drops.ListBySeason(t.Context(), season.ID)
	if err != nil {
		t.Fatalf("ListBySeason: %v", err)
	}
	if len(got) != 2 || got[0].Name != "Drop 1" || got[1].Name != "Drop 2" {
		t.Errorf("ListBySeason = %+v, want Drop 1 then Drop 2 of season %s", got, season.ID)
	}
}

func TestDropsReportMissingRowAsErrNoDrop(t *testing.T) {
	t.Parallel()

	store := catalogDB(t)

	if _, err := store.drops.ByID(t.Context(), gen.New()); !errors.Is(err, catalog.ErrNoDrop) {
		t.Errorf("ByID error = %v, want %v", err, catalog.ErrNoDrop)
	}
}
