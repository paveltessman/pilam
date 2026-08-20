package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/platform/date"
	"github.com/paveltessman/pilam/internal/platform/media"
)

// repos is the four ports over one database, which is the whole catalog store.
type repos struct {
	db      *DB
	seasons *Seasons
	drops   *Drops
	models  *Models
	photos  *Photos
}

func catalogDB(t *testing.T) repos {
	t.Helper()
	db := openTestDB(t)
	return repos{
		db:      db,
		seasons: NewSeasons(db),
		drops:   NewDrops(db),
		models:  NewModels(db),
		photos:  NewPhotos(db),
	}
}

// season writes one season and returns it.
func (r repos) season(t *testing.T, name, start string) catalog.Season {
	t.Helper()

	season := catalog.Season{ID: gen.New(), Name: name, StartDate: date.MustParse(start), Active: true}
	if err := r.seasons.Create(t.Context(), season); err != nil {
		t.Fatalf("Seasons.Create: %v", err)
	}
	return season
}

// drop writes one drop inside a season and returns it.
func (r repos) drop(t *testing.T, season catalog.Season, name, target string) catalog.Drop {
	t.Helper()

	drop := catalog.Drop{
		ID:         gen.New(),
		SeasonID:   season.ID,
		Name:       name,
		TargetDate: date.MustParse(target),
		Active:     true,
	}
	if err := r.drops.Create(t.Context(), drop); err != nil {
		t.Fatalf("Drops.Create: %v", err)
	}
	return drop
}

// model writes one model inside a drop and returns it.
func (r repos) model(t *testing.T, drop catalog.Drop, article string) catalog.Model {
	t.Helper()

	model := catalog.Model{ID: gen.New(), DropID: drop.ID, Article: article, Active: true}
	if err := r.models.Create(t.Context(), model); err != nil {
		t.Fatalf("Models.Create: %v", err)
	}
	return model
}

// photo appends one photo to the strip of a model and returns it.
func (r repos) photo(t *testing.T, model catalog.Model, digit string, position int) catalog.Photo {
	t.Helper()

	photo := catalog.Photo{
		ID:       gen.New(),
		ModelID:  model.ID,
		MediaKey: media.Key("images/aa/bb/" + strings.Repeat(digit, 64) + ".jpg"),
		Position: position,
	}
	if err := r.photos.Add(t.Context(), photo); err != nil {
		t.Fatalf("Photos.Add: %v", err)
	}
	return photo
}

// articles is the article of every model of a list, in order.
func articles(models []catalog.Model) []string {
	out := make([]string, len(models))
	for i, model := range models {
		out[i] = model.Article
	}
	return out
}

func TestCatalogWriteRollsBackWithTransaction(t *testing.T) {
	store := catalogDB(t)
	season := store.season(t, "S1", "2026-11-01")

	sentinel := errors.New("the work after the write failed")
	drop := catalog.Drop{
		ID:         gen.New(),
		SeasonID:   season.ID,
		Name:       "Drop 1",
		TargetDate: date.MustParse("2027-02-15"),
		Active:     true,
	}

	err := store.db.InTx(t.Context(), func(ctx context.Context) error {
		if err := store.drops.Create(ctx, drop); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("InTx error = %v, want %v", err, sentinel)
	}

	if _, err := store.drops.ByID(t.Context(), drop.ID); !errors.Is(err, catalog.ErrNoDrop) {
		t.Errorf("ByID error = %v, want %v: the rollback left the row behind", err, catalog.ErrNoDrop)
	}
}
