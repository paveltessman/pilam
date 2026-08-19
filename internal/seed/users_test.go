package seed

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/password"
)

// short is a run of the first few employees. Hashing a password is deliberately
// slow, so a test that does not read the whole company writes a few rows.
const short = 4

func TestRosterIsWellFormed(t *testing.T) {
	locals := make(map[string]bool, len(roster))
	roots, inactive := 0, 0

	for _, person := range roster {
		switch {
		case person.first == "" || person.last == "":
			t.Errorf("An employee carries no name: %+v", person)
		case person.local == "":
			t.Errorf("%s %s carries no address", person.first, person.last)
		case !person.role.Valid():
			t.Errorf("%s carries the incorrect role %q", person.local, person.role)
		case locals[person.local]:
			t.Errorf("Two employees hold the address %q", person.local)
		}
		locals[person.local] = true

		if person.role == auth.RootRole {
			roots++
		}
		if !person.active {
			inactive++
		}
	}

	if roots == 0 {
		t.Error("Nobody on the roster reaches the user screens")
	}
	if roster[rootAt].role != auth.RootRole {
		t.Errorf("The roster opens with the %q %s, want a root: the dataset is created by them",
			roster[rootAt].role, roster[rootAt].local)
	}
	if inactive == 0 {
		t.Error("Nobody on the roster is deactivated, so the screen never shows that state")
	}
}

func TestRunWritesTheWholeRoster(t *testing.T) {
	svc, users, _ := newTestService(t)

	report := run(t, svc, Options{})

	switch {
	case report.Users.Created != len(roster):
		t.Errorf("Incorrect count: want=%d, got=%d", len(roster), report.Users.Created)
	case report.Users.Skipped != 0:
		t.Errorf("The first run skipped %d employees", report.Users.Skipped)
	case report.Users.Passwd != DefaultPasswd:
		t.Errorf("Incorrect password reported: want=%q, got=%q", DefaultPasswd, report.Users.Passwd)
	case len(users.rows) != len(roster):
		t.Errorf("The store holds %d rows, want %d", len(users.rows), len(roster))
	}

	roots, inactive := 0, 0
	for _, user := range users.rows {
		if user.Role == auth.RootRole {
			roots++
		}
		if !user.Active {
			inactive++
		}
	}
	if roots == 0 {
		t.Error("No seeded user reaches the user screens")
	}
	if inactive == 0 {
		t.Error("No seeded user is deactivated")
	}
}

func TestSeededUserLogsInWithTheOnePassword(t *testing.T) {
	svc, users, _ := newTestService(t)

	run(t, svc, Options{Users: short, Domain: "example.test"})

	for _, user := range users.rows {
		if !strings.HasSuffix(user.Email, "@example.test") {
			t.Errorf("Incorrect address: %q", user.Email)
		}
		if user.PasswdExpired {
			t.Errorf("%s has to change the password before reaching the board", user.Email)
		}

		ok, _, err := password.Verify(user.PasswdHash, DefaultPasswd)
		if err != nil {
			t.Fatalf("Verify: %v", err)
		}
		if !ok {
			t.Errorf("The reported password does not open %s", user.Email)
		}
	}
}

func TestRunWritesOnlyTheEmployeesAskedFor(t *testing.T) {
	svc, users, _ := newTestService(t)

	report := run(t, svc, Options{Users: short})

	if report.Users.Created != short || len(users.rows) != short {
		t.Errorf("Incorrect count: want=%d, created=%d, stored=%d", short, report.Users.Created, len(users.rows))
	}
}

func TestSecondRunChangesNothing(t *testing.T) {
	svc, users, _ := newTestService(t)

	run(t, svc, Options{Users: short})
	first := slices.Collect(maps.Keys(users.rows))

	report := run(t, svc, Options{Users: short})

	switch {
	case report.Users.Created != 0:
		t.Errorf("The second run wrote %d employees again", report.Users.Created)
	case report.Users.Skipped != short:
		t.Errorf("The second run skipped %d employees, want %d", report.Users.Skipped, short)
	case len(users.rows) != short:
		t.Errorf("The store holds %d rows, want %d", len(users.rows), short)
	}

	second := slices.Collect(maps.Keys(users.rows))
	if !sameIDs(first, second) {
		t.Error("The second run moved the rows of the first")
	}
}

