package catalog

import (
	"context"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/platform/clock"
	"github.com/paveltessman/pilam/internal/platform/date"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/media"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

var actorID = ids.MustParse("01912345-6789-7abc-def0-123456789abc")

// The media keys the photo tests store.
var (
	firstKey  = mediaKey("a")
	secondKey = mediaKey("b")
	thirdKey  = mediaKey("c")
)

func mediaKey(digit string) media.Key {
	return media.Key("images/aa/bb/" + strings.Repeat(digit, 64) + ".jpg")
}

// ---------------------------------------------------------------- fakes

// store is the in-memory stand-in for the four ports.
type store struct {
	seasons  map[ids.ID]Season
	drops    map[ids.ID]Drop
	models   map[ids.ID]Model
	photos   map[ids.ID]Photo
	failWith error
}

func newStore() *store {
	return &store{
		seasons: make(map[ids.ID]Season),
		drops:   make(map[ids.ID]Drop),
		models:  make(map[ids.ID]Model),
		photos:  make(map[ids.ID]Photo),
	}
}

func (s *store) SeasonByID(_ context.Context, id ids.ID) (Season, error) {
	if s.failWith != nil {
		return Season{}, s.failWith
	}
	season, found := s.seasons[id]
	if !found {
		return Season{}, ErrNoSeason
	}
	return season, nil
}

func (s *store) SeasonList(context.Context) ([]Season, error) {
	if s.failWith != nil {
		return nil, s.failWith
	}
	seasons := slices.Collect(maps.Values(s.seasons))
	slices.SortFunc(seasons, func(a, b Season) int {
		if !date.Equal(a.StartDate, b.StartDate) {
			return a.StartDate.Compare(b.StartDate)
		}
		return strings.Compare(a.Name, b.Name)
	})
	return seasons, nil
}

func (s *store) SeasonCreate(_ context.Context, season Season) error {
	if s.failWith != nil {
		return s.failWith
	}
	for _, held := range s.seasons {
		if strings.EqualFold(held.Name, season.Name) {
			return ErrSeasonNameTaken
		}
	}
	s.seasons[season.ID] = season
	return nil
}

func (s *store) SeasonUpdate(_ context.Context, season Season) error {
	if s.failWith != nil {
		return s.failWith
	}
	if _, found := s.seasons[season.ID]; !found {
		return ErrNoSeason
	}
	s.seasons[season.ID] = season
	return nil
}

func (s *store) DropByID(_ context.Context, id ids.ID) (Drop, error) {
	if s.failWith != nil {
		return Drop{}, s.failWith
	}
	drop, found := s.drops[id]
	if !found {
		return Drop{}, ErrNoDrop
	}
	return drop, nil
}

func (s *store) DropListBySeason(_ context.Context, seasonID ids.ID) ([]Drop, error) {
	if s.failWith != nil {
		return nil, s.failWith
	}
	var drops []Drop
	for _, drop := range s.drops {
		if drop.SeasonID == seasonID {
			drops = append(drops, drop)
		}
	}
	slices.SortFunc(drops, func(a, b Drop) int { return a.TargetDate.Compare(b.TargetDate) })
	return drops, nil
}

func (s *store) DropCreate(_ context.Context, drop Drop) error {
	if s.failWith != nil {
		return s.failWith
	}
	for _, held := range s.drops {
		if held.SeasonID == drop.SeasonID && strings.EqualFold(held.Name, drop.Name) {
			return ErrDropNameTaken
		}
	}
	s.drops[drop.ID] = drop
	return nil
}

func (s *store) DropUpdate(_ context.Context, drop Drop) error {
	if s.failWith != nil {
		return s.failWith
	}
	if _, found := s.drops[drop.ID]; !found {
		return ErrNoDrop
	}
	s.drops[drop.ID] = drop
	return nil
}

func (s *store) ModelByID(_ context.Context, id ids.ID) (Model, error) {
	if s.failWith != nil {
		return Model{}, s.failWith
	}
	model, found := s.models[id]
	if !found {
		return Model{}, ErrNoModel
	}
	return model, nil
}

func (s *store) ModelList(_ context.Context, filter ModelListParams) ([]Model, error) {
	if s.failWith != nil {
		return nil, s.failWith
	}
	models := make([]Model, 0, len(s.models))
	for _, model := range s.models {
		if filter.DropID != ids.Nil && model.DropID != filter.DropID {
			continue
		}
		if filter.SeasonID != ids.Nil && s.drops[model.DropID].SeasonID != filter.SeasonID {
			continue
		}
		if filter.Active != nil && model.Active != *filter.Active {
			continue
		}
		models = append(models, model)
	}
	slices.SortFunc(models, func(a, b Model) int { return strings.Compare(a.Article, b.Article) })
	return models, nil
}

func (s *store) ModelCreate(_ context.Context, model Model) error {
	if s.failWith != nil {
		return s.failWith
	}
	s.models[model.ID] = model
	return nil
}

func (s *store) ModelUpdate(_ context.Context, model Model) error {
	if s.failWith != nil {
		return s.failWith
	}
	if _, found := s.models[model.ID]; !found {
		return ErrNoModel
	}
	s.models[model.ID] = model
	return nil
}

func (s *store) PhotoByModel(_ context.Context, modelID ids.ID) ([]Photo, error) {
	if s.failWith != nil {
		return nil, s.failWith
	}
	var photos []Photo
	for _, photo := range s.photos {
		if photo.ModelID == modelID {
			photos = append(photos, photo)
		}
	}
	slices.SortFunc(photos, func(a, b Photo) int { return a.Position - b.Position })
	return photos, nil
}

func (s *store) PhotoAdd(_ context.Context, photo Photo) error {
	if s.failWith != nil {
		return s.failWith
	}
	s.photos[photo.ID] = photo
	return nil
}

func (s *store) PhotoRemove(_ context.Context, photoID ids.ID) error {
	if s.failWith != nil {
		return s.failWith
	}
	if _, found := s.photos[photoID]; !found {
		return ErrNoPhoto
	}
	delete(s.photos, photoID)
	return nil
}

func (s *store) PhotoReorder(_ context.Context, ordered []ids.ID) error {
	if s.failWith != nil {
		return s.failWith
	}
	for position, id := range ordered {
		photo, found := s.photos[id]
		if !found {
			return ErrNoPhoto
		}
		photo.Position = position
		s.photos[id] = photo
	}
	return nil
}

// The four ports, each one a view of the same store.
type (
	seasonsOf struct{ *store }
	dropsOf   struct{ *store }
	modelsOf  struct{ *store }
	photosOf  struct{ *store }
)

func (s seasonsOf) ByID(ctx context.Context, id ids.ID) (Season, error) { return s.SeasonByID(ctx, id) }
func (s seasonsOf) List(ctx context.Context) ([]Season, error)          { return s.SeasonList(ctx) }
func (s seasonsOf) Create(ctx context.Context, in Season) error         { return s.SeasonCreate(ctx, in) }
func (s seasonsOf) Update(ctx context.Context, in Season) error         { return s.SeasonUpdate(ctx, in) }

func (d dropsOf) ByID(ctx context.Context, id ids.ID) (Drop, error) { return d.DropByID(ctx, id) }
func (d dropsOf) ListBySeason(ctx context.Context, id ids.ID) ([]Drop, error) {
	return d.DropListBySeason(ctx, id)
}
func (d dropsOf) Create(ctx context.Context, in Drop) error { return d.DropCreate(ctx, in) }
func (d dropsOf) Update(ctx context.Context, in Drop) error { return d.DropUpdate(ctx, in) }

func (m modelsOf) ByID(ctx context.Context, id ids.ID) (Model, error) { return m.ModelByID(ctx, id) }
func (m modelsOf) List(ctx context.Context, f ModelListParams) ([]Model, error) {
	return m.ModelList(ctx, f)
}
func (m modelsOf) Create(ctx context.Context, in Model) error { return m.ModelCreate(ctx, in) }
func (m modelsOf) Update(ctx context.Context, in Model) error { return m.ModelUpdate(ctx, in) }

func (p photosOf) ByModel(ctx context.Context, id ids.ID) ([]Photo, error) {
	return p.PhotoByModel(ctx, id)
}
func (p photosOf) Add(ctx context.Context, in Photo) error       { return p.PhotoAdd(ctx, in) }
func (p photosOf) Remove(ctx context.Context, id ids.ID) error   { return p.PhotoRemove(ctx, id) }
func (p photosOf) Reorder(ctx context.Context, o []ids.ID) error { return p.PhotoReorder(ctx, o) }

// directAtomic runs the unit of work without a transaction. The domain tests do
// not reach a database.
type directAtomic struct{}

func (directAtomic) InTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

// fakeRecorder holds the trail the test reads back.
type fakeRecorder struct{ entries []audit.Entry }

func (f *fakeRecorder) Record(_ context.Context, entries ...audit.Entry) error {
	f.entries = append(f.entries, entries...)
	return nil
}

// only returns the single entry the trail holds, and fails the test otherwise.
func (f *fakeRecorder) only(t *testing.T) audit.Entry {
	t.Helper()
	if len(f.entries) != 1 {
		t.Fatalf("The trail holds %d entries, want 1: %+v", len(f.entries), f.entries)
	}
	return f.entries[0]
}

// actions is what the trail recorded, in order.
func (f *fakeRecorder) actions() []string {
	out := make([]string, len(f.entries))
	for i, entry := range f.entries {
		out[i] = entry.Action
	}
	return out
}

// ---------------------------------------------------------------- fixtures

// newTestService returns the service, the store behind it, and the trail it
// records to.
func newTestService(t *testing.T) (*Service, *store, *fakeRecorder) {
	t.Helper()

	clk := clock.Fixed(time.Date(2026, 8, 19, 9, 0, 0, 0, time.UTC), time.UTC)
	recorder := &fakeRecorder{}
	trail := audit.NewTrail(recorder, clk, ids.NewDeterministic(100))

	rows := newStore()
	stores := Stores{
		Seasons: seasonsOf{rows},
		Drops:   dropsOf{rows},
		Models:  modelsOf{rows},
		Photos:  photosOf{rows},
		Atomic:  directAtomic{},
	}
	return NewService(stores, ids.NewDeterministic(1), trail), rows, recorder
}

// signedIn is a context carrying the actor every write is recorded against.
func signedIn(t *testing.T) context.Context {
	t.Helper()
	return auth.NewContext(t.Context(), auth.Identity{UserID: actorID, Role: auth.MemberRole})
}

// spine writes one season, one drop inside it, and one model inside the drop.
func spine(t *testing.T, svc *Service, ctx context.Context) (Season, Drop, Model) {
	t.Helper()

	season, err := svc.CreateSeason(ctx, SeasonCreateParams{Name: "S1", StartDate: date.MustParse("2026-11-01")})
	if err != nil {
		t.Fatalf("CreateSeason: %v", err)
	}
	drop, err := svc.CreateDrop(ctx, DropCreateParams{
		SeasonID:   season.ID,
		Name:       "Drop 1",
		TargetDate: date.MustParse("2027-02-15"),
	})
	if err != nil {
		t.Fatalf("CreateDrop: %v", err)
	}
	model, err := svc.CreateModel(ctx, ModelCreateParams{DropID: drop.ID, Article: "A-100"})
	if err != nil {
		t.Fatalf("CreateModel: %v", err)
	}
	return season, drop, model
}

// rejects reports the code the error carries for field, and fails otherwise.
func rejects(t *testing.T, err error, field string, code validate.Code) {
	t.Helper()

	errs, ok := validate.From(err)
	if !ok {
		t.Fatalf("error = %v, want a field rejection", err)
	}
	got, found := errs.Get(field)
	if !found {
		t.Fatalf("error = %v, want a rejection of %q", err, field)
	}
	if got.Code != code {
		t.Errorf("%q code = %q, want %q", field, got.Code, code)
	}
}

// keysOf is the media keys of a strip, in order.
func keysOf(photos []Photo) []media.Key {
	out := make([]media.Key, len(photos))
	for i, photo := range photos {
		out[i] = photo.MediaKey
	}
	return out
}
