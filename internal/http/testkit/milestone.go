package testkit

import (
	"context"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/milestones"
	"github.com/paveltessman/pilam/internal/platform/ids"
)

// milestoneTypeStore is the in-memory stand-in for the milestone type port. It
// holds the two rules the screens depend on: the order of the list, and the
// duplicate short name each write refuses.
type milestoneTypeStore struct {
	mu    sync.Mutex
	types map[ids.ID]milestones.Type
}

func newMilestoneTypeStore() *milestoneTypeStore {
	return &milestoneTypeStore{types: make(map[ids.ID]milestones.Type)}
}

func (s *milestoneTypeStore) ByID(_ context.Context, id ids.ID) (milestones.Type, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	milestoneType, found := s.types[id]
	if !found {
		return milestones.Type{}, milestones.ErrNoType
	}
	return milestoneType, nil
}

func (s *milestoneTypeStore) List(context.Context) ([]milestones.Type, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	types := slices.Collect(maps.Values(s.types))
	slices.SortFunc(types, func(a, b milestones.Type) int { return strings.Compare(a.Name, b.Name) })
	return types, nil
}

func (s *milestoneTypeStore) Create(_ context.Context, in milestones.Type) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, held := range s.types {
		if strings.EqualFold(held.Name, in.Name) {
			return milestones.ErrNameTaken
		}
	}
	s.types[in.ID] = in
	return nil
}

func (s *milestoneTypeStore) Update(_ context.Context, in milestones.Type) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, found := s.types[in.ID]; !found {
		return milestones.ErrNoType
	}
	for _, held := range s.types {
		if held.ID != in.ID && strings.EqualFold(held.Name, in.Name) {
			return milestones.ErrNameTaken
		}
	}
	s.types[in.ID] = in
	return nil
}

// milestoneTemplateStore is the in-memory stand-in for the template port. It
// holds the rules the screens depend on: the order of the list, the duplicate
// name, the one default, and the one type per template.
type milestoneTemplateStore struct {
	mu        sync.Mutex
	templates map[ids.ID]milestones.Template
	items     map[ids.ID]milestones.TemplateItem
}

func newMilestoneTemplateStore() *milestoneTemplateStore {
	return &milestoneTemplateStore{
		templates: make(map[ids.ID]milestones.Template),
		items:     make(map[ids.ID]milestones.TemplateItem),
	}
}

func (s *milestoneTemplateStore) ByID(_ context.Context, id ids.ID) (milestones.Template, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	template, found := s.templates[id]
	if !found {
		return milestones.Template{}, milestones.ErrNoTemplate
	}
	return template, nil
}

func (s *milestoneTemplateStore) List(context.Context) ([]milestones.Template, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	templates := slices.Collect(maps.Values(s.templates))
	slices.SortFunc(templates, func(a, b milestones.Template) int { return strings.Compare(a.Name, b.Name) })
	return templates, nil
}

func (s *milestoneTemplateStore) Create(_ context.Context, in milestones.Template) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.nameTaken(in) {
		return milestones.ErrNameTaken
	}
	s.templates[in.ID] = in
	return nil
}

func (s *milestoneTemplateStore) Update(_ context.Context, in milestones.Template) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, found := s.templates[in.ID]; !found {
		return milestones.ErrNoTemplate
	}
	if s.nameTaken(in) {
		return milestones.ErrNameTaken
	}
	s.templates[in.ID] = in
	return nil
}

func (s *milestoneTemplateStore) ClearDefault(_ context.Context, keep ids.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, held := range s.templates {
		if id != keep && held.Default {
			held.Default = false
			s.templates[id] = held
		}
	}
	return nil
}

// nameTaken reports whether another row already holds the name, whatever the
// case. It is the unique index of the migration.
func (s *milestoneTemplateStore) nameTaken(in milestones.Template) bool {
	for _, held := range s.templates {
		if held.ID != in.ID && strings.EqualFold(held.Name, in.Name) {
			return true
		}
	}
	return false
}

func (s *milestoneTemplateStore) Items(_ context.Context, templateID ids.ID) ([]milestones.TemplateItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var items []milestones.TemplateItem
	for _, item := range s.items {
		if item.TemplateID == templateID {
			items = append(items, item)
		}
	}
	slices.SortFunc(items, func(a, b milestones.TemplateItem) int { return a.Position - b.Position })
	return items, nil
}

