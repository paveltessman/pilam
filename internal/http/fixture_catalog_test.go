package http

import (
	"context"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/platform/date"
	"github.com/paveltessman/pilam/internal/platform/ids"
)

// catalogStore is the in-memory stand-in for the four catalog ports. It holds
// the rules the screens depend on: the ordering of the two lists, and the
// duplicate name each write refuses.
type catalogStore struct {
	mu      sync.Mutex
	seasons map[ids.ID]catalog.Season
	drops   map[ids.ID]catalog.Drop
	models  map[ids.ID]catalog.Model
	photos  map[ids.ID]catalog.Photo
}

func newCatalogStore() *catalogStore {
	store := &catalogStore{
		seasons: make(map[ids.ID]catalog.Season),
		drops:   make(map[ids.ID]catalog.Drop),
		models:  make(map[ids.ID]catalog.Model),
		photos:  make(map[ids.ID]catalog.Photo),
	}
	return store
}

func (s *catalogStore) seasonByID(_ context.Context, id ids.ID) (catalog.Season, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	season, found := s.seasons[id]
	if !found {
		return catalog.Season{}, catalog.ErrNoSeason
	}
	return season, nil
}

// seasonList orders by the start date, then by the name, as §4.1 of the plan
// states.
func (s *catalogStore) seasonList(context.Context) ([]catalog.Season, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	seasons := slices.Collect(maps.Values(s.seasons))
	slices.SortFunc(seasons, func(a, b catalog.Season) int {
		if !date.Equal(a.StartDate, b.StartDate) {
			return a.StartDate.Compare(b.StartDate)
		}
		return strings.Compare(a.Name, b.Name)
	})
	return seasons, nil
}

func (s *catalogStore) seasonCreate(_ context.Context, season catalog.Season) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, held := range s.seasons {
		if strings.EqualFold(held.Name, season.Name) {
			return catalog.ErrSeasonNameTaken
		}
	}
	s.seasons[season.ID] = season
	return nil
}

func (s *catalogStore) seasonUpdate(_ context.Context, season catalog.Season) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, found := s.seasons[season.ID]; !found {
		return catalog.ErrNoSeason
	}
	for _, held := range s.seasons {
		if held.ID != season.ID && strings.EqualFold(held.Name, season.Name) {
			return catalog.ErrSeasonNameTaken
		}
	}
	s.seasons[season.ID] = season
	return nil
}

func (s *catalogStore) dropByID(_ context.Context, id ids.ID) (catalog.Drop, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	drop, found := s.drops[id]
	if !found {
		return catalog.Drop{}, catalog.ErrNoDrop
	}
	return drop, nil
}

func (s *catalogStore) dropListBySeason(_ context.Context, seasonID ids.ID) ([]catalog.Drop, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var drops []catalog.Drop
	for _, drop := range s.drops {
		if drop.SeasonID == seasonID {
			drops = append(drops, drop)
		}
	}
	slices.SortFunc(drops, func(a, b catalog.Drop) int { return a.TargetDate.Compare(b.TargetDate) })
	return drops, nil
}

func (s *catalogStore) dropCreate(_ context.Context, drop catalog.Drop) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, held := range s.drops {
		if held.SeasonID == drop.SeasonID && strings.EqualFold(held.Name, drop.Name) {
			return catalog.ErrDropNameTaken
		}
	}
	s.drops[drop.ID] = drop
	return nil
}

func (s *catalogStore) dropUpdate(_ context.Context, drop catalog.Drop) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, found := s.drops[drop.ID]; !found {
		return catalog.ErrNoDrop
	}
	for _, held := range s.drops {
		if held.ID != drop.ID && held.SeasonID == drop.SeasonID && strings.EqualFold(held.Name, drop.Name) {
			return catalog.ErrDropNameTaken
		}
	}
	s.drops[drop.ID] = drop
	return nil
}

func (s *catalogStore) modelByID(_ context.Context, id ids.ID) (catalog.Model, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	model, found := s.models[id]
	if !found {
		return catalog.Model{}, catalog.ErrNoModel
	}
	return model, nil
}

func (s *catalogStore) modelList(_ context.Context, filter catalog.ModelListParams) ([]catalog.Model, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	models := make([]catalog.Model, 0, len(s.models))
	for _, model := range s.models {
		switch {
		case filter.DropID != ids.Nil && model.DropID != filter.DropID:
			continue
		case filter.SeasonID != ids.Nil && s.drops[model.DropID].SeasonID != filter.SeasonID:
			continue
		case filter.Active != nil && model.Active != *filter.Active:
			continue
		}
		models = append(models, model)
	}
	slices.SortFunc(models, func(a, b catalog.Model) int { return strings.Compare(a.Article, b.Article) })
	return models, nil
}

