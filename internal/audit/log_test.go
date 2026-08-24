package audit

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/platform/clock"
	"github.com/paveltessman/pilam/internal/platform/ids"
)

// fakeReader hands back the entries it holds for an entity, and keeps the limit
// it was asked for.
type fakeReader struct {
	// entries is what one call returns. byEntity answers per entity where the
	// test states more than one of them.
	entries  []Entry
	byEntity map[string][]Entry
	limit    int
	reads    int
	failWith error
}

func (f *fakeReader) ByEntities(_ context.Context, entity string, _ []ids.ID, limit int) ([]Entry, error) {
	f.limit = limit
	f.reads++
	if f.failWith != nil {
		return nil, f.failWith
	}
	if f.byEntity != nil {
		return slices.Clone(f.byEntity[entity]), nil
	}
	return slices.Clone(f.entries), nil
}

func newTestLog(reader Reader, tz *time.Location) *Log {
	return NewLog(reader, clock.Fixed(now, tz))
}

func TestLogHoldsTheLimitInRange(t *testing.T) {
	cases := []struct {
		name  string
		asked int
		want  int
	}{
		{"none stated", 0, DefaultLimit},
		{"below zero", -3, DefaultLimit},
		{"in range", 7, 7},
		{"above the most", MaxLimit + 1, MaxLimit},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reader := &fakeReader{}
			log := newTestLog(reader, time.UTC)

			if _, err := log.Entity(t.Context(), EntityUser, targetID, c.asked); err != nil {
				t.Fatalf("Entity: %v", err)
			}
			if reader.limit != c.want {
				t.Errorf("the store was asked for %d entries, want %d", reader.limit, c.want)
			}
		})
	}
}

// The store hands the time back in whichever zone it holds. The screens show
// the hour the change happened at in the business timezone, so the log moves it.
func TestLogStampsTheEntriesInTheBusinessZone(t *testing.T) {
	moscow, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("loading the zone: %v", err)
	}

	stored := Entry{ID: ids.MustParse("01912345-6789-7abc-def0-1234567890a3"), At: now}
	reader := &fakeReader{entries: []Entry{stored}}

	entries, err := newTestLog(reader, moscow).Entity(t.Context(), EntityUser, targetID, 0)
	if err != nil {
		t.Fatalf("Entity: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("the log returns %d entries, want 1", len(entries))
	}

	got := entries[0].At
	if !got.Equal(stored.At) {
		t.Errorf("At = %s, want the same instant as %s", got, stored.At)
	}
	if got.Location() != moscow {
		t.Errorf("At carries the zone %s, want %s", got.Location(), moscow)
	}
}

func TestLogReportsAReadFailure(t *testing.T) {
	failure := errors.New("the store is not there")
	log := newTestLog(&fakeReader{failWith: failure}, time.UTC)

	if _, err := log.Entity(t.Context(), EntityUser, targetID, 0); !errors.Is(err, failure) {
		t.Errorf("Entity: %v, want the failure of the store", err)
	}
}

// stampedAt is one entry of an entity, stamped at the minute given.
func stampedAt(entity string, minute int) Entry {
	return Entry{
		ID:       ids.MustParse("01912345-6789-7abc-def0-1234567890" + strconv.Itoa(10+minute)),
		At:       now.Add(time.Duration(minute) * time.Minute),
		Entity:   entity,
		EntityID: targetID,
	}
}

func TestScopesMergeTheHistoriesNewestFirst(t *testing.T) {
	reader := &fakeReader{byEntity: map[string][]Entry{
		EntityModel: {stampedAt(EntityModel, 4), stampedAt(EntityModel, 1)},
		EntityMilestone: {
			stampedAt(EntityMilestone, 5),
			stampedAt(EntityMilestone, 3),
			stampedAt(EntityMilestone, 2),
		},
	}}
	log := newTestLog(reader, time.UTC)

	entries, err := log.Scopes(t.Context(), 0,
		Scope{Entity: EntityModel, IDs: []ids.ID{targetID}},
		Scope{Entity: EntityMilestone, IDs: []ids.ID{targetID, actorID}},
	)
	if err != nil {
		t.Fatalf("Scopes: %v", err)
	}

	want := []string{
		EntityMilestone, EntityModel, EntityMilestone, EntityMilestone, EntityModel,
	}
	got := make([]string, len(entries))
	for i, entry := range entries {
		got[i] = entry.Entity
	}
	if !slices.Equal(got, want) {
		t.Errorf("the merged trail = %v, want %v", got, want)
	}
}

func TestScopesCutTheMergeToTheLimit(t *testing.T) {
	reader := &fakeReader{byEntity: map[string][]Entry{
		EntityModel:     {stampedAt(EntityModel, 4), stampedAt(EntityModel, 1)},
		EntityMilestone: {stampedAt(EntityMilestone, 5), stampedAt(EntityMilestone, 3)},
	}}
	log := newTestLog(reader, time.UTC)

	entries, err := log.Scopes(t.Context(), 3,
		Scope{Entity: EntityModel, IDs: []ids.ID{targetID}},
		Scope{Entity: EntityMilestone, IDs: []ids.ID{targetID}},
	)
	if err != nil {
		t.Fatalf("Scopes: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("the merged trail holds %d entries, want 3", len(entries))
	}
	if got := entries[0].Entity; got != EntityMilestone {
		t.Errorf("the newest entry is a %s, want a %s", got, EntityMilestone)
	}
}

func TestScopesReadPastAScopeThatNamesNoRow(t *testing.T) {
	reader := &fakeReader{}
	log := newTestLog(reader, time.UTC)

	entries, err := log.Scopes(t.Context(), 0, Scope{Entity: EntityMilestone})
	if err != nil {
		t.Fatalf("Scopes: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("the trail holds %+v, want nothing", entries)
	}
	if reader.reads != 0 {
		t.Errorf("the store was read %d times, want none", reader.reads)
	}
}

func TestNewLogRefusesMissingDependencies(t *testing.T) {
	cases := []struct {
		name   string
		reader Reader
		clk    clock.Clock
	}{
		{"no reader", nil, clock.Fixed(now, time.UTC)},
		{"no clock", &fakeReader{}, nil},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("NewLog did not panic")
				}
			}()
			NewLog(c.reader, c.clk)
		})
	}
}
