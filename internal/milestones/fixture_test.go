package milestones

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
	"github.com/paveltessman/pilam/internal/platform/validate"
)

var actorID = ids.MustParse("01912345-6789-7abc-def0-123456789abc")

// testNow is the instant the service reads the current business day from.
var testNow = time.Date(2026, 8, 19, 9, 0, 0, 0, time.UTC)

// ---------------------------------------------------------------- fakes

// store is the in-memory stand-in for the ports of the package.
type store struct {
	types      map[ids.ID]Type
	templates  map[ids.ID]Template
	items      map[ids.ID]TemplateItem
	milestones map[ids.ID]Milestone

	// targets is the target date of the drop of every model the test wrote. A
	// model that is not on it is a model that is not there.
	targets map[ids.ID]time.Time

	// list and counts are what the two reads of the milestone list answer. The
	// statement behind them narrows the rows, and asked is what it was asked
	// for, so a test reads the filter the service handed over.
	list   []ListRow
	counts []NoCalendar
	asked  ListParams

	failWith error
}

func newStore() *store {
	return &store{
		types:      make(map[ids.ID]Type),
		templates:  make(map[ids.ID]Template),
		items:      make(map[ids.ID]TemplateItem),
		milestones: make(map[ids.ID]Milestone),
		targets:    make(map[ids.ID]time.Time),
	}
}

func (s *store) TypeByID(_ context.Context, id ids.ID) (Type, error) {
	if s.failWith != nil {
		return Type{}, s.failWith
	}
	milestoneType, found := s.types[id]
	if !found {
		return Type{}, ErrNoType
	}
	return milestoneType, nil
}

func (s *store) TypeList(context.Context) ([]Type, error) {
	if s.failWith != nil {
		return nil, s.failWith
	}
	types := slices.Collect(maps.Values(s.types))
	slices.SortFunc(types, func(a, b Type) int { return strings.Compare(a.Name, b.Name) })
	return types, nil
}

func (s *store) TypeCreate(_ context.Context, in Type) error {
	if s.failWith != nil {
		return s.failWith
	}
	if s.taken(in) {
		return ErrNameTaken
	}
	s.types[in.ID] = in
	return nil
}

func (s *store) TypeUpdate(_ context.Context, in Type) error {
	if s.failWith != nil {
		return s.failWith
	}
	if _, found := s.types[in.ID]; !found {
		return ErrNoType
	}
	if s.taken(in) {
		return ErrNameTaken
	}
	s.types[in.ID] = in
	return nil
}

