package auth

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/platform/clock"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/password"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

const goodPasswd = "a long enough password"

type fakeUsers struct {
	rows           map[ids.ID]User
	failWith       error
	failUpdateWith error
	updates        int
}

func newFakeUsers(users ...User) *fakeUsers {
	f := &fakeUsers{rows: make(map[ids.ID]User, len(users))}
	for _, u := range users {
		f.rows[u.ID] = u
	}
	return f
}

func (f *fakeUsers) ByID(_ context.Context, id ids.ID) (User, error) {
	if f.failWith != nil {
		return User{}, f.failWith
	}
	user, found := f.rows[id]
	if !found {
		return User{}, ErrNoUser
	}
	return user, nil
}

func (f *fakeUsers) ByEmail(_ context.Context, email string) (User, error) {
	if f.failWith != nil {
		return User{}, f.failWith
	}
	for _, user := range f.rows {
		if user.Email == email {
			return user, nil
		}
	}
	return User{}, ErrNoUser
}

func (f *fakeUsers) Create(_ context.Context, user User) error {
	if f.failWith != nil {
		return f.failWith
	}
	for _, held := range f.rows {
		if held.Email == user.Email {
			return ErrEmailTaken
		}
	}
	f.rows[user.ID] = user
	return nil
}

func (f *fakeUsers) Update(_ context.Context, user User) error {
	if f.failWith != nil {
		return f.failWith
	}
	if f.failUpdateWith != nil {
		return f.failUpdateWith
	}
	if _, found := f.rows[user.ID]; !found {
		return ErrNoUser
	}
	f.updates++
	f.rows[user.ID] = user
	return nil
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

func newTestService(t *testing.T, users *fakeUsers) *Service {
	t.Helper()
	svc, _ := newAuditedService(t, users)
	return svc
}

// newAuditedService returns the service, and the trail it records to.
func newAuditedService(t *testing.T, users *fakeUsers) (*Service, *fakeRecorder) {
	t.Helper()
	clk := clock.Fixed(time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC), time.UTC)
	recorder := &fakeRecorder{}
	trail := audit.NewTrail(recorder, clk, ids.NewDeterministic(100))
	return NewService(users, directAtomic{}, NewThrottle(clk), ids.NewDeterministic(1), trail), recorder
}

// seedUser returns a stored user holding goodPasswd.
func seedUser(t *testing.T, email string) User {
	t.Helper()
	hash, err := password.Hash(goodPasswd)
	if err != nil {
		t.Fatalf("Hashing the seed password failed: %v", err)
	}
	user := User{
		ID:           ids.MustParse("01912345-6789-7abc-def0-123456789abc"),
		Email:        email,
		FirstName:    "Ada",
		LastName:     "Lovelace",
		PasswdHash:   hash,
		Role:         MemberRole,
		Active:       true,
		SessionEpoch: 3,
	}
	return user
}

// wantRejected fails the test unless err is the login rejection.
func wantRejected(t *testing.T, err error) {
	t.Helper()
	errs, ok := validate.From(err)
	if !ok {
		t.Fatalf("Rejection is not FieldErrors: %v", err)
	}
	want := validate.FieldErrors{{Field: "", Code: validate.Incorrect}}
	if !slices.Equal(errs, want) {
		t.Fatalf("Incorrect rejection: want=%v, got=%v", want, errs)
	}
}

func TestAuthenticateAcceptsRightPassword(t *testing.T) {
	user := seedUser(t, "ada@example.com")
	svc := newTestService(t, newFakeUsers(user))

	identity, principal, err := svc.Authenticate(t.Context(), "ada@example.com", goodPasswd)
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if identity != user.Identity() {
		t.Errorf("Incorrect identity: want=%+v, got=%+v", user.Identity(), identity)
	}
	if want := (Principal{UserID: user.ID, Epoch: 3}); principal != want {
		t.Errorf("Incorrect principal: want=%+v, got=%+v", want, principal)
	}
}

func TestAuthenticateNormalizesEmail(t *testing.T) {
	svc := newTestService(t, newFakeUsers(seedUser(t, "ada@example.com")))

	if _, _, err := svc.Authenticate(t.Context(), "  Ada@Example.COM  ", goodPasswd); err != nil {
		t.Errorf("An address in another case and with spaces was refused: %v", err)
	}
}

