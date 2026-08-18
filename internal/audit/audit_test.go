package audit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/platform/clock"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/requestid"
)

var (
	actorID  = ids.MustParse("01912345-6789-7abc-def0-1234567890a1")
	targetID = ids.MustParse("01912345-6789-7abc-def0-1234567890a2")
)

var now = time.Date(2026, 8, 18, 9, 30, 0, 0, time.UTC)

// fakeRecorder keeps the entries in memory, and fails on demand.
type fakeRecorder struct {
	entries  []Entry
	calls    int
	failWith error
}

func (f *fakeRecorder) Record(_ context.Context, entries ...Entry) error {
	f.calls++
	if f.failWith != nil {
		return f.failWith
	}
	f.entries = append(f.entries, entries...)
	return nil
}

func newTestTrail(recorder Recorder) *Trail {
	return NewTrail(recorder, clock.Fixed(now, time.UTC), ids.NewDeterministic(1))
}

// roleChange is one complete change, as auth states it.
func roleChange() Change {
	change := Change{
		Entity:   EntityUser,
		EntityID: targetID,
		Action:   ActionRoleChanged,
		FieldKey: "role",
		Old:      "member",
		New:      "root",
	}
	return change
}

func TestRecordStampsTheEntry(t *testing.T) {
	recorder := &fakeRecorder{}
	ctx := requestid.NewContext(t.Context(), "req-7")

	if err := newTestTrail(recorder).Record(ctx, actorID, roleChange()); err != nil {
		t.Fatalf("Record: %v", err)
	}

	if len(recorder.entries) != 1 {
		t.Fatalf("Recorded %d entries, want 1", len(recorder.entries))
	}
	got := recorder.entries[0]

	if got.ID == ids.Nil {
		t.Error("The entry carries no id")
	}
	if !got.At.Equal(now) {
		t.Errorf("At = %v, want %v", got.At, now)
	}
	if got.ActorID != actorID {
		t.Errorf("ActorID = %v, want %v", got.ActorID, actorID)
	}
	if got.RequestID != "req-7" {
		t.Errorf("RequestID = %q, want %q", got.RequestID, "req-7")
	}

	want := roleChange()
	if got.Entity != want.Entity || got.EntityID != want.EntityID || got.Action != want.Action ||
		got.FieldKey != want.FieldKey || got.Old != want.Old || got.New != want.New {
		t.Errorf("The entry lost the change: got=%+v, want=%+v", got, want)
	}
}

func TestRecordOutsideRequestCarriesNoRequestID(t *testing.T) {
	recorder := &fakeRecorder{}

	if err := newTestTrail(recorder).Record(t.Context(), actorID, roleChange()); err != nil {
		t.Fatalf("Record: %v", err)
	}

	if got := recorder.entries[0].RequestID; got != "" {
		t.Errorf("RequestID = %q, want the empty string", got)
	}
}

func TestRecordGivesEveryEntryItsOwnID(t *testing.T) {
	recorder := &fakeRecorder{}

	err := newTestTrail(recorder).Record(t.Context(), actorID, roleChange(), roleChange())
	if err != nil {
		t.Fatalf("Record: %v", err)
	}

	if len(recorder.entries) != 2 {
		t.Fatalf("Recorded %d entries, want 2", len(recorder.entries))
	}
	if recorder.entries[0].ID == recorder.entries[1].ID {
		t.Errorf("Both entries carry the id %v", recorder.entries[0].ID)
	}
}

func TestRecordWithoutChangesDoesNotReachRecorder(t *testing.T) {
	recorder := &fakeRecorder{}

	if err := newTestTrail(recorder).Record(t.Context(), actorID); err != nil {
		t.Fatalf("Record: %v", err)
	}

	if recorder.calls != 0 {
		t.Errorf("The recorder was called %d times, want 0", recorder.calls)
	}
}

func TestRecordRefusesIncompleteChange(t *testing.T) {
	cases := map[string]Change{
		"no entity":    {EntityID: targetID, Action: ActionCreated},
		"no entity id": {Entity: EntityUser, Action: ActionCreated},
		"no action":    {Entity: EntityUser, EntityID: targetID},
	}

	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			recorder := &fakeRecorder{}

			if err := newTestTrail(recorder).Record(t.Context(), actorID, change); err == nil {
				t.Fatal("Record accepted the change, want an error")
			}
			if recorder.calls != 0 {
				t.Errorf("The recorder was called %d times, want 0", recorder.calls)
			}
		})
	}
}

func TestRecordRefusesEntryWithoutActor(t *testing.T) {
	recorder := &fakeRecorder{}

	if err := newTestTrail(recorder).Record(t.Context(), ids.Nil, roleChange()); err == nil {
		t.Fatal("Record accepted the nil actor, want an error")
	}
	if recorder.calls != 0 {
		t.Errorf("The recorder was called %d times, want 0", recorder.calls)
	}
}

func TestRecordReportsWhatRecorderRefused(t *testing.T) {
	failure := errors.New("the store is down")
	recorder := &fakeRecorder{failWith: failure}

	err := newTestTrail(recorder).Record(t.Context(), actorID, roleChange())
	if !errors.Is(err, failure) {
		t.Errorf("Record = %v, want %v", err, failure)
	}
}

func TestNewTrailRefusesMissingCollaborators(t *testing.T) {
	clk := clock.Fixed(now, time.UTC)
	gen := ids.NewDeterministic(1)

	cases := map[string]func(){
		"no recorder": func() { NewTrail(nil, clk, gen) },
		"no clock":    func() { NewTrail(&fakeRecorder{}, nil, gen) },
		"no ids":      func() { NewTrail(&fakeRecorder{}, clk, nil) },
	}

	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("NewTrail returned, want a panic")
				}
			}()
			build()
		})
	}
}
