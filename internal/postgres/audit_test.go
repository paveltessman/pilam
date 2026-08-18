package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/platform/ids"
)

// auditDB gives the test the recorder, the users port and one stored user the
// entries can name as their actor. The audit rows reference app_user.
func auditDB(t *testing.T) (*Audit, *Users, auth.User) {
	t.Helper()

	db := openTestDB(t)
	users := NewUsers(db)
	actor := create(t, users, sample("root@example.com"))
	return NewAudit(db), users, actor
}

// entry is one complete entry, with a value on both sides.
func entry(actor, target ids.ID) audit.Entry {
	entry := audit.Entry{
		ID:        gen.New(),
		At:        time.Date(2026, 8, 18, 9, 30, 0, 0, time.UTC),
		ActorID:   actor,
		Entity:    audit.EntityUser,
		EntityID:  target,
		Action:    audit.ActionRoleChanged,
		FieldKey:  "role",
		Old:       "member",
		New:       "root",
		RequestID: "req-7",
	}
	return entry
}

// stored reads the whole trail back, oldest first. A NULL value column reads as
// the empty string, which is the form the domain states it in.
func stored(t *testing.T, db *DB) []audit.Entry {
	t.Helper()

	const query = `SELECT id, at, actor_id, entity, entity_id,
	                      action, field_key, old_value, new_value, request_id
	               FROM audit_entry ORDER BY at, action`

	rows, err := db.pool.Query(t.Context(), query)
	if err != nil {
		t.Fatalf("reading the trail: %v", err)
	}
	defer rows.Close()

	var held []audit.Entry
	for rows.Next() {
		var got audit.Entry
		var old, current *string
		err := rows.Scan(&got.ID, &got.At, &got.ActorID, &got.Entity, &got.EntityID,
			&got.Action, &got.FieldKey, &old, &current, &got.RequestID)
		if err != nil {
			t.Fatalf("scanning an entry: %v", err)
		}
		got.At = got.At.UTC()
		if old != nil {
			got.Old = *old
		}
		if current != nil {
			got.New = *current
		}
		held = append(held, got)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading the trail: %v", err)
	}
	return held
}

func TestAuditRoundTripsEveryField(t *testing.T) {
	recorder, _, actor := auditDB(t)
	want := entry(actor.ID, actor.ID)

	if err := recorder.Record(t.Context(), want); err != nil {
		t.Fatalf("Record: %v", err)
	}

	held := stored(t, recorder.db)
	if len(held) != 1 {
		t.Fatalf("The trail holds %d entries, want 1", len(held))
	}
	if got := held[0]; got != want {
		t.Errorf("The stored entry = %+v, want %+v", got, want)
	}
}

func TestAuditStoresAbsentValueAsNull(t *testing.T) {
	recorder, _, actor := auditDB(t)

	created := entry(actor.ID, actor.ID)
	created.Action = audit.ActionCreated
	created.FieldKey = ""
	created.Old = ""
	created.New = ""

	if err := recorder.Record(t.Context(), created); err != nil {
		t.Fatalf("Record: %v", err)
	}

	const query = `SELECT old_value IS NULL AND new_value IS NULL FROM audit_entry`
	var null bool
	if err := recorder.db.pool.QueryRow(t.Context(), query).Scan(&null); err != nil {
		t.Fatalf("reading the value columns: %v", err)
	}
	if !null {
		t.Error("An absent value is stored as the empty string, want NULL")
	}
}

func TestAuditRecordsEveryEntryItGets(t *testing.T) {
	recorder, _, actor := auditDB(t)

	first := entry(actor.ID, actor.ID)
	second := entry(actor.ID, actor.ID)
	second.Action = audit.ActionDeactivated

	if err := recorder.Record(t.Context(), first, second); err != nil {
		t.Fatalf("Record: %v", err)
	}

	if held := stored(t, recorder.db); len(held) != 2 {
		t.Fatalf("The trail holds %d entries, want 2", len(held))
	}
}

func TestAuditRecordWithoutEntriesWritesNothing(t *testing.T) {
	recorder, _, _ := auditDB(t)

	if err := recorder.Record(t.Context()); err != nil {
		t.Fatalf("Record: %v", err)
	}

	if held := stored(t, recorder.db); len(held) != 0 {
		t.Errorf("The trail holds %d entries, want 0", len(held))
	}
}

func TestAuditEntryRollsBackWithTheWriteItDescribes(t *testing.T) {
	recorder, users, actor := auditDB(t)
	failure := errors.New("the work failed after the write")
	target := sample("ada@example.com")

	write := func(ctx context.Context) error {
		if err := users.Create(ctx, target); err != nil {
			return err
		}
		created := entry(actor.ID, target.ID)
		created.Action = audit.ActionCreated
		if err := recorder.Record(ctx, created); err != nil {
			return err
		}
		return failure
	}

	if err := recorder.db.InTx(t.Context(), write); !errors.Is(err, failure) {
		t.Fatalf("InTx = %v, want %v", err, failure)
	}

	if held := stored(t, recorder.db); len(held) != 0 {
		t.Errorf("The trail holds %d entries, want 0: %+v", len(held), held)
	}
	if _, err := users.ByID(t.Context(), target.ID); !errors.Is(err, auth.ErrNoUser) {
		t.Errorf("ByID = %v, want %v", err, auth.ErrNoUser)
	}
}

func TestAuditEntryLandsWithTheWriteItDescribes(t *testing.T) {
	recorder, users, actor := auditDB(t)
	target := sample("ada@example.com")

	write := func(ctx context.Context) error {
		if err := users.Create(ctx, target); err != nil {
			return err
		}
		created := entry(actor.ID, target.ID)
		created.Action = audit.ActionCreated
		return recorder.Record(ctx, created)
	}

	if err := recorder.db.InTx(t.Context(), write); err != nil {
		t.Fatalf("InTx: %v", err)
	}

	held := stored(t, recorder.db)
	if len(held) != 1 {
		t.Fatalf("The trail holds %d entries, want 1", len(held))
	}
	if held[0].EntityID != target.ID {
		t.Errorf("The entry names %v, want the created user %v", held[0].EntityID, target.ID)
	}
}

func TestAuditRefusesActorWithNoRow(t *testing.T) {
	recorder, _, actor := auditDB(t)
	unknown := entry(gen.New(), actor.ID)

	if err := recorder.Record(t.Context(), unknown); err == nil {
		t.Fatal("Record accepted an actor with no user row")
	}

	if held := stored(t, recorder.db); len(held) != 0 {
		t.Errorf("The trail holds %d entries, want 0", len(held))
	}
}

func TestNewAuditRefusesNilDatabase(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewAudit accepted a nil database")
		}
	}()
	NewAudit(nil)
}