func TestAuthenticateRefusalsAreIdentical(t *testing.T) {
	inactive := seedUser(t, "off@example.com")
	inactive.ID = ids.MustParse("01912345-6789-7abc-def0-00000000000f")
	inactive.Active = false

	svc := newTestService(t, newFakeUsers(seedUser(t, "ada@example.com"), inactive))

	cases := map[string][2]string{
		"wrong password":   {"ada@example.com", "another long password"},
		"unknown email":    {"nobody@example.com", goodPasswd},
		"deactivated user": {"off@example.com", goodPasswd},
	}

	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			identity, principal, err := svc.Authenticate(t.Context(), in[0], in[1])
			wantRejected(t, err)
			if !identity.IsZero() || principal != (Principal{}) {
				t.Errorf("A refusal returned a subject: %+v, %+v", identity, principal)
			}
		})
	}
}

func TestAuthenticateVerifiesAnUnknownEmailAgainstTheDummy(t *testing.T) {
	svc := newTestService(t, newFakeUsers())

	_, _, err := svc.Authenticate(t.Context(), "nobody@example.com", goodPasswd)

	wantRejected(t, err)
	if errors.Is(err, password.ErrInvalidHash) {
		t.Error("An unknown email reached Verify with no hash")
	}
}

func TestAuthenticateReportsStoreFailure(t *testing.T) {
	users := newFakeUsers()
	users.failWith = errors.New("the database is down")
	svc := newTestService(t, users)

	_, _, err := svc.Authenticate(t.Context(), "ada@example.com", goodPasswd)

	if _, ok := validate.From(err); ok {
		t.Fatalf("A store failure was reported as a rejection: %v", err)
	}
	if !errors.Is(err, ErrNotResolved) {
		t.Errorf("A store failure is not ErrNotResolved: %v", err)
	}
}

func TestAuthenticateHoldsBackAfterFreeAttempts(t *testing.T) {
	user := seedUser(t, "ada@example.com")
	svc := newTestService(t, newFakeUsers(user))

	for range freeAttempts {
		if _, _, err := svc.Authenticate(t.Context(), "ada@example.com", "wrong password here"); err == nil {
			t.Fatal("A wrong password was accepted")
		}
	}

	// The throttle is shut, so even the right password is refused now.
	_, _, err := svc.Authenticate(t.Context(), "ada@example.com", goodPasswd)
	wantRejected(t, err)
}

func TestAuthenticateThrottleCountsNormalizedEmail(t *testing.T) {
	svc := newTestService(t, newFakeUsers(seedUser(t, "ada@example.com")))

	for range freeAttempts {
		if _, _, err := svc.Authenticate(t.Context(), "ADA@example.com ", "wrong password here"); err == nil {
			t.Fatal("A wrong password was accepted")
		}
	}

	if _, _, err := svc.Authenticate(t.Context(), "ada@example.com", goodPasswd); err == nil {
		t.Error("Changing the case of the address walked around the throttle")
	}
}

func TestResolveReturnsIdentity(t *testing.T) {
	user := seedUser(t, "ada@example.com")
	svc := newTestService(t, newFakeUsers(user))

	identity, err := svc.Resolve(t.Context(), user.Principal().String())
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if identity != user.Identity() {
		t.Errorf("Incorrect identity: want=%+v, got=%+v", user.Identity(), identity)
	}
}

func TestResolveRefusesStaleEpoch(t *testing.T) {
	user := seedUser(t, "ada@example.com")
	svc := newTestService(t, newFakeUsers(user))

	old := Principal{UserID: user.ID, Epoch: user.SessionEpoch - 1}

	_, err := svc.Resolve(t.Context(), old.String())
	if !errors.Is(err, ErrNotResolved) || !errors.Is(err, ErrStaleEpoch) {
		t.Errorf("Incorrect error for a stale epoch: %v", err)
	}
}

func TestResolveRefusesDeactivatedUser(t *testing.T) {
	user := seedUser(t, "ada@example.com")
	user.Active = false
	svc := newTestService(t, newFakeUsers(user))

	_, err := svc.Resolve(t.Context(), user.Principal().String())
	if !errors.Is(err, ErrNotResolved) || !errors.Is(err, ErrInactive) {
		t.Errorf("Incorrect error for a deactivated user: %v", err)
	}
}

