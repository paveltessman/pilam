package postgres

import (
	"errors"
	"slices"
	"testing"

	"github.com/paveltessman/pilam/internal/catalog"
)

func TestModelsRoundTripEveryField(t *testing.T) {
	store := catalogDB(t)
	season := store.season(t, "S1", "2026-11-01")
	drop := store.drop(t, season, "Drop 1", "2027-02-15")
	want := store.model(t, drop, "A-100")

	got, err := store.models.ByID(t.Context(), want.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}

	if got != want {
		t.Errorf("ByID = %+v, want %+v", got, want)
	}
}

// The season filter reaches every drop of the season, through the join.
func TestModelsFilterBySeasonAcrossEveryDropOfIt(t *testing.T) {
	store := catalogDB(t)
	season := store.season(t, "S1", "2026-11-01")
	other := store.season(t, "S2", "2027-05-01")

	first := store.drop(t, season, "Drop 1", "2027-02-15")
	second := store.drop(t, season, "Drop 2", "2027-04-15")
	elsewhere := store.drop(t, other, "Drop 1", "2027-08-15")

	store.model(t, first, "A-100")
	store.model(t, second, "A-200")
	store.model(t, elsewhere, "B-100")

	got, err := store.models.List(t.Context(), catalog.ModelListParams{SeasonID: season.ID})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if want := []string{"A-100", "A-200"}; !slices.Equal(articles(got), want) {
		t.Errorf("articles = %v, want %v", articles(got), want)
	}

	byDrop, err := store.models.List(t.Context(), catalog.ModelListParams{DropID: first.ID})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if want := []string{"A-100"}; !slices.Equal(articles(byDrop), want) {
		t.Errorf("articles = %v, want %v", articles(byDrop), want)
	}
}

func TestModelsFilterByTheActiveFlag(t *testing.T) {
	store := catalogDB(t)
	season := store.season(t, "S1", "2026-11-01")
	drop := store.drop(t, season, "Drop 1", "2027-02-15")

	store.model(t, drop, "A-200")
	off := store.model(t, drop, "A-100")
	off.Active = false
	if err := store.models.Update(t.Context(), off); err != nil {
		t.Fatalf("Update: %v", err)
	}

	on, gone := true, false
	for name, tc := range map[string]struct {
		filter catalog.ModelListParams
		want   []string
	}{
		"both":     {catalog.ModelListParams{}, []string{"A-100", "A-200"}},
		"active":   {catalog.ModelListParams{Active: &on}, []string{"A-200"}},
		"inactive": {catalog.ModelListParams{Active: &gone}, []string{"A-100"}},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := store.models.List(t.Context(), tc.filter)
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if !slices.Equal(articles(got), tc.want) {
				t.Errorf("articles = %v, want %v", articles(got), tc.want)
			}
		})
	}
}

func TestModelsReportMissingRowAsErrNoModel(t *testing.T) {
	store := catalogDB(t)

	if _, err := store.models.ByID(t.Context(), gen.New()); !errors.Is(err, catalog.ErrNoModel) {
		t.Errorf("ByID error = %v, want %v", err, catalog.ErrNoModel)
	}

	missing := catalog.Model{ID: gen.New(), Article: "A-100"}
	if err := store.models.Update(t.Context(), missing); !errors.Is(err, catalog.ErrNoModel) {
		t.Errorf("Update error = %v, want %v", err, catalog.ErrNoModel)
	}
}
