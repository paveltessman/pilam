package audit

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/platform/clock"
	"github.com/paveltessman/pilam/internal/platform/ids"
)

// fakeReader hands back the entries the test put in it, and keeps the limit it
// was asked for.
type fakeReader struct {
	entries  []Entry
	limit    int
	failWith error
}

func (f *fakeReader) ByEntity(_ context.Context, _ string, _ ids.ID, limit int) ([]Entry, error) {
	f.limit = limit
	if f.failWith != nil {
		return nil, f.failWith
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