func (s *milestoneTemplateStore) AddItem(_ context.Context, in milestones.TemplateItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, held := range s.items {
		if held.TemplateID == in.TemplateID && held.TypeID == in.TypeID {
			return milestones.ErrTypeInTemplate
		}
	}
	s.items[in.ID] = in
	return nil
}

func (s *milestoneTemplateStore) UpdateItem(_ context.Context, in milestones.TemplateItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, found := s.items[in.ID]; !found {
		return milestones.ErrNoItem
	}
	s.items[in.ID] = in
	return nil
}

func (s *milestoneTemplateStore) RemoveItem(_ context.Context, itemID ids.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, found := s.items[itemID]; !found {
		return milestones.ErrNoItem
	}
	delete(s.items, itemID)
	return nil
}

func (s *milestoneTemplateStore) ReorderItems(_ context.Context, ordered []ids.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for position, itemID := range ordered {
		item, found := s.items[itemID]
		if !found {
			return milestones.ErrNoItem
		}
		item.Position = position
		s.items[itemID] = item
	}
	return nil
}

// milestoneCalendarStore is the in-memory stand-in for the calendar port. It
// holds the one rule the screens depend on: a model holds one milestone per
// type.
//
// It reads the catalog rows the test wrote, because the target date every
// calendar is reckoned from belongs to the drop of the model.
type milestoneCalendarStore struct {
	mu      sync.Mutex
	rows    map[ids.ID]milestones.Milestone
	catalog *catalogStore
}

func newMilestoneCalendarStore(rows *catalogStore) *milestoneCalendarStore {
	return &milestoneCalendarStore{rows: make(map[ids.ID]milestones.Milestone), catalog: rows}
}

func (s *milestoneCalendarStore) ByID(_ context.Context, id ids.ID) (milestones.Milestone, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	milestone, found := s.rows[id]
	if !found {
		return milestones.Milestone{}, milestones.ErrNoMilestone
	}
	return milestone, nil
}

func (s *milestoneCalendarStore) ByModel(_ context.Context, modelID ids.ID) ([]milestones.Milestone, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var calendar []milestones.Milestone
	for _, held := range s.rows {
		if held.ModelID == modelID {
			calendar = append(calendar, held)
		}
	}
	slices.SortFunc(calendar, func(a, b milestones.Milestone) int {
		if !a.Plan.Equal(b.Plan) {
			return a.Plan.Compare(b.Plan)
		}
		return strings.Compare(a.ID.String(), b.ID.String())
	})
	return calendar, nil
}

func (s *milestoneCalendarStore) Create(_ context.Context, in ...milestones.Milestone) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, one := range in {
		for _, held := range s.rows {
			if held.ModelID == one.ModelID && held.TypeID == one.TypeID {
				return milestones.ErrTypeOnModel
			}
		}
		s.rows[one.ID] = one
	}
	return nil
}

func (s *milestoneCalendarStore) Update(_ context.Context, in milestones.Milestone) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, found := s.rows[in.ID]; !found {
		return milestones.ErrNoMilestone
	}
	s.rows[in.ID] = in
	return nil
}

func (s *milestoneCalendarStore) Target(ctx context.Context, modelID ids.ID) (time.Time, error) {
	model, err := s.catalog.modelByID(ctx, modelID)
	if err != nil {
		return time.Time{}, milestones.ErrNoModel
	}
	drop, err := s.catalog.dropByID(ctx, model.DropID)
	if err != nil {
		return time.Time{}, milestones.ErrNoModel
	}
	return drop.TargetDate, nil
}

// milestoneServiceOn returns the service the test router is wired with,
// recording to the trail the test reads back. It reads the catalog rows given,
// which is the same store the catalog service writes.
func milestoneServiceOn(t *testing.T, trail *Trail, rows *catalogStore) *milestones.Service {
	t.Helper()

	gen := ids.NewGenerator()
	store := milestones.Store{
		Types:      newMilestoneTypeStore(),
		Templates:  newMilestoneTemplateStore(),
		Milestones: newMilestoneCalendarStore(rows),
		Atomic:     directAtomic{},
	}
	return milestones.NewService(store, gen, Clock(), audit.NewTrail(trail, Clock(), gen))
}