func TestResolveRefusesAnUnreadableSubject(t *testing.T) {
	svc := newTestService(t, newFakeUsers())

	_, err := svc.Resolve(t.Context(), "not a subject")
	if !errors.Is(err, ErrNotResolved) {
		t.Errorf("Incorrect error for an unreadable subject: %v", err)
	}
}

func TestChangePasswordEndsEveryOtherSession(t *testing.T) {
	user := seedUser(t, "ada@example.com")
	user.PasswdExpired = true
	users := newFakeUsers(user)
	svc := newTestService(t, users)

	const next = "the next long password"

	principal, err := svc.ChangePassword(t.Context(), user.ID, goodPasswd, next)
	if err != nil {
		t.Fatalf("ChangePassword failed: %v", err)
	}

	// The device that made the change keeps its session.
	if principal.Epoch != user.SessionEpoch+1 {
		t.Errorf("Incorrect new epoch: want=%d, got=%d", user.SessionEpoch+1, principal.Epoch)
	}
	if _, err := svc.Resolve(t.Context(), principal.String()); err != nil {
		t.Errorf("The device that changed the password lost its session: %v", err)
	}

	// Every other device loses it.
	if _, err := svc.Resolve(t.Context(), user.Principal().String()); !errors.Is(err, ErrStaleEpoch) {
		t.Errorf("Another device kept its session: %v", err)
	}

	stored := users.rows[user.ID]
	if stored.PasswdExpired {
		t.Error("The change did not clear passwd_expired")
	}
	if _, _, err := svc.Authenticate(t.Context(), user.Email, next); err != nil {
		t.Errorf("The new password does not log in: %v", err)
	}
}

func TestChangePasswordRefusesWrongCurrentPassword(t *testing.T) {
	user := seedUser(t, "ada@example.com")
	users := newFakeUsers(user)
	svc := newTestService(t, users)

	_, err := svc.ChangePassword(t.Context(), user.ID, "not the password", "the next long password")

	errs, ok := validate.From(err)
	if !ok {
		t.Fatalf("Rejection is not FieldErrors: %v", err)
	}
	if _, found := errs.Get(FieldCurrentPass); !found {
		t.Errorf("Rejection does not name %q: %v", FieldCurrentPass, errs)
	}
	if users.updates != 0 {
		t.Error("A refused change still wrote the row")
	}
}

func TestChangePasswordHoldsLengthPolicy(t *testing.T) {
	user := seedUser(t, "ada@example.com")
	users := newFakeUsers(user)
	svc := newTestService(t, users)

	_, err := svc.ChangePassword(t.Context(), user.ID, goodPasswd, "short")

	errs, ok := validate.From(err)
	if !ok {
		t.Fatalf("Rejection is not FieldErrors: %v", err)
	}
	if _, found := errs.Get(FieldNewPass); !found {
		t.Errorf("Rejection does not name %q: %v", FieldNewPass, errs)
	}
	if users.updates != 0 {
		t.Error("A refused change still wrote the row")
	}
}

func TestCreateWritesCorrectFields(t *testing.T) {
	users := newFakeUsers()
	svc := newTestService(t, users)

	in := NewUser{
		Email:     " Ada@Example.COM ",
		FirstName: "Ada",
		LastName:  "Lovelace",
		Role:      RootRole,
		Passwd:    goodPasswd,
	}

	user, err := svc.Create(t.Context(), in)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	switch {
	case user.Email != "ada@example.com":
		t.Errorf("Create did not normalize the email: %q", user.Email)
	case !user.PasswdExpired:
		t.Error("Create did not set passwd_expired")
	case !user.Active:
		t.Error("Create did not set active")
	case user.SessionEpoch != 1:
		t.Errorf("Incorrect first epoch: want=1, got=%d", user.SessionEpoch)
	case user.PasswdHash == goodPasswd:
		t.Error("Create stored the password in plain text")
	}

	if _, _, err := svc.Authenticate(t.Context(), "ada@example.com", goodPasswd); err != nil {
		t.Errorf("The new user can't log in: %v", err)
	}
}

func TestCreateRefusesDuplicateEmail(t *testing.T) {
	users := newFakeUsers(seedUser(t, "ada@example.com"))
	svc := newTestService(t, users)

	_, err := svc.Create(t.Context(), NewUser{
		Email:     "ADA@example.com",
		FirstName: "Ada",
		LastName:  "Byron",
		Role:      MemberRole,
		Passwd:    goodPasswd,
	})

	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("Incorrect error for a duplicate email: %v", err)
	}
}

