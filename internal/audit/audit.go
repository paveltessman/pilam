// Package audit holds the trail of who changed what, and when.
//
// A domain states a Change. The Trail turns it into an Entry, stamped with the
// time, the actor and the request in flight, and the Recorder stores it.
package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/paveltessman/pilam/internal/platform/clock"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/requestid"
)

// EntityUser is the entity name the user rows are recorded under.
const EntityUser = "user"

// The actions the trail names.
const (
	ActionCreated       = "created"
	ActionRoleChanged   = "role_changed"
	ActionDeactivated   = "deactivated"
	ActionReactivated   = "reactivated"
	ActionPasswdChanged = "passwd_changed"
)

// Marker stands in for a value the trail must not hold, such as a password. The
// entry then records that the field changed, and nothing about the value.
const Marker = "set"

// Change is one fact a domain states: this action on this entity, and the value
// the named field moved between.
//
// Old and New are empty where the change has no value on that side. A created
// entity has neither, and a password change has the Marker alone.
type Change struct {
	Entity   string
	EntityID ids.ID
	Action   string
	FieldKey string
	Old      string
	New      string
}

// Entry is one row of the trail: a Change, with the fields the Trail stamps
// onto it.
type Entry struct {
	ID        ids.ID
	At        time.Time
	ActorID   ids.ID
	Entity    string
	EntityID  ids.ID
	Action    string
	FieldKey  string
	Old       string
	New       string
	RequestID string
}

// Recorder appends entries to the trail.
type Recorder interface {
	Record(ctx context.Context, entries ...Entry) error
}

// Trail turns the changes a domain states into entries and hands them to the
// Recorder.
type Trail struct {
	recorder Recorder
	clock    clock.Clock
	ids      ids.Generator
}

func NewTrail(recorder Recorder, clk clock.Clock, gen ids.Generator) *Trail {
	switch {
	case recorder == nil:
		panic("audit: nil recorder")
	case clk == nil:
		panic("audit: nil clock")
	case gen == nil:
		panic("audit: nil id generator")
	}
	return &Trail{recorder: recorder, clock: clk, ids: gen}
}

// Record writes one entry per change, and nothing at all for no changes.
//
// Must be called inside the transaction that carries the write it describes. A
// write which rolls back then leaves no entry behind.
func (t *Trail) Record(ctx context.Context, actor ids.ID, changes ...Change) error {
	if len(changes) == 0 {
		return nil
	}
	if actor == ids.Nil {
		return fmt.Errorf("audit: recording %s: no actor", changes[0].Action)
	}

	at := t.clock.Now()
	request := requestid.FromContext(ctx)

	entries := make([]Entry, 0, len(changes))
	for _, change := range changes {
		if err := change.check(); err != nil {
			return err
		}
		entry := Entry{
			ID:        t.ids.New(),
			At:        at,
			ActorID:   actor,
			Entity:    change.Entity,
			EntityID:  change.EntityID,
			Action:    change.Action,
			FieldKey:  change.FieldKey,
			Old:       change.Old,
			New:       change.New,
			RequestID: request,
		}
		entries = append(entries, entry)
	}

	if err := t.recorder.Record(ctx, entries...); err != nil {
		return fmt.Errorf("audit: recording %d entries: %w", len(entries), err)
	}
	return nil
}

// check refuses a change that names no entity or no action.
func (c Change) check() error {
	switch {
	case c.Entity == "":
		return fmt.Errorf("audit: change on %s: no entity", c.EntityID)
	case c.EntityID == ids.Nil:
		return fmt.Errorf("audit: change on the %s entity: no entity id", c.Entity)
	case c.Action == "":
		return fmt.Errorf("audit: change on %s %s: no action", c.Entity, c.EntityID)
	}
	return nil
}
