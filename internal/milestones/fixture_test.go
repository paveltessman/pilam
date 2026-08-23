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
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

var actorID = ids.MustParse("01912345-6789-7abc-def0-123456789abc")

// ---------------------------------------------------------------- fakes

// store is the in-memory stand-in for the ports of the package.
type store struct {
	types    map[ids.ID]Type
	failWith error
}

func newStore() *store {
	return &store{types: make(map[ids.ID]Type)}
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

// The ports, each one a view of the same store.
type typesOf struct{ *store }

func (t typesOf) ByID(ctx context.Context, id ids.ID) (Type, error) { return t.TypeByID(ctx, id) }
func (t typesOf) List(ctx context.Context) ([]Type, error)          { return t.TypeList(ctx) }
func (t typesOf) Create(ctx context.Context, in Type) error         { return t.TypeCreate(ctx, in) }
func (t typesOf) Update(ctx context.Context, in Type) error         { return t.TypeUpdate(ctx, in) }

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
	stores := Store{Types: typesOf{rows}, Atomic: directAtomic{}}
	return NewService(stores, ids.NewDeterministic(1), trail), rows, recorder
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