func TestCreateChecksItsInput(t *testing.T) {
	svc := newTestService(t, newFakeUsers())

	cases := map[string]struct {
		in    NewUser
		field string
	}{
		"no email":       {NewUser{FirstName: "A", LastName: "B", Role: MemberRole, Passwd: goodPasswd}, FieldEmail},
		"no first name":  {NewUser{Email: "a@b.c", LastName: "B", Role: MemberRole, Passwd: goodPasswd}, FieldFirstName},
		"unknown role":   {NewUser{Email: "a@b.c", FirstName: "A", LastName: "B", Role: "manager", Passwd: goodPasswd}, FieldRole},
		"short password": {NewUser{Email: "a@b.c", FirstName: "A", LastName: "B", Role: MemberRole, Passwd: "short"}, FieldPasswd},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := svc.Create(t.Context(), tc.in)
			errs, ok := validate.From(err)
			if !ok {
				t.Fatalf("Rejection is not FieldErrors: %v", err)
			}
			if _, found := errs.Get(tc.field); !found {
				t.Errorf("Rejection does not name %q: %v", tc.field, errs)
			}
		})
	}
}

func TestSetActiveBumpsEpochOnlyWhenDeactivating(t *testing.T) {
	user := seedUser(t, "ada@example.com")
	users := newFakeUsers(user)
	svc := newTestService(t, users)

	if err := svc.SetActive(t.Context(), user.ID, false); err != nil {
		t.Fatalf("SetActive(false) failed: %v", err)
	}
	off := users.rows[user.ID]
	if off.Active {
		t.Error("SetActive(false) left the user active")
	}
	if off.SessionEpoch != user.SessionEpoch+1 {
		t.Errorf("Deactivating did not bump the epoch: want=%d, got=%d", user.SessionEpoch+1, off.SessionEpoch)
	}

	if err := svc.SetActive(t.Context(), user.ID, true); err != nil {
		t.Fatalf("SetActive(true) failed: %v", err)
	}
	on := users.rows[user.ID]
	if on.SessionEpoch != off.SessionEpoch {
		t.Errorf("Reactivating bumped the epoch: want=%d, got=%d", off.SessionEpoch, on.SessionEpoch)
	}
}

func TestSetRoleKeepsTheSession(t *testing.T) {
	user := seedUser(t, "ada@example.com")
	users := newFakeUsers(user)
	svc := newTestService(t, users)

	if err := svc.SetRole(t.Context(), user.ID, RootRole); err != nil {
		t.Fatalf("SetRole failed: %v", err)
	}

	stored := users.rows[user.ID]
	if stored.Role != RootRole {
		t.Errorf("Incorrect role: want=%q, got=%q", RootRole, stored.Role)
	}
	if stored.SessionEpoch != user.SessionEpoch {
		t.Errorf("SetRole bumped the epoch: want=%d, got=%d", user.SessionEpoch, stored.SessionEpoch)
	}
}

func TestSetRoleRefusesUnknownRole(t *testing.T) {
	user := seedUser(t, "ada@example.com")
	users := newFakeUsers(user)
	svc := newTestService(t, users)

	err := svc.SetRole(t.Context(), user.ID, "manager")

	errs, ok := validate.From(err)
	if !ok {
		t.Fatalf("Rejection is not FieldErrors: %v", err)
	}
	if _, found := errs.Get(FieldRole); !found {
		t.Errorf("Rejection does not name %q: %v", FieldRole, errs)
	}
	if users.updates != 0 {
		t.Error("A refused role change still wrote the row")
	}
}

func TestNewServiceRefusesNilDependencies(t *testing.T) {
	clk := clock.Fixed(time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC), time.UTC)
	trail := audit.NewTrail(&fakeRecorder{}, clk, ids.NewDeterministic(1))

	cases := map[string]func(){
		"nil users":    func() { NewService(nil, directAtomic{}, NewThrottle(clk), ids.NewDeterministic(1), trail) },
		"nil atomic":   func() { NewService(newFakeUsers(), nil, NewThrottle(clk), ids.NewDeterministic(1), trail) },
		"nil throttle": func() { NewService(newFakeUsers(), directAtomic{}, nil, ids.NewDeterministic(1), trail) },
		"nil ids":      func() { NewService(newFakeUsers(), directAtomic{}, NewThrottle(clk), nil, trail) },
		"nil trail":    func() { NewService(newFakeUsers(), directAtomic{}, NewThrottle(clk), ids.NewDeterministic(1), nil) },
	}

	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("NewService accepted a nil dependency")
				}
			}()
			build()
		})
	}
}

