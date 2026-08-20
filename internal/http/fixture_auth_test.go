package http

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
	testEmail  = "ada@example.com"
	testPasswd = "a long enough password"

	// The user who never changed the first password.
	expiredEmail  = "grace@example.com"
	expiredPasswd = "the first password"

	// The one user the users section lets in.
	rootEmail  = "root@example.com"
	rootPasswd = "the password of the root"
)

var (
	testUserID    = ids.MustParse("01912345-6789-7abc-def0-123456789abc")
	expiredUserID = ids.MustParse("01912345-6789-7abc-def0-123456789abd")
	rootUserID    = ids.MustParse("01912345-6789-7abc-def0-123456789abe")
)

// seedChange is the stamp the seeded rows carry as their last change. The fake
// store does not keep a clock, so the rows state it.
var seedChange = time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)

// testNow is the instant the fixed clock stands at. The trail stamps its
// entries with it, and the screens render them in the zone it carries.
var testNow = time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC)

func testClock() clock.Clock { return clock.Fixed(testNow, time.UTC) }

// Hashing costs about as much as the rest of a test, so each password is
// hashed once for the whole package.
var (
	testHash    = hashOnce(testPasswd)
	expiredHash = hashOnce(expiredPasswd)
	rootHash    = hashOnce(rootPasswd)
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
		ID:           testUserID,
		Email:        testEmail,
		FirstName:    "Ada",
		LastName:     "Lovelace",
		PasswdHash:   testHash(),
		Role:         auth.MemberRole,
		Active:       true,
		SessionEpoch: 1,
		UpdatedAt:    seedChange,
	}, {
		ID:            expiredUserID,
		Email:         expiredEmail,
		FirstName:     "Grace",
		LastName:      "Hopper",
		PasswdHash:    expiredHash(),
		Role:          auth.MemberRole,
		Active:        true,
		SessionEpoch:  1,
		PasswdExpired: true,
		UpdatedAt:     seedChange,
	}, {
		ID:           rootUserID,
		Email:        rootEmail,
		FirstName:    "Barbara",
		LastName:     "Liskov",
		PasswdHash:   rootHash(),
		Role:         auth.RootRole,
		Active:       true,
		SessionEpoch: 1,
		UpdatedAt:    seedChange,
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

// recorded is the trail a test reads back, for the screens that must leave an
// entry behind.
type recorded struct {
	mu      sync.Mutex
	entries []audit.Entry
}

func (r *recorded) Record(_ context.Context, entries ...audit.Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries = append(r.entries, entries...)
	return nil
}

// ByEntity is the read side: the entries of one entity, newest first, at most
// limit of them. The clock is fixed, so entries stamped alike come back in the
// reverse of the order they were recorded in.
func (r *recorded) ByEntity(_ context.Context, entity string, entityID ids.ID, limit int) ([]audit.Entry, error) {
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

// actions is what the trail holds, in the order it was recorded.
func (r *recorded) actions() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	actions := make([]string, len(r.entries))
	for i, entry := range r.entries {
		actions[i] = entry.Action
	}
	return actions
}

// holds reports whether every value the trail carries stays clear of secret.
func (r *recorded) holds(secret string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, entry := range r.entries {
		if entry.Old == secret || entry.New == secret {
			return true
		}
	}
	return false
}

// authServiceOn returns the service, recording to the trail given.
func authServiceOn(t *testing.T, trail *recorded) *auth.Service {
	t.Helper()

	clk := testClock()
	gen := ids.NewGenerator()
	return auth.NewService(newFakeUsers(), directAtomic{}, auth.NewThrottle(clk), gen,
		audit.NewTrail(trail, clk, gen))
}

// auditedDeps is deps with a trail the test reads back. Both services record to
// it, and the screens read the same trail back through the log.
func auditedDeps(t *testing.T) (Deps, *recorded) {
	t.Helper()

	trail := &recorded{}
	d := baseDeps(t)
	d.AuthSvc = authServiceOn(t, trail)
	d.CatalogSvc = catalogServiceOn(t, trail)
	d.AuditLog = audit.NewLog(trail, testClock())
	return d, trail
}

// deactivate turns a seeded user off, the way the users section does.
func deactivate(t *testing.T, deps Deps, id ids.ID) error {
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
