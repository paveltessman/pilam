package postgres

import (
	"errors"
	"slices"
	"testing"

	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/platform/ids"
)

func TestPhotosRoundTripTheStripInOrder(t *testing.T) {
	store := catalogDB(t)
	season := store.season(t, "S1", "2026-11-01")
	drop := store.drop(t, season, "Drop 1", "2027-02-15")
	model := store.model(t, drop, "A-100")

	second := store.photo(t, model, "b", 1)
	first := store.photo(t, model, "a", 0)

	got, err := store.photos.ByModel(t.Context(), model.ID)
	if err != nil {
		t.Fatalf("ByModel: %v", err)
	}
	if want := []catalog.Photo{first, second}; !slices.Equal(got, want) {
		t.Errorf("ByModel = %+v, want %+v", got, want)
	}
}

// A reorder swaps two positions inside one transaction. The unique constraint
// over (model_id, position) is deferred, so the swap is not a violation.
func TestPhotosReorderSwapsTwoPositions(t *testing.T) {
	store := catalogDB(t)
	season := store.season(t, "S1", "2026-11-01")
	drop := store.drop(t, season, "Drop 1", "2027-02-15")
	model := store.model(t, drop, "A-100")

	first := store.photo(t, model, "a", 0)
	second := store.photo(t, model, "b", 1)

	if err := store.photos.Reorder(t.Context(), []ids.ID{second.ID, first.ID}); err != nil {
		t.Fatalf("Reorder: %v", err)
	}

	got, err := store.photos.ByModel(t.Context(), model.ID)
	if err != nil {
		t.Fatalf("ByModel: %v", err)
	}
	if len(got) != 2 || got[0].ID != second.ID || got[1].ID != first.ID {
		t.Fatalf("ByModel = %+v, want the swapped strip", got)
	}
	if got[0].Position != 0 || got[1].Position != 1 {
		t.Errorf("positions = %d and %d, want 0 and 1", got[0].Position, got[1].Position)
	}
}

func TestPhotosRemoveDeletesOneRow(t *testing.T) {
	store := catalogDB(t)
	season := store.season(t, "S1", "2026-11-01")
	drop := store.drop(t, season, "Drop 1", "2027-02-15")
	model := store.model(t, drop, "A-100")
	photo := store.photo(t, model, "a", 0)

	if err := store.photos.Remove(t.Context(), photo.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	got, err := store.photos.ByModel(t.Context(), model.ID)
	if err != nil {
		t.Fatalf("ByModel: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ByModel returned %d photos, want 0", len(got))
	}

	if err := store.photos.Remove(t.Context(), photo.ID); !errors.Is(err, catalog.ErrNoPhoto) {
		t.Errorf("Remove error = %v, want %v", err, catalog.ErrNoPhoto)
	}
}
