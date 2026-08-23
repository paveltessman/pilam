package http

import (
	"context"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/milestone"
	"github.com/paveltessman/pilam/internal/platform/ids"
)

// milestoneTypeStore is the in-memory stand-in for the milestone type port. It
// holds the two rules the screens depend on: the order of the list, and the
// duplicate short name each write refuses.
type milestoneTypeStore struct {
	mu    sync.Mutex
	types map[ids.ID]milestone.Type
}

func newMilestoneTypeStore() *milestoneTypeStore {
	return &milestoneTypeStore{types: make(map[ids.ID]milestone.Type)}
}

func (s *milestoneTypeStore) ByID(_ context.Context, id ids.ID) (milestone.Type, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	milestoneType, found := s.types[id]
	if !found {
		return milestone.Type{}, milestone.ErrNoType
	}
	return milestoneType, nil
}

func (s *milestoneTypeStore) List(context.Context) ([]milestone.Type, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	types := slices.Collect(maps.Values(s.types))
	slices.SortFunc(types, func(a, b milestone.Type) int { return strings.Compare(a.Name, b.Name) })
	return types, nil
}

func (s *milestoneTypeStore) Create(_ context.Context, in milestone.Type) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, held := range s.types {
		if strings.EqualFold(held.Name, in.Name) {
			return milestone.ErrNameTaken
		}
	}
	s.types[in.ID] = in
	return nil
}

func (s *milestoneTypeStore) Update(_ context.Context, in milestone.Type) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, found := s.types[in.ID]; !found {
		return milestone.ErrNoType
	}
	for _, held := range s.types {
		if held.ID != in.ID && strings.EqualFold(held.Name, in.Name) {
			return milestone.ErrNameTaken
		}
	}
	s.types[in.ID] = in
	return nil
}

// milestoneServiceOn returns the service the test router is wired with,
// recording to the trail the test reads back.
func milestoneServiceOn(t *testing.T, trail *recorded) *milestone.Service {
	t.Helper()

	gen := ids.NewGenerator()
	store := milestone.Store{
		Types:  newMilestoneTypeStore(),
		Atomic: directAtomic{},
	}
	return milestone.NewService(store, gen, audit.NewTrail(trail, testClock(), gen))
}
