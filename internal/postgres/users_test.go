package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/platform/ids"
)

// gen issues the identifiers the tests write. The rows never leave the schema
// the test owns, so the sequence does not have to be unique across tests.
var gen = ids.NewGenerator()

// usersDB gives the test the port and an empty app_user table.
func usersDB(t *testing.T) *Users {
	t.Helper()
	return NewUsers(openTestDB(t))
}

// sample is one complete row.
func sample(email string) auth.User {
	return auth.User{
		ID:            gen.New(),
		Email:         email,
		FirstName:     "Ada",
		LastName:      "Lovelace",
		PasswdHash:    "$argon2id$v=19$m=65536,t=3,p=4$c2FsdHNhbHRzYWx0c2E$aGFzaA",
		Role:          auth.RootRole,
		Active:        false,
		SessionEpoch:  7,
		PasswdExpired: false,
	}
}

// create writes one row and fails the test when it cannot.
func create(t *testing.T, users *Users, user auth.User) auth.User {
	t.Helper()
	if err := users.Create(t.Context(), user); err != nil {
		t.Fatalf("Create: %v", err)
	}
	return user
}

func TestUsersRoundTripEveryField(t *testing.T) {
	users := usersDB(t)
	want := create(t, users, sample("ada@example.com"))

	got, err := users.ByID(t.Context(), want.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if got != want {
		t.Errorf("ByID = %+v, want %+v", got, want)
	}
}

func TestUsersByEmailIgnoresCase(t *testing.T) {
	users := usersDB(t)
	want := create(t, users, sample("ada@example.com"))

	got, err := users.ByEmail(t.Context(), "Ada@Example.COM")
	if err != nil {
		t.Fatalf("ByEmail: %v", err)
	}
	if got != want {
		t.Errorf("ByEmail = %+v, want %+v", got, want)
	}
}

func TestUsersReportMissingRowAsErrNoUser(t *testing.T) {
	users := usersDB(t)

	if _, err := users.ByID(t.Context(), gen.New()); !errors.Is(err, auth.ErrNoUser) {
		t.Errorf("ByID error = %v, want %v", err, auth.ErrNoUser)
	}
	if _, err := users.ByEmail(t.Context(), "nobody@example.com"); !errors.Is(err, auth.ErrNoUser) {
		t.Errorf("ByEmail error = %v, want %v", err, auth.ErrNoUser)
	}
}

func TestUsersRefuseTakenEmail(t *testing.T) {
	users := usersDB(t)
	create(t, users, sample("ada@example.com"))

	for _, email := range []string{"ada@example.com", "ADA@example.com"} {
		err := users.Create(t.Context(), sample(email))
		if !errors.Is(err, auth.ErrEmailTaken) {
			t.Errorf("Create %q error = %v, want %v", email, err, auth.ErrEmailTaken)
		}
	}
}

func TestUsersHoldEmailOfInactiveUser(t *testing.T) {
	users := usersDB(t)
	taken := sample("ada@example.com")
	taken.Active = false
	create(t, users, taken)

	if err := users.Create(t.Context(), sample("ada@example.com")); !errors.Is(err, auth.ErrEmailTaken) {
		t.Errorf("Create error = %v, want %v", err, auth.ErrEmailTaken)
	}
}

func TestUsersUpdateWritesEveryField(t *testing.T) {
	users := usersDB(t)
	want := create(t, users, sample("ada@example.com"))

	want.Email = "ada.lovelace@example.com"
	want.FirstName = "Augusta"
	want.LastName = "King"
	want.PasswdHash = "$argon2id$v=19$m=65536,t=3,p=4$c2FsdHNhbHRzYWx0c2I$b3RoZXI"
	want.Role = auth.MemberRole
	want.Active = true
	want.SessionEpoch = 8
	want.PasswdExpired = true

	if err := users.Update(t.Context(), want); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := users.ByID(t.Context(), want.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if got != want {
		t.Errorf("ByID = %+v, want %+v", got, want)
	}
}

func TestUsersUpdateReportsMissingRow(t *testing.T) {
	users := usersDB(t)

	if err := users.Update(t.Context(), sample("ada@example.com")); !errors.Is(err, auth.ErrNoUser) {
		t.Errorf("Update error = %v, want %v", err, auth.ErrNoUser)
	}
}

func TestUsersUpdateRefusesTakenEmail(t *testing.T) {
	users := usersDB(t)
	create(t, users, sample("ada@example.com"))
	mover := create(t, users, sample("grace@example.com"))

	mover.Email = "ada@example.com"
	if err := users.Update(t.Context(), mover); !errors.Is(err, auth.ErrEmailTaken) {
		t.Errorf("Update error = %v, want %v", err, auth.ErrEmailTaken)
	}
}

func TestUsersWriteRollsBackWithTransaction(t *testing.T) {
	db := openTestDB(t)
	users := NewUsers(db)

	sentinel := errors.New("the work after the write failed")
	user := sample("ada@example.com")

	err := db.InTx(t.Context(), func(ctx context.Context) error {
		if err := users.Create(ctx, user); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("InTx error = %v, want %v", err, sentinel)
	}

	if _, err := users.ByID(t.Context(), user.ID); !errors.Is(err, auth.ErrNoUser) {
		t.Errorf("ByID error = %v, want %v: the rollback left the row behind", err, auth.ErrNoUser)
	}
}

func TestUsersWriteCommitsWithTransaction(t *testing.T) {
	db := openTestDB(t)
	users := NewUsers(db)
	user := sample("ada@example.com")

	err := db.InTx(t.Context(), func(ctx context.Context) error {
		return users.Create(ctx, user)
	})
	if err != nil {
		t.Fatalf("InTx: %v", err)
	}

	if _, err := users.ByID(t.Context(), user.ID); err != nil {
		t.Errorf("ByID: %v", err)
	}
}