func (s *catalogStore) modelCreate(_ context.Context, model catalog.Model) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.models[model.ID] = model
	return nil
}

func (s *catalogStore) modelUpdate(_ context.Context, model catalog.Model) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, found := s.models[model.ID]; !found {
		return catalog.ErrNoModel
	}
	s.models[model.ID] = model
	return nil
}

func (s *catalogStore) photoByModel(_ context.Context, modelID ids.ID) ([]catalog.Photo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var photos []catalog.Photo
	for _, photo := range s.photos {
		if photo.ModelID == modelID {
			photos = append(photos, photo)
		}
	}
	slices.SortFunc(photos, func(a, b catalog.Photo) int { return a.Position - b.Position })
	return photos, nil
}

func (s *catalogStore) photoAdd(_ context.Context, photo catalog.Photo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.photos[photo.ID] = photo
	return nil
}

func (s *catalogStore) photoRemove(_ context.Context, photoID ids.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, found := s.photos[photoID]; !found {
		return catalog.ErrNoPhoto
	}
	delete(s.photos, photoID)
	return nil
}

func (s *catalogStore) photoReorder(_ context.Context, ordered []ids.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for position, id := range ordered {
		photo, found := s.photos[id]
		if !found {
			return catalog.ErrNoPhoto
		}
		photo.Position = position
		s.photos[id] = photo
	}
	return nil
}

// The four ports, each one a view of the same store.
type (
	seasonsOf struct{ *catalogStore }
	dropsOf   struct{ *catalogStore }
	modelsOf  struct{ *catalogStore }
	photosOf  struct{ *catalogStore }
)

func (s seasonsOf) ByID(ctx context.Context, id ids.ID) (catalog.Season, error) {
	return s.seasonByID(ctx, id)
}
func (s seasonsOf) List(ctx context.Context) ([]catalog.Season, error) { return s.seasonList(ctx) }
func (s seasonsOf) Create(ctx context.Context, in catalog.Season) error {
	return s.seasonCreate(ctx, in)
}
func (s seasonsOf) Update(ctx context.Context, in catalog.Season) error {
	return s.seasonUpdate(ctx, in)
}

func (d dropsOf) ByID(ctx context.Context, id ids.ID) (catalog.Drop, error) {
	return d.dropByID(ctx, id)
}
func (d dropsOf) ListBySeason(ctx context.Context, id ids.ID) ([]catalog.Drop, error) {
	return d.dropListBySeason(ctx, id)
}
func (d dropsOf) Create(ctx context.Context, in catalog.Drop) error { return d.dropCreate(ctx, in) }
func (d dropsOf) Update(ctx context.Context, in catalog.Drop) error { return d.dropUpdate(ctx, in) }

func (m modelsOf) ByID(ctx context.Context, id ids.ID) (catalog.Model, error) {
	return m.modelByID(ctx, id)
}
func (m modelsOf) List(ctx context.Context, f catalog.ModelListParams) ([]catalog.Model, error) {
	return m.modelList(ctx, f)
}
func (m modelsOf) Create(ctx context.Context, in catalog.Model) error { return m.modelCreate(ctx, in) }
func (m modelsOf) Update(ctx context.Context, in catalog.Model) error { return m.modelUpdate(ctx, in) }

func (p photosOf) ByModel(ctx context.Context, id ids.ID) ([]catalog.Photo, error) {
	return p.photoByModel(ctx, id)
}
func (p photosOf) Add(ctx context.Context, in catalog.Photo) error { return p.photoAdd(ctx, in) }
func (p photosOf) Remove(ctx context.Context, id ids.ID) error     { return p.photoRemove(ctx, id) }
func (p photosOf) Reorder(ctx context.Context, o []ids.ID) error   { return p.photoReorder(ctx, o) }

// catalogServiceOn returns the service the test router is wired with, recording
// to the trail the test reads back.
func catalogServiceOn(t *testing.T, trail *recorded) *catalog.Service {
	t.Helper()

	gen := ids.NewGenerator()
	rows := newCatalogStore()
	stores := catalog.Stores{
		Seasons: seasonsOf{rows},
		Drops:   dropsOf{rows},
		Models:  modelsOf{rows},
		Photos:  photosOf{rows},
		Atomic:  directAtomic{},
	}
	return catalog.NewService(stores, gen, audit.NewTrail(trail, testClock(), gen))
}
