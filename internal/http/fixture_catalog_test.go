package http

import (
	"context"
	"testing"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/platform/ids"
)

// The catalog stores the router is built over.
type (
	emptySeasons struct{}
	emptyDrops   struct{}
	emptyModels  struct{}
	emptyPhotos  struct{}
)

func (emptySeasons) ByID(context.Context, ids.ID) (catalog.Season, error) {
	return catalog.Season{}, catalog.ErrNoSeason
}
func (emptySeasons) List(context.Context) ([]catalog.Season, error) { return nil, nil }
func (emptySeasons) Create(context.Context, catalog.Season) error   { return nil }
func (emptySeasons) Update(context.Context, catalog.Season) error   { return catalog.ErrNoSeason }

func (emptyDrops) ByID(context.Context, ids.ID) (catalog.Drop, error) {
	return catalog.Drop{}, catalog.ErrNoDrop
}
func (emptyDrops) ListBySeason(context.Context, ids.ID) ([]catalog.Drop, error) { return nil, nil }
func (emptyDrops) Create(context.Context, catalog.Drop) error                   { return nil }
func (emptyDrops) Update(context.Context, catalog.Drop) error                   { return catalog.ErrNoDrop }

func (emptyModels) ByID(context.Context, ids.ID) (catalog.Model, error) {
	return catalog.Model{}, catalog.ErrNoModel
}
func (emptyModels) List(context.Context, catalog.ModelListParams) ([]catalog.Model, error) {
	return nil, nil
}
func (emptyModels) Create(context.Context, catalog.Model) error { return nil }
func (emptyModels) Update(context.Context, catalog.Model) error { return catalog.ErrNoModel }

func (emptyPhotos) ByModel(context.Context, ids.ID) ([]catalog.Photo, error) { return nil, nil }
func (emptyPhotos) Add(context.Context, catalog.Photo) error                 { return nil }
func (emptyPhotos) Remove(context.Context, ids.ID) error                     { return catalog.ErrNoPhoto }
func (emptyPhotos) Reorder(context.Context, []ids.ID) error                  { return nil }

// emptyCatalog is the service the test router is wired with.
func emptyCatalog(t *testing.T) *catalog.Service {
	t.Helper()

	gen := ids.NewGenerator()
	stores := catalog.Stores{
		Seasons: emptySeasons{},
		Drops:   emptyDrops{},
		Models:  emptyModels{},
		Photos:  emptyPhotos{},
		Atomic:  directAtomic{},
	}
	return catalog.NewService(stores, gen, audit.NewTrail(&recorded{}, testClock(), gen))
}
