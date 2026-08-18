package http

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/platform/clock"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/password"
)

const (
	testEmail  = "ada@example.com"
	testPasswd = "a long enough password"
)

var testUserID = ids.MustParse("01912345-6789-7abc-def0-123456789abc")

var testHash = sync.OnceValue(func() string {
	hash, err := password.Hash(testPasswd)
	if err != nil {
		panic(err)
	}
	return hash
})

type fakeUsers struct {
	mu   sync.Mutex
	rows map[ids.ID]auth.User
}

func newFakeUsers() *fakeUsers {
	user := auth.User{
		ID:           testUserID,
		Email:        testEmail,
		FirstName:    "Ada",
		LastName:     "Lovelace",
		PasswdHash:   testHash(),
		Role:         auth.MemberRole,
		Active:       true,
		SessionEpoch: 1,
	}
	return &fakeUsers{rows: map[ids.ID]auth.User{user.ID: user}}
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

func authService(t *testing.T) *auth.Service {
	t.Helper()
	clk := clock.Fixed(time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC), time.UTC)
	return auth.NewService(newFakeUsers(), directAtomic{}, auth.NewThrottle(clk), ids.NewGenerator())
}