func TestIdentityCarriesCorrectFields(t *testing.T) {
	user := seedUser(t, "ada@example.com")
	identity := user.Identity()

	if identity.UserID != user.ID || identity.Email != user.Email {
		t.Errorf("Identity lost a field: %+v", identity)
	}
	if identity.LogValue().String() != user.Email {
		t.Errorf("LogValue does not render the email: %q", identity.LogValue().String())
	}
	if (Identity{}).LogValue().String() != "anonymous" {
		t.Error("The zero identity does not log as anonymous")
	}
}

func TestCreateRecordsOneEntry(t *testing.T) {
	svc, trail := newAuditedService(t, newFakeUsers())

	user, err := svc.Create(t.Context(), NewUser{
		Email: "ada@example.com", FirstName: "Ada", LastName: "Lovelace",
		Role: MemberRole, Passwd: goodPasswd,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got := trail.only(t)
	if got.Entity != audit.EntityUser || got.EntityID != user.ID {
		t.Errorf("The entry names %s %s, want %s %s", got.Entity, got.EntityID, audit.EntityUser, user.ID)
	}
	if got.Action != audit.ActionCreated {
		t.Errorf("Action = %q, want %q", got.Action, audit.ActionCreated)
	}
}

func TestCreateWithoutSessionNamesTheNewUserAsActor(t *testing.T) {
	svc, trail := newAuditedService(t, newFakeUsers())

	user, err := svc.Create(t.Context(), NewUser{
		Email: "ada@example.com", FirstName: "Ada", LastName: "Lovelace",
		Role: RootRole, Passwd: goodPasswd,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if got := trail.only(t).ActorID; got != user.ID {
		t.Errorf("ActorID = %v, want the new user %v", got, user.ID)
	}
}

func TestWritesNameTheLoggedInUserAsActor(t *testing.T) {
	root := seedUser(t, "root@example.com")
	root.Role = RootRole
	svc, trail := newAuditedService(t, newFakeUsers(root))
	ctx := NewContext(t.Context(), root.Identity())

	_, err := svc.Create(ctx, NewUser{
		Email: "ada@example.com", FirstName: "Ada", LastName: "Lovelace",
		Role: MemberRole, Passwd: goodPasswd,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if got := trail.only(t).ActorID; got != root.ID {
		t.Errorf("ActorID = %v, want the logged-in root %v", got, root.ID)
	}
}

func TestCreateThatFailsRecordsNothing(t *testing.T) {
	svc, trail := newAuditedService(t, newFakeUsers(seedUser(t, "ada@example.com")))

	_, err := svc.Create(t.Context(), NewUser{
		Email: "ada@example.com", FirstName: "Ada", LastName: "Lovelace",
		Role: MemberRole, Passwd: goodPasswd,
	})
	if !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("Create = %v, want %v", err, ErrEmailTaken)
	}

	if len(trail.entries) != 0 {
		t.Errorf("The trail holds %d entries, want 0: %+v", len(trail.entries), trail.entries)
	}
}

func TestChangePasswordRecordsTheChangeAndNotThePassword(t *testing.T) {
	user := seedUser(t, "ada@example.com")
	svc, trail := newAuditedService(t, newFakeUsers(user))

	const newPasswd = "another long password"
	if _, err := svc.ChangePassword(t.Context(), user.ID, goodPasswd, newPasswd); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	got := trail.only(t)
	if got.Action != audit.ActionPasswdChanged || got.FieldKey != FieldPasswd {
		t.Errorf("The entry is %s on %q, want %s on %q",
			got.Action, got.FieldKey, audit.ActionPasswdChanged, FieldPasswd)
	}
	if got.New != audit.Marker {
		t.Errorf("New = %q, want the marker %q", got.New, audit.Marker)
	}
	for _, secret := range []string{goodPasswd, newPasswd, user.PasswdHash} {
		if got.Old == secret || got.New == secret {
			t.Errorf("The entry carries the secret %q", secret)
		}
	}
}

func TestChangePasswordThatFailsRecordsNothing(t *testing.T) {
	user := seedUser(t, "ada@example.com")
	svc, trail := newAuditedService(t, newFakeUsers(user))

	_, err := svc.ChangePassword(t.Context(), user.ID, "the wrong password", "another long password")
	if err == nil {
		t.Fatal("ChangePassword accepted the wrong current password")
	}

	if len(trail.entries) != 0 {
		t.Errorf("The trail holds %d entries, want 0: %+v", len(trail.entries), trail.entries)
	}
}

func TestSetRoleRecordsBothValues(t *testing.T) {
	user := seedUser(t, "ada@example.com")
	svc, trail := newAuditedService(t, newFakeUsers(user))

	if err := svc.SetRole(t.Context(), user.ID, RootRole); err != nil {
		t.Fatalf("SetRole: %v", err)
	}

	got := trail.only(t)
	if got.Action != audit.ActionRoleChanged || got.FieldKey != FieldRole {
		t.Errorf("The entry is %s on %q, want %s on %q",
			got.Action, got.FieldKey, audit.ActionRoleChanged, FieldRole)
	}
	if got.Old != string(MemberRole) || got.New != string(RootRole) {
		t.Errorf("The entry moves %q to %q, want %q to %q", got.Old, got.New, MemberRole, RootRole)
	}
}

func TestSetRoleToTheHeldRoleRecordsNothing(t *testing.T) {
	user := seedUser(t, "ada@example.com")
	svc, trail := newAuditedService(t, newFakeUsers(user))

	if err := svc.SetRole(t.Context(), user.ID, user.Role); err != nil {
		t.Fatalf("SetRole: %v", err)
	}

	if len(trail.entries) != 0 {
		t.Errorf("The trail holds %d entries, want 0: %+v", len(trail.entries), trail.entries)
	}
}

func TestSetActiveRecordsEachDirection(t *testing.T) {
	cases := map[string]struct {
		held, want bool
		action     string
	}{
		"deactivate": {held: true, want: false, action: audit.ActionDeactivated},
		"reactivate": {held: false, want: true, action: audit.ActionReactivated},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			user := seedUser(t, "ada@example.com")
			user.Active = c.held
			svc, trail := newAuditedService(t, newFakeUsers(user))

			if err := svc.SetActive(t.Context(), user.ID, c.want); err != nil {
				t.Fatalf("SetActive: %v", err)
			}

			got := trail.only(t)
			if got.Action != c.action || got.FieldKey != FieldActive {
				t.Errorf("The entry is %s on %q, want %s on %q",
					got.Action, got.FieldKey, c.action, FieldActive)
			}
			if got.Old != strconv.FormatBool(c.held) || got.New != strconv.FormatBool(c.want) {
				t.Errorf("The entry moves %q to %q, want %v to %v", got.Old, got.New, c.held, c.want)
			}
		})
	}
}

func TestSetActiveToTheHeldStateRecordsNothing(t *testing.T) {
	user := seedUser(t, "ada@example.com")
	svc, trail := newAuditedService(t, newFakeUsers(user))

	if err := svc.SetActive(t.Context(), user.ID, user.Active); err != nil {
		t.Fatalf("SetActive: %v", err)
	}

	if len(trail.entries) != 0 {
		t.Errorf("The trail holds %d entries, want 0: %+v", len(trail.entries), trail.entries)
	}
}

func TestWriteThatFailsRecordsNothing(t *testing.T) {
	user := seedUser(t, "ada@example.com")
	users := newFakeUsers(user)
	svc, trail := newAuditedService(t, users)
	users.failUpdateWith = errors.New("the store is down")

	if err := svc.SetRole(t.Context(), user.ID, RootRole); err == nil {
		t.Fatal("SetRole returned nil, want the store failure")
	}

	if len(trail.entries) != 0 {
		t.Errorf("The trail holds %d entries, want 0: %+v", len(trail.entries), trail.entries)
	}
}

func TestAuthenticateRecordsNothing(t *testing.T) {
	user := seedUser(t, "ada@example.com")
	svc, trail := newAuditedService(t, newFakeUsers(user))

	if _, _, err := svc.Authenticate(t.Context(), user.Email, goodPasswd); err != nil {
		t.Fatalf("Authenticate: %v", err)
	}

	if len(trail.entries) != 0 {
		t.Errorf("The trail holds %d entries, want 0: %+v", len(trail.entries), trail.entries)
	}
}