// taken reports whether another row already holds the name, whatever the
// case. It is the unique index of the migration.
func (s *store) taken(in Type) bool {
	for _, held := range s.types {
		if held.ID != in.ID && strings.EqualFold(held.Name, in.Name) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- templates

func (s *store) TemplateByID(_ context.Context, id ids.ID) (Template, error) {
	if s.failWith != nil {
		return Template{}, s.failWith
	}
	template, found := s.templates[id]
	if !found {
		return Template{}, ErrNoTemplate
	}
	return template, nil
}

func (s *store) TemplateList(context.Context) ([]Template, error) {
	if s.failWith != nil {
		return nil, s.failWith
	}
	templates := slices.Collect(maps.Values(s.templates))
	slices.SortFunc(templates, func(a, b Template) int { return strings.Compare(a.Name, b.Name) })
	return templates, nil
}

func (s *store) TemplateCreate(_ context.Context, in Template) error {
	if s.failWith != nil {
		return s.failWith
	}
	if s.templateTaken(in) {
		return ErrNameTaken
	}
	s.templates[in.ID] = in
	return nil
}

func (s *store) TemplateUpdate(_ context.Context, in Template) error {
	if s.failWith != nil {
		return s.failWith
	}
	if _, found := s.templates[in.ID]; !found {
		return ErrNoTemplate
	}
	if s.templateTaken(in) {
		return ErrNameTaken
	}
	s.templates[in.ID] = in
	return nil
}

func (s *store) TemplateClearDefault(_ context.Context, keep ids.ID) error {
	if s.failWith != nil {
		return s.failWith
	}
	for id, held := range s.templates {
		if id != keep && held.Default {
			held.Default = false
			s.templates[id] = held
		}
	}
	return nil
}

func (s *store) templateTaken(in Template) bool {
	for _, held := range s.templates {
		if held.ID != in.ID && strings.EqualFold(held.Name, in.Name) {
			return true
		}
	}
	return false
}

func (s *store) ItemList(_ context.Context, templateID ids.ID) ([]TemplateItem, error) {
	if s.failWith != nil {
		return nil, s.failWith
	}
	var items []TemplateItem
	for _, item := range s.items {
		if item.TemplateID == templateID {
			items = append(items, item)
		}
	}
	slices.SortFunc(items, func(a, b TemplateItem) int { return a.Position - b.Position })
	return items, nil
}

func (s *store) ItemAdd(_ context.Context, in TemplateItem) error {
	if s.failWith != nil {
		return s.failWith
	}
	for _, held := range s.items {
		if held.TemplateID == in.TemplateID && held.TypeID == in.TypeID {
			return ErrTypeInTemplate
		}
	}
	s.items[in.ID] = in
	return nil
}

func (s *store) ItemUpdate(_ context.Context, in TemplateItem) error {
	if s.failWith != nil {
		return s.failWith
	}
	if _, found := s.items[in.ID]; !found {
		return ErrNoItem
	}
	s.items[in.ID] = in
	return nil
}

func (s *store) ItemRemove(_ context.Context, itemID ids.ID) error {
	if s.failWith != nil {
		return s.failWith
	}
	if _, found := s.items[itemID]; !found {
		return ErrNoItem
	}
	delete(s.items, itemID)
	return nil
}

func (s *store) ItemReorder(_ context.Context, ordered []ids.ID) error {
	if s.failWith != nil {
		return s.failWith
	}
	for position, itemID := range ordered {
		item, found := s.items[itemID]
		if !found {
			return ErrNoItem
		}
		item.Position = position
		s.items[itemID] = item
	}
	return nil
}

// ---------------------------------------------------------------- calendar

func (s *store) MilestoneByID(_ context.Context, id ids.ID) (Milestone, error) {
	if s.failWith != nil {
		return Milestone{}, s.failWith
	}
	milestone, found := s.milestones[id]
	if !found {
		return Milestone{}, ErrNoMilestone
	}
	return milestone, nil
}

func (s *store) MilestoneByModel(_ context.Context, modelID ids.ID) ([]Milestone, error) {
	if s.failWith != nil {
		return nil, s.failWith
	}
	var calendar []Milestone
	for _, held := range s.milestones {
		if held.ModelID == modelID {
			calendar = append(calendar, held)
		}
	}
	slices.SortFunc(calendar, func(a, b Milestone) int {
		if !date.Equal(a.Plan, b.Plan) {
			return a.Plan.Compare(b.Plan)
		}
		return strings.Compare(a.ID.String(), b.ID.String())
	})
	return calendar, nil
}

func (s *store) MilestoneCreate(_ context.Context, in ...Milestone) error {
	if s.failWith != nil {
		return s.failWith
	}
	for _, one := range in {
		for _, held := range s.milestones {
			if held.ModelID == one.ModelID && held.TypeID == one.TypeID {
				return ErrTypeOnModel
			}
		}
		s.milestones[one.ID] = one
	}
	return nil
}

func (s *store) MilestoneUpdate(_ context.Context, in Milestone) error {
	if s.failWith != nil {
		return s.failWith
	}
	if _, found := s.milestones[in.ID]; !found {
		return ErrNoMilestone
	}
	s.milestones[in.ID] = in
	return nil
}

func (s *store) MilestoneTarget(_ context.Context, modelID ids.ID) (time.Time, error) {
	if s.failWith != nil {
		return time.Time{}, s.failWith
	}
	target, found := s.targets[modelID]
	if !found {
		return time.Time{}, ErrNoModel
	}
	return target, nil
}

func (s *store) MilestoneList(_ context.Context, filter ListParams) ([]ListRow, error) {
	if s.failWith != nil {
		return nil, s.failWith
	}
	s.asked = filter
	return s.list, nil
}

func (s *store) MilestoneWithoutCalendar(_ context.Context, seasonID, dropID ids.ID) ([]NoCalendar, error) {
	if s.failWith != nil {
		return nil, s.failWith
	}
	s.asked = ListParams{SeasonID: seasonID, DropID: dropID}
	return s.counts, nil
}

// The ports, each one a view of the same store.
type typesOf struct{ *store }

func (t typesOf) ByID(ctx context.Context, id ids.ID) (Type, error) { return t.TypeByID(ctx, id) }
func (t typesOf) List(ctx context.Context) ([]Type, error)          { return t.TypeList(ctx) }
func (t typesOf) Create(ctx context.Context, in Type) error         { return t.TypeCreate(ctx, in) }
func (t typesOf) Update(ctx context.Context, in Type) error         { return t.TypeUpdate(ctx, in) }

type templatesOf struct{ *store }

func (t templatesOf) ByID(ctx context.Context, id ids.ID) (Template, error) {
	return t.TemplateByID(ctx, id)
}
func (t templatesOf) List(ctx context.Context) ([]Template, error) { return t.TemplateList(ctx) }
func (t templatesOf) Create(ctx context.Context, in Template) error {
	return t.TemplateCreate(ctx, in)
}
func (t templatesOf) Update(ctx context.Context, in Template) error {
	return t.TemplateUpdate(ctx, in)
}
func (t templatesOf) ClearDefault(ctx context.Context, keep ids.ID) error {
	return t.TemplateClearDefault(ctx, keep)
}
func (t templatesOf) Items(ctx context.Context, templateID ids.ID) ([]TemplateItem, error) {
	return t.ItemList(ctx, templateID)
}
func (t templatesOf) AddItem(ctx context.Context, in TemplateItem) error {
	return t.ItemAdd(ctx, in)
}
func (t templatesOf) UpdateItem(ctx context.Context, in TemplateItem) error {
	return t.ItemUpdate(ctx, in)
}
func (t templatesOf) RemoveItem(ctx context.Context, itemID ids.ID) error {
	return t.ItemRemove(ctx, itemID)
}
func (t templatesOf) ReorderItems(ctx context.Context, ordered []ids.ID) error {
	return t.ItemReorder(ctx, ordered)
}

type milestonesOf struct{ *store }

func (m milestonesOf) ByID(ctx context.Context, id ids.ID) (Milestone, error) {
	return m.MilestoneByID(ctx, id)
}
func (m milestonesOf) ByModel(ctx context.Context, modelID ids.ID) ([]Milestone, error) {
	return m.MilestoneByModel(ctx, modelID)
}
func (m milestonesOf) Create(ctx context.Context, in ...Milestone) error {
	return m.MilestoneCreate(ctx, in...)
}
func (m milestonesOf) Update(ctx context.Context, in Milestone) error {
	return m.MilestoneUpdate(ctx, in)
}
func (m milestonesOf) Target(ctx context.Context, modelID ids.ID) (time.Time, error) {
	return m.MilestoneTarget(ctx, modelID)
}
func (m milestonesOf) List(ctx context.Context, filter ListParams) ([]ListRow, error) {
	return m.MilestoneList(ctx, filter)
}
func (m milestonesOf) WithoutCalendar(ctx context.Context, seasonID, dropID ids.ID) ([]NoCalendar, error) {
	return m.MilestoneWithoutCalendar(ctx, seasonID, dropID)
}

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

	clk := clock.Fixed(testNow, time.UTC)
	recorder := &fakeRecorder{}
	trail := audit.NewTrail(recorder, clk, ids.NewDeterministic(100))

	rows := newStore()
	stores := Store{
		Types:      typesOf{rows},
		Templates:  templatesOf{rows},
		Milestones: milestonesOf{rows},
		Atomic:     directAtomic{},
	}
	return NewService(stores, ids.NewDeterministic(1), clk, trail), rows, recorder
}

// modelIDs names the models a test writes. The catalog owns the row, so the
// identifier comes from outside the package under test.
var modelIDs = ids.NewDeterministic(900)

// aModel writes the target date of the drop of a model and returns the model.
// The catalog owns the row: the calendar only knows the date.
func aModel(rows *store, target string) ids.ID {
	modelID := modelIDs.New()
	rows.targets[modelID] = date.MustParse(target)
	return modelID
}

// signedIn is a context carrying the actor every write is recorded against.
func signedIn(t *testing.T) context.Context {
	t.Helper()
	return auth.NewContext(t.Context(), auth.Identity{UserID: actorID, Role: auth.RootRole})
}

// milestoneType writes one type and returns it.
func milestoneType(t *testing.T, svc *Service, ctx context.Context, short, description string) Type {
	t.Helper()

	created, err := svc.CreateType(ctx, TypeCreateParams{Name: short, Description: description})
	if err != nil {
		t.Fatalf("CreateType %q: %v", short, err)
	}
	return created
}

// milestoneTemplate writes one template and returns it.
func milestoneTemplate(t *testing.T, svc *Service, ctx context.Context, name, description string) Template {
	t.Helper()

	created, err := svc.CreateTemplate(ctx, TemplateCreateParams{Name: name, Description: description})
	if err != nil {
		t.Fatalf("CreateTemplate %q: %v", name, err)
	}
	return created
}

// templateItem appends one step to a template and returns it.
func templateItem(t *testing.T, svc *Service, ctx context.Context, templateID, typeID ids.ID, offset int) TemplateItem {
	t.Helper()

	created, err := svc.AddTemplateItem(ctx, templateID, typeID, offset)
	if err != nil {
		t.Fatalf("AddTemplateItem %d: %v", offset, err)
	}
	return created
}

// offsets is the offset of every item of a list, in order.
func offsets(items []TemplateItem) []int {
	out := make([]int, len(items))
	for i, item := range items {
		out[i] = item.Offset
	}
	return out
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
