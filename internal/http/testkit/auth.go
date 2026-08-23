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
	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/platform/clock"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/password"
)

const (
	TestEmail  = "ada@example.com"
	TestPasswd = "a long enough password"

	// The user who never changed the first password.
	ExpiredEmail  = "grace@example.com"
	ExpiredPasswd = "the first password"

	// The one user the users section lets in.
	RootEmail  = "root@example.com"
	RootPasswd = "the password of the root"
)

var (
	TestUserID    = ids.MustParse("01912345-6789-7abc-def0-123456789abc")
	ExpiredUserID = ids.MustParse("01912345-6789-7abc-def0-123456789abd")
	RootUserID    = ids.MustParse("01912345-6789-7abc-def0-123456789abe")
)

// SeedChange is the stamp the seeded rows carry as their last change. The fake
// store does not keep a clock, so the rows state it.
var SeedChange = time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)

// Now is the instant the fixed clock stands at. The trail stamps its
// entries with it, and the screens render them in the zone it carries.
var Now = time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC)

func Clock() clock.Clock { return clock.Fixed(Now, time.UTC) }

// Hashing costs about as much as the rest of a test, so each password is
// hashed once for the whole package.
var (
	testHash    = hashOnce(TestPasswd)
	expiredHash = hashOnce(ExpiredPasswd)
	rootHash    = hashOnce(RootPasswd)
)

func hashOnce(plain string) func() string {
	return sync.OnceValue(func() string {
		hash, err := password.Hash(plain)
		if err != nil {
			panic(err)
		}
		return hash
	})
}

type fakeUsers struct {
	mu   sync.Mutex
	rows map[ids.ID]auth.User
}

func newFakeUsers() *fakeUsers {
	users := []auth.User{{
		ID:           TestUserID,
		Email:        TestEmail,
		FirstName:    "Ada",
		LastName:     "Lovelace",
		PasswdHash:   testHash(),
		Role:         auth.MemberRole,
		Active:       true,
		SessionEpoch: 1,
		UpdatedAt:    SeedChange,
	}, {
		ID:            ExpiredUserID,
		Email:         ExpiredEmail,
		FirstName:     "Grace",
		LastName:      "Hopper",
		PasswdHash:    expiredHash(),
		Role:          auth.MemberRole,
		Active:        true,
		SessionEpoch:  1,
		PasswdExpired: true,
		UpdatedAt:     SeedChange,
	}, {
		ID:           RootUserID,
		Email:        RootEmail,
		FirstName:    "Barbara",
		LastName:     "Liskov",
		PasswdHash:   rootHash(),
		Role:         auth.RootRole,
		Active:       true,
		SessionEpoch: 1,
		UpdatedAt:    SeedChange,
	}}

	rows := make(map[ids.ID]auth.User, len(users))
	for _, user := range users {
		rows[user.ID] = user
	}
	return &fakeUsers{rows: rows}
}

func (f *fakeUsers) ByID(_ context.Context, id ids.ID) (auth.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	user, found := f.rows[id]
	if !found {
		return auth.User{}, auth.ErrNoUser
	}
	return user, nil
}

func (f *fakeUsers) ByEmail(_ context.Context, email string) (auth.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, user := range f.rows {
		if user.Email == email {
			return user, nil
		}
	}
	return auth.User{}, auth.ErrNoUser
}

func (f *fakeUsers) List(_ context.Context) ([]auth.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	users := slices.Collect(maps.Values(f.rows))
	slices.SortFunc(users, func(a, b auth.User) int { return strings.Compare(a.Email, b.Email) })
	return users, nil
}

func (f *fakeUsers) Create(_ context.Context, user auth.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, held := range f.rows {
		if held.Email == user.Email {
			return auth.ErrEmailTaken
		}
	}
	f.rows[user.ID] = user
	return nil
}

func (f *fakeUsers) Update(_ context.Context, user auth.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, found := f.rows[user.ID]; !found {
		return auth.ErrNoUser
	}
	f.rows[user.ID] = user
	return nil
}

type directAtomic struct{}

func (directAtomic) InTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

// Trail is the trail a test reads back, for the screens that must leave an
// entry behind.
type Trail struct {
	mu      sync.Mutex
	entries []audit.Entry
}

func (r *Trail) Record(_ context.Context, entries ...audit.Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries = append(r.entries, entries...)
	return nil
}

// ByEntity is the read side: the entries of one entity, newest first, at most
// limit of them. The clock is fixed, so entries stamped alike come back in the
// reverse of the order they were Trail in.
func (r *Trail) ByEntity(_ context.Context, entity string, entityID ids.ID, limit int) ([]audit.Entry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var held []audit.Entry
	for _, entry := range r.entries {
		if entry.Entity == entity && entry.EntityID == entityID {
			held = append(held, entry)
		}
	}

	slices.Reverse(held)
	if len(held) > limit {
		held = held[:limit]
	}
	return held, nil
}

// Actions is what the trail holds, in the order it was Trail.
func (r *Trail) Actions() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	actions := make([]string, len(r.entries))
	for i, entry := range r.entries {
		actions[i] = entry.Action
	}
	return actions
}

// Holds reports whether every value the trail carries stays clear of secret.
func (r *Trail) Holds(secret string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, entry := range r.entries {
		if entry.Old == secret || entry.New == secret {
			return true
		}
	}
	return false
}

// AuthServiceOn returns the service, recording to the trail given.
func AuthServiceOn(t *testing.T, trail *Trail) *auth.Service {
	t.Helper()

	clk := Clock()
	gen := ids.NewGenerator()
	return auth.NewService(newFakeUsers(), directAtomic{}, auth.NewThrottle(clk), gen,
		audit.NewTrail(trail, clk, gen))
}

// Deactivate turns a seeded user off, the way the users section does.
func Deactivate(t *testing.T, deps Deps, id ids.ID) error {
	t.Helper()

	account, err := deps.AuthSvc.Account(t.Context(), id)
	if err != nil {
		return err
	}
	return deps.AuthSvc.Update(t.Context(), id, auth.UpdateParams{
		FirstName: account.FirstName,
		LastName:  account.LastName,
		Role:      account.Role,
		Active:    false,
	})
}
