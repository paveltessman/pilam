package seed

import (
	"context"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/milestones"
	"github.com/paveltessman/pilam/internal/platform/clock"
	"github.com/paveltessman/pilam/internal/platform/config"
	"github.com/paveltessman/pilam/internal/platform/date"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/media"
)

// fakeUsers is the store the seed writes into. It holds the rows in memory and
// enforces the two rules the seed depends on: an identifier is taken once, and
// an address is taken once. The table refuses them in that order too.
type fakeUsers struct{ rows map[ids.ID]auth.User }

func newFakeUsers() *fakeUsers { return &fakeUsers{rows: make(map[ids.ID]auth.User)} }

func (f *fakeUsers) ByID(_ context.Context, id ids.ID) (auth.User, error) {
	user, found := f.rows[id]
	if !found {
		return auth.User{}, auth.ErrNoUser
	}
	return user, nil
}

func (f *fakeUsers) ByEmail(_ context.Context, email string) (auth.User, error) {
	for _, user := range f.rows {
		if user.Email == email {
			return user, nil
		}
	}
	return auth.User{}, auth.ErrNoUser
}

func (f *fakeUsers) List(_ context.Context) ([]auth.User, error) {
	users := slices.Collect(maps.Values(f.rows))
	slices.SortFunc(users, func(a, b auth.User) int { return strings.Compare(a.Email, b.Email) })
	return users, nil
}

func (f *fakeUsers) Create(_ context.Context, user auth.User) error {
	if _, taken := f.rows[user.ID]; taken {
		return auth.ErrIDTaken
	}
	for _, held := range f.rows {
		if held.Email == user.Email {
			return auth.ErrEmailTaken
		}
	}
	f.rows[user.ID] = user
	return nil
}

func (f *fakeUsers) Update(_ context.Context, user auth.User) error {
	if _, found := f.rows[user.ID]; !found {
		return auth.ErrNoUser
	}
	f.rows[user.ID] = user
	return nil
}

// directAtomic runs the unit of work without a transaction. These tests do not
// reach a database.
type directAtomic struct{}

func (directAtomic) InTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

// fakeRecorder holds the trail the test reads back.
type fakeRecorder struct{ entries []audit.Entry }

func (f *fakeRecorder) Record(_ context.Context, entries ...audit.Entry) error {
	f.entries = append(f.entries, entries...)
	return nil
}

var today = date.Of(2026, 8, 19)

func newTestService(t *testing.T) (Deps, *fakeUsers, *fakeRecorder) {
	deps, users, _, recorder := newTestDeps(t)
	return deps, users, recorder
}

