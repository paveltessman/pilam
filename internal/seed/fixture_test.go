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
	"github.com/paveltessman/pilam/internal/platform/clock"
	"github.com/paveltessman/pilam/internal/platform/ids"
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

// newTestService returns the service the seed writes through, the store behind
// it, and the trail it records to. It carries the identifier generator the
// command carries, so the tests see the identifiers a real run writes.
func newTestService(t *testing.T) (*auth.Service, *fakeUsers, *fakeRecorder) {
	t.Helper()

	clk := clock.Fixed(time.Date(2026, 8, 19, 9, 0, 0, 0, time.UTC), time.UTC)
	users := newFakeUsers()
	recorder := &fakeRecorder{}
	idGen := ids.NewDeterministic(IDSeed)
	trail := audit.NewTrail(recorder, clk, ids.NewDeterministic(IDSeed))

	svc := auth.NewService(users, directAtomic{}, auth.NewThrottle(clk), idGen, trail)
	return svc, users, recorder
}

// run loads the dataset and fails the test when it can't.
func run(t *testing.T, svc *auth.Service, opts Options) Report {
	t.Helper()

	report, err := Run(t.Context(), svc, opts)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	return report
}