func TestTwoRunsWriteTheSameIdentifiers(t *testing.T) {
	first, second := identifiers(t), identifiers(t)

	if !sameIDs(first, second) {
		t.Errorf("Two runs from empty wrote different identifiers:\n%v\n%v", first, second)
	}
}

// identifiers runs the seed against an empty store and returns what it keyed
// the rows by.
func identifiers(t *testing.T) []ids.ID {
	t.Helper()

	svc, users, _ := newTestService(t)
	run(t, svc, Options{Users: short})
	return slices.Collect(maps.Keys(users.rows))
}

func sameIDs(a, b []ids.ID) bool {
	sort := func(in []ids.ID) []ids.ID {
		out := slices.Clone(in)
		slices.SortFunc(out, func(x, y ids.ID) int { return strings.Compare(x.String(), y.String()) })
		return out
	}
	return slices.Equal(sort(a), sort(b))
}

func TestEverySeededWriteLeavesATrailEntry(t *testing.T) {
	svc, _, recorder := newTestService(t)

	run(t, svc, Options{Users: short})

	created := 0
	for _, entry := range recorder.entries {
		if entry.Entity != audit.EntityUser {
			t.Errorf("Incorrect entity in the trail: %q", entry.Entity)
		}
		if entry.ActorID == ids.Nil {
			t.Error("A trail entry names no actor")
		}
		if entry.Action == audit.ActionCreated {
			created++
		}
	}

	if created != short {
		t.Errorf("The trail holds %d creations, want %d", created, short)
	}
}

func TestRunRefusesOptionsItCantWorkFrom(t *testing.T) {
	cases := map[string]Options{
		"more than the roster holds": {Users: len(roster) + 1},
		"a negative count":           {Users: -1},
		"a short password":           {Passwd: "short"},
	}

	for name, opts := range cases {
		t.Run(name, func(t *testing.T) {
			svc, users, _ := newTestService(t)

			if _, err := Run(t.Context(), svc, opts); err == nil {
				t.Error("Run accepted the options")
			}
			if len(users.rows) != 0 {
				t.Errorf("A refused run still wrote %d rows", len(users.rows))
			}
		})
	}
}

// The whole demo dataset is created by one administrator, the root the roster
// opens with. That root is the only user who creates themselves.
func TestSeededUsersAreCreatedByTheRoot(t *testing.T) {
	svc, users, recorder := newTestService(t)

	run(t, svc, Options{Users: short})
	root := storedUser(t, users, roster[rootAt].email(DefaultDomain))

	for _, entry := range recorder.entries {
		actor, target := entry.ActorID, entry.EntityID
		if target == root.ID {
			if actor != root.ID {
				t.Errorf("The root is created by %s, want themselves", actor)
			}
			continue
		}
		if actor != root.ID {
			t.Errorf("User %s is created by %s, want the root %s", target, actor, root.ID)
		}
	}
}

// A later run writes the employees the roster grew by. The root of the first
// run is still the actor, so the whole trail names one administrator.
func TestALaterRunNamesTheRootOfTheFirst(t *testing.T) {
	svc, users, recorder := newTestService(t)

	run(t, svc, Options{Users: short})
	root := storedUser(t, users, roster[rootAt].email(DefaultDomain))
	written := len(recorder.entries)

	report := run(t, svc, Options{Users: short + 2})
	if report.Users.Created != 2 {
		t.Fatalf("The second run wrote %d employees, want 2", report.Users.Created)
	}

	later := recorder.entries[written:]
	if len(later) == 0 {
		t.Fatal("The second run left no entry behind")
	}
	for _, entry := range later {
		if entry.ActorID != root.ID {
			t.Errorf("User %s is created by %s, want the root %s of the first run",
				entry.EntityID, entry.ActorID, root.ID)
		}
	}
}

// storedUser returns the row the store holds under that address.
func storedUser(t *testing.T, users *fakeUsers, email string) auth.User {
	t.Helper()

	user, err := users.ByEmail(t.Context(), email)
	if err != nil {
		t.Fatalf("The store holds no user at %s: %v", email, err)
	}
	return user
}