func newTestDeps(t *testing.T) (Deps, *fakeUsers, *catalogStore, *fakeRecorder) {
	t.Helper()

	clk := clock.Fixed(time.Date(2026, 8, 19, 9, 0, 0, 0, time.UTC), time.UTC)
	users := newFakeUsers()
	rows := newCatalogStore()
	calendar := newMilestoneStore(rows)
	recorder := &fakeRecorder{}
	idGen := ids.NewDeterministic(IDSeed)
	trail := audit.NewTrail(recorder, clk, ids.NewDeterministic(IDSeed))

	blobs, err := media.New(config.Media{Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("media.New: %v", err)
	}

	deps := Deps{
		Auth:  auth.NewService(users, directAtomic{}, auth.NewThrottle(clk), idGen, trail),
		Clock: clk,
		Media: blobs,
		Catalog: catalog.NewService(catalog.Stores{
			Seasons: seasonsOf{rows},
			Drops:   dropsOf{rows},
			Models:  modelsOf{rows},
			Photos:  photosOf{rows},
			Atomic:  directAtomic{},
		}, idGen, trail),
		Milestones: milestones.NewService(milestones.Store{
			Types:      typesOf{calendar},
			Templates:  templatesOf{calendar},
			Milestones: calendarOf{calendar},
			Atomic:     directAtomic{},
		}, idGen, clk, trail),
	}
	return deps, users, rows, recorder
}

// run loads the dataset and fails the test when it can't.
func run(t *testing.T, deps Deps, opts Options) Report {
	t.Helper()

	report, err := Run(t.Context(), deps, opts)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	return report
}

// ---------------------------------------------------------------- catalog

// catalogStore is the in-memory stand-in for the four catalog ports. It holds
// the two rules the seed depends on: a season name is taken once, and a drop
// name is taken once inside a season.
type catalogStore struct {
	seasons map[ids.ID]catalog.Season
	drops   map[ids.ID]catalog.Drop
	models  map[ids.ID]catalog.Model
	photos  map[ids.ID]catalog.Photo
}

func newCatalogStore() *catalogStore {
	return &catalogStore{
		seasons: make(map[ids.ID]catalog.Season),
		drops:   make(map[ids.ID]catalog.Drop),
		models:  make(map[ids.ID]catalog.Model),
		photos:  make(map[ids.ID]catalog.Photo),
	}
}

func (c *catalogStore) SeasonByID(_ context.Context, id ids.ID) (catalog.Season, error) {
	season, found := c.seasons[id]
	if !found {
		return catalog.Season{}, catalog.ErrNoSeason
	}
	return season, nil
}

func (c *catalogStore) SeasonList(context.Context) ([]catalog.Season, error) {
	seasons := slices.Collect(maps.Values(c.seasons))
	slices.SortFunc(seasons, func(a, b catalog.Season) int {
		if !date.Equal(a.StartDate, b.StartDate) {
			return a.StartDate.Compare(b.StartDate)
		}
		return strings.Compare(a.Name, b.Name)
	})
	return seasons, nil
}

func (c *catalogStore) SeasonCreate(_ context.Context, season catalog.Season) error {
	for _, held := range c.seasons {
		if strings.EqualFold(held.Name, season.Name) {
			return catalog.ErrSeasonNameTaken
		}
	}
	c.seasons[season.ID] = season
	return nil
}

func (c *catalogStore) SeasonUpdate(_ context.Context, season catalog.Season) error {
	if _, found := c.seasons[season.ID]; !found {
		return catalog.ErrNoSeason
	}
	c.seasons[season.ID] = season
	return nil
}

func (c *catalogStore) DropByID(_ context.Context, id ids.ID) (catalog.Drop, error) {
	drop, found := c.drops[id]
	if !found {
		return catalog.Drop{}, catalog.ErrNoDrop
	}
	return drop, nil
}

func (c *catalogStore) DropListBySeason(_ context.Context, seasonID ids.ID) ([]catalog.Drop, error) {
	var drops []catalog.Drop
	for _, drop := range c.drops {
		if drop.SeasonID == seasonID {
			drops = append(drops, drop)
		}
	}
	slices.SortFunc(drops, func(a, b catalog.Drop) int { return a.TargetDate.Compare(b.TargetDate) })
	return drops, nil
}

func (c *catalogStore) DropCreate(_ context.Context, drop catalog.Drop) error {
	for _, held := range c.drops {
		if held.SeasonID == drop.SeasonID && strings.EqualFold(held.Name, drop.Name) {
			return catalog.ErrDropNameTaken
		}
	}
	c.drops[drop.ID] = drop
	return nil
}

func (c *catalogStore) DropUpdate(_ context.Context, drop catalog.Drop) error {
	if _, found := c.drops[drop.ID]; !found {
		return catalog.ErrNoDrop
	}
	c.drops[drop.ID] = drop
	return nil
}

func (c *catalogStore) ModelByID(_ context.Context, id ids.ID) (catalog.Model, error) {
	model, found := c.models[id]
	if !found {
		return catalog.Model{}, catalog.ErrNoModel
	}
	return model, nil
}

func (c *catalogStore) ModelList(_ context.Context, filter catalog.ModelListParams) ([]catalog.Model, error) {
	models := make([]catalog.Model, 0, len(c.models))
	for _, model := range c.models {
		if filter.DropID != ids.Nil && model.DropID != filter.DropID {
			continue
		}
		if filter.SeasonID != ids.Nil && c.drops[model.DropID].SeasonID != filter.SeasonID {
			continue
		}
		if filter.Active != nil && model.Active != *filter.Active {
			continue
		}
		models = append(models, model)
	}
	slices.SortFunc(models, func(a, b catalog.Model) int { return strings.Compare(a.Article, b.Article) })
	return models, nil
}

func (c *catalogStore) ModelCreate(_ context.Context, model catalog.Model) error {
	if _, taken := c.models[model.ID]; taken {
		return catalog.ErrIDTaken
	}
	c.models[model.ID] = model
	return nil
}

func (c *catalogStore) ModelUpdate(_ context.Context, model catalog.Model) error {
	if _, found := c.models[model.ID]; !found {
		return catalog.ErrNoModel
	}
	c.models[model.ID] = model
	return nil
}

func (c *catalogStore) PhotoByModel(_ context.Context, modelID ids.ID) ([]catalog.Photo, error) {
	var photos []catalog.Photo
	for _, photo := range c.photos {
		if photo.ModelID == modelID {
			photos = append(photos, photo)
		}
	}
	slices.SortFunc(photos, func(a, b catalog.Photo) int { return a.Position - b.Position })
	return photos, nil
}

func (c *catalogStore) PhotoThumbnails(_ context.Context, modelIDs []ids.ID) (map[ids.ID]catalog.Photo, error) {
	covers := make(map[ids.ID]catalog.Photo)
	for _, photo := range c.photos {
		if photo.Position == 0 && slices.Contains(modelIDs, photo.ModelID) {
			covers[photo.ModelID] = photo
		}
	}
	return covers, nil
}

func (c *catalogStore) PhotoAdd(_ context.Context, photo catalog.Photo) error {
	c.photos[photo.ID] = photo
	return nil
}

func (c *catalogStore) PhotoRemove(_ context.Context, photoID ids.ID) error {
	if _, found := c.photos[photoID]; !found {
		return catalog.ErrNoPhoto
	}
	delete(c.photos, photoID)
	return nil
}

func (c *catalogStore) PhotoReorder(_ context.Context, ordered []ids.ID) error {
	for position, id := range ordered {
		photo, found := c.photos[id]
		if !found {
			return catalog.ErrNoPhoto
		}
		photo.Position = position
		c.photos[id] = photo
	}
	return nil
}

// The four catalog ports, each one a view of the same store.
type (
	seasonsOf struct{ *catalogStore }
	dropsOf   struct{ *catalogStore }
	modelsOf  struct{ *catalogStore }
	photosOf  struct{ *catalogStore }
)

func (s seasonsOf) ByID(ctx context.Context, id ids.ID) (catalog.Season, error) {
	return s.SeasonByID(ctx, id)
}
func (s seasonsOf) List(ctx context.Context) ([]catalog.Season, error) { return s.SeasonList(ctx) }
func (s seasonsOf) Create(ctx context.Context, in catalog.Season) error {
	return s.SeasonCreate(ctx, in)
}
func (s seasonsOf) Update(ctx context.Context, in catalog.Season) error {
	return s.SeasonUpdate(ctx, in)
}

func (d dropsOf) ByID(ctx context.Context, id ids.ID) (catalog.Drop, error) {
	return d.DropByID(ctx, id)
}
func (d dropsOf) ListBySeason(ctx context.Context, id ids.ID) ([]catalog.Drop, error) {
	return d.DropListBySeason(ctx, id)
}
func (d dropsOf) Create(ctx context.Context, in catalog.Drop) error { return d.DropCreate(ctx, in) }
func (d dropsOf) Update(ctx context.Context, in catalog.Drop) error { return d.DropUpdate(ctx, in) }

func (m modelsOf) ByID(ctx context.Context, id ids.ID) (catalog.Model, error) {
	return m.ModelByID(ctx, id)
}
func (m modelsOf) List(ctx context.Context, f catalog.ModelListParams) ([]catalog.Model, error) {
	return m.ModelList(ctx, f)
}
func (m modelsOf) Create(ctx context.Context, in catalog.Model) error { return m.ModelCreate(ctx, in) }
func (m modelsOf) Update(ctx context.Context, in catalog.Model) error { return m.ModelUpdate(ctx, in) }

func (p photosOf) ByModel(ctx context.Context, id ids.ID) ([]catalog.Photo, error) {
	return p.PhotoByModel(ctx, id)
}
func (p photosOf) Thumbnails(ctx context.Context, of []ids.ID) (map[ids.ID]catalog.Photo, error) {
	return p.PhotoThumbnails(ctx, of)
}
func (p photosOf) Add(ctx context.Context, in catalog.Photo) error { return p.PhotoAdd(ctx, in) }
func (p photosOf) Remove(ctx context.Context, id ids.ID) error     { return p.PhotoRemove(ctx, id) }
func (p photosOf) Reorder(ctx context.Context, o []ids.ID) error   { return p.PhotoReorder(ctx, o) }

// ---------------------------------------------------------------- milestones

// milestoneStore is the in-memory stand-in for the three milestone ports. It
// holds the rules the seed depends on: a name is taken once, one type appears
// once per template, and a model holds one milestone per type.
//
// It reads the catalog rows the run wrote, because the target date a calendar
// is reckoned from belongs to the drop of the model.
type milestoneStore struct {
	types       map[ids.ID]milestones.Type
	templates   map[ids.ID]milestones.Template
	items       map[ids.ID]milestones.TemplateItem
	rows        map[ids.ID]milestones.Milestone
	catalogRows *catalogStore
}

func newMilestoneStore(rows *catalogStore) *milestoneStore {
	return &milestoneStore{
		types:       make(map[ids.ID]milestones.Type),
		templates:   make(map[ids.ID]milestones.Template),
		items:       make(map[ids.ID]milestones.TemplateItem),
		rows:        make(map[ids.ID]milestones.Milestone),
		catalogRows: rows,
	}
}

func (m *milestoneStore) TypeByID(_ context.Context, id ids.ID) (milestones.Type, error) {
	one, found := m.types[id]
	if !found {
		return milestones.Type{}, milestones.ErrNoType
	}
	return one, nil
}

func (m *milestoneStore) TypeList(context.Context) ([]milestones.Type, error) {
	types := slices.Collect(maps.Values(m.types))
	slices.SortFunc(types, func(a, b milestones.Type) int { return strings.Compare(a.Name, b.Name) })
	return types, nil
}

func (m *milestoneStore) TypeCreate(_ context.Context, in milestones.Type) error {
	if m.typeNameTaken(in) {
		return milestones.ErrNameTaken
	}
	m.types[in.ID] = in
	return nil
}

func (m *milestoneStore) TypeUpdate(_ context.Context, in milestones.Type) error {
	if _, found := m.types[in.ID]; !found {
		return milestones.ErrNoType
	}
	if m.typeNameTaken(in) {
		return milestones.ErrNameTaken
	}
	m.types[in.ID] = in
	return nil
}

// typeNameTaken reports whether another row already holds the short name,
// whatever the case. It is the unique index of the migration.
func (m *milestoneStore) typeNameTaken(in milestones.Type) bool {
	for _, held := range m.types {
		if held.ID != in.ID && strings.EqualFold(held.Name, in.Name) {
			return true
		}
	}
	return false
}

func (m *milestoneStore) TemplateByID(_ context.Context, id ids.ID) (milestones.Template, error) {
	template, found := m.templates[id]
	if !found {
		return milestones.Template{}, milestones.ErrNoTemplate
	}
	return template, nil
}

func (m *milestoneStore) TemplateList(context.Context) ([]milestones.Template, error) {
	templates := slices.Collect(maps.Values(m.templates))
	slices.SortFunc(templates, func(a, b milestones.Template) int { return strings.Compare(a.Name, b.Name) })
	return templates, nil
}

func (m *milestoneStore) TemplateCreate(_ context.Context, in milestones.Template) error {
	if m.templateNameTaken(in) {
		return milestones.ErrNameTaken
	}
	m.templates[in.ID] = in
	return nil
}

func (m *milestoneStore) TemplateUpdate(_ context.Context, in milestones.Template) error {
	if _, found := m.templates[in.ID]; !found {
		return milestones.ErrNoTemplate
	}
	if m.templateNameTaken(in) {
		return milestones.ErrNameTaken
	}
	m.templates[in.ID] = in
	return nil
}

func (m *milestoneStore) TemplateClearDefault(_ context.Context, keep ids.ID) error {
	for id, held := range m.templates {
		if id != keep && held.Default {
			held.Default = false
			m.templates[id] = held
		}
	}
	return nil
}

func (m *milestoneStore) templateNameTaken(in milestones.Template) bool {
	for _, held := range m.templates {
		if held.ID != in.ID && strings.EqualFold(held.Name, in.Name) {
			return true
		}
	}
	return false
}

func (m *milestoneStore) ItemList(_ context.Context, templateID ids.ID) ([]milestones.TemplateItem, error) {
	var items []milestones.TemplateItem
	for _, item := range m.items {
		if item.TemplateID == templateID {
			items = append(items, item)
		}
	}
	slices.SortFunc(items, func(a, b milestones.TemplateItem) int { return a.Position - b.Position })
	return items, nil
}

func (m *milestoneStore) ItemAdd(_ context.Context, in milestones.TemplateItem) error {
	for _, held := range m.items {
		if held.TemplateID == in.TemplateID && held.TypeID == in.TypeID {
			return milestones.ErrTypeInTemplate
		}
	}
	m.items[in.ID] = in
	return nil
}

func (m *milestoneStore) ItemUpdate(_ context.Context, in milestones.TemplateItem) error {
	if _, found := m.items[in.ID]; !found {
		return milestones.ErrNoItem
	}
	m.items[in.ID] = in
	return nil
}

func (m *milestoneStore) ItemRemove(_ context.Context, itemID ids.ID) error {
	if _, found := m.items[itemID]; !found {
		return milestones.ErrNoItem
	}
	delete(m.items, itemID)
	return nil
}

func (m *milestoneStore) ItemReorder(_ context.Context, ordered []ids.ID) error {
	for position, itemID := range ordered {
		item, found := m.items[itemID]
		if !found {
			return milestones.ErrNoItem
		}
		item.Position = position
		m.items[itemID] = item
	}
	return nil
}

func (m *milestoneStore) MilestoneByID(_ context.Context, id ids.ID) (milestones.Milestone, error) {
	one, found := m.rows[id]
	if !found {
		return milestones.Milestone{}, milestones.ErrNoMilestone
	}
	return one, nil
}

func (m *milestoneStore) MilestoneByModel(_ context.Context, modelID ids.ID) ([]milestones.Milestone, error) {
	var calendar []milestones.Milestone
	for _, held := range m.rows {
		if held.ModelID == modelID {
			calendar = append(calendar, held)
		}
	}
	slices.SortFunc(calendar, func(a, b milestones.Milestone) int {
		if !date.Equal(a.Plan, b.Plan) {
			return a.Plan.Compare(b.Plan)
		}
		return strings.Compare(a.ID.String(), b.ID.String())
	})
	return calendar, nil
}

func (m *milestoneStore) MilestoneCreate(_ context.Context, in ...milestones.Milestone) error {
	for _, one := range in {
		for _, held := range m.rows {
			if held.ModelID == one.ModelID && held.TypeID == one.TypeID {
				return milestones.ErrTypeOnModel
			}
		}
		m.rows[one.ID] = one
	}
	return nil
}

func (m *milestoneStore) MilestoneUpdate(_ context.Context, in milestones.Milestone) error {
	if _, found := m.rows[in.ID]; !found {
		return milestones.ErrNoMilestone
	}
	m.rows[in.ID] = in
	return nil
}

func (m *milestoneStore) MilestoneTarget(_ context.Context, modelID ids.ID) (time.Time, error) {
	model, found := m.catalogRows.models[modelID]
	if !found {
		return time.Time{}, milestones.ErrNoModel
	}
	drop, found := m.catalogRows.drops[model.DropID]
	if !found {
		return time.Time{}, milestones.ErrNoModel
	}
	return drop.TargetDate, nil
}

// The three milestone ports, each one a view of the same store.
type (
	typesOf     struct{ *milestoneStore }
	templatesOf struct{ *milestoneStore }
	calendarOf  struct{ *milestoneStore }
)

func (t typesOf) ByID(ctx context.Context, id ids.ID) (milestones.Type, error) {
	return t.TypeByID(ctx, id)
}
func (t typesOf) List(ctx context.Context) ([]milestones.Type, error) { return t.TypeList(ctx) }
func (t typesOf) Create(ctx context.Context, in milestones.Type) error {
	return t.TypeCreate(ctx, in)
}
func (t typesOf) Update(ctx context.Context, in milestones.Type) error {
	return t.TypeUpdate(ctx, in)
}

func (t templatesOf) ByID(ctx context.Context, id ids.ID) (milestones.Template, error) {
	return t.TemplateByID(ctx, id)
}
func (t templatesOf) List(ctx context.Context) ([]milestones.Template, error) {
	return t.TemplateList(ctx)
}
func (t templatesOf) Create(ctx context.Context, in milestones.Template) error {
	return t.TemplateCreate(ctx, in)
}
func (t templatesOf) Update(ctx context.Context, in milestones.Template) error {
	return t.TemplateUpdate(ctx, in)
}
func (t templatesOf) ClearDefault(ctx context.Context, keep ids.ID) error {
	return t.TemplateClearDefault(ctx, keep)
}
func (t templatesOf) Items(ctx context.Context, id ids.ID) ([]milestones.TemplateItem, error) {
	return t.ItemList(ctx, id)
}
func (t templatesOf) AddItem(ctx context.Context, in milestones.TemplateItem) error {
	return t.ItemAdd(ctx, in)
}
func (t templatesOf) UpdateItem(ctx context.Context, in milestones.TemplateItem) error {
	return t.ItemUpdate(ctx, in)
}
func (t templatesOf) RemoveItem(ctx context.Context, id ids.ID) error { return t.ItemRemove(ctx, id) }
func (t templatesOf) ReorderItems(ctx context.Context, o []ids.ID) error {
	return t.ItemReorder(ctx, o)
}

func (c calendarOf) ByID(ctx context.Context, id ids.ID) (milestones.Milestone, error) {
	return c.MilestoneByID(ctx, id)
}
func (c calendarOf) ByModel(ctx context.Context, id ids.ID) ([]milestones.Milestone, error) {
	return c.MilestoneByModel(ctx, id)
}
func (c calendarOf) Create(ctx context.Context, in ...milestones.Milestone) error {
	return c.MilestoneCreate(ctx, in...)
}
func (c calendarOf) Update(ctx context.Context, in milestones.Milestone) error {
	return c.MilestoneUpdate(ctx, in)
}
func (c calendarOf) Target(ctx context.Context, id ids.ID) (time.Time, error) {
	return c.MilestoneTarget(ctx, id)
}
