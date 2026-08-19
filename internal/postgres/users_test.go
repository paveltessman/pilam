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
func sample(email, fristName string) auth.User {
	return auth.User{
		ID:            gen.New(),
		Email:         email,
		FirstName:     fristName,
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

// sameUser compares the row that came back with the one the test wrote.
//
// UpdatedAt is left out: the store stamps it, so no caller can state it. The
// tests below check it on its own.
func sameUser(t *testing.T, got, want auth.User) {
	t.Helper()

	want.UpdatedAt = got.UpdatedAt
	if got != want {
		t.Errorf("ByID = %+v, want %+v", got, want)
	}
	if got.UpdatedAt.IsZero() {
		t.Error("The row carries no UpdatedAt")
	}
}

func TestUsersRoundTripEveryField(t *testing.T) {
	users := usersDB(t)
	want := create(t, users, sample("ada@example.com", "Ada"))

	got, err := users.ByID(t.Context(), want.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	sameUser(t, got, want)
}

func TestUsersByEmailIgnoresCase(t *testing.T) {
	users := usersDB(t)
	want := create(t, users, sample("ada@example.com", "Ada"))

	got, err := users.ByEmail(t.Context(), "Ada@Example.COM")
	if err != nil {
		t.Fatalf("ByEmail: %v", err)
	}
	sameUser(t, got, want)
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
	create(t, users, sample("ada@example.com", "Ada"))

	for _, email := range []string{"ada@example.com", "ADA@example.com"} {
		err := users.Create(t.Context(), sample(email, "Ada"))
		if !errors.Is(err, auth.ErrEmailTaken) {
			t.Errorf("Create %q error = %v, want %v", email, err, auth.ErrEmailTaken)
		}
	}
}

func TestUsersHoldEmailOfInactiveUser(t *testing.T) {
	users := usersDB(t)
	taken := sample("ada@example.com", "Ada")
	taken.Active = false
	create(t, users, taken)

	if err := users.Create(t.Context(), sample("ada@example.com", "Ada")); !errors.Is(err, auth.ErrEmailTaken) {
		t.Errorf("Create error = %v, want %v", err, auth.ErrEmailTaken)
	}
}

func TestUsersUpdateWritesEveryField(t *testing.T) {
	users := usersDB(t)
	want := create(t, users, sample("ada@example.com", "Ada"))

	written, err := users.ByID(t.Context(), want.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}

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
	sameUser(t, got, want)

	if !got.UpdatedAt.After(written.UpdatedAt) {
		t.Errorf("UpdatedAt = %v, want it after the create stamp %v", got.UpdatedAt, written.UpdatedAt)
	}
}

func TestUsersListOrdersByFirstNameAndHoldsInactiveRows(t *testing.T) {
	users := usersDB(t)
	create(t, users, sample("bob@example.com", "Bob"))
	create(t, users, sample("ada@example.com", "Ada"))

	got, err := users.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("List returned %d rows, want 2", len(got))
	}
	// sample writes an inactive row, so a list that holds these two holds
	// deactivated users as well.
	if got[0].FirstName != "Ada" || got[1].FirstName != "Bob" {
		t.Errorf("List is not ordered by first name: %q then %q", got[0].FirstName, got[1].FirstName)
	}
}

func TestUsersListIsEmptyWithoutRows(t *testing.T) {
	users := usersDB(t)

	got, err := users.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("List returned %d rows, want 0", len(got))
	}
}

func TestUsersUpdateReportsMissingRow(t *testing.T) {
	users := usersDB(t)

	if err := users.Update(t.Context(), sample("ada@example.com", "Ada")); !errors.Is(err, auth.ErrNoUser) {
		t.Errorf("Update error = %v, want %v", err, auth.ErrNoUser)
	}
}

func TestUsersUpdateRefusesTakenEmail(t *testing.T) {
	users := usersDB(t)
	create(t, users, sample("ada@example.com", "Ada"))
	mover := create(t, users, sample("grace@example.com", "Ada"))

	mover.Email = "ada@example.com"
	if err := users.Update(t.Context(), mover); !errors.Is(err, auth.ErrEmailTaken) {
		t.Errorf("Update error = %v, want %v", err, auth.ErrEmailTaken)
	}
}

func TestUsersWriteRollsBackWithTransaction(t *testing.T) {
	db := openTestDB(t)
	users := NewUsers(db)

	sentinel := errors.New("the work after the write failed")
	user := sample("ada@example.com", "Ada")

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
	user := sample("ada@example.com", "Ada")

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
