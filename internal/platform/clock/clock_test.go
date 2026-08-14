package clock_test

import (
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/platform/clock"
)

// Tests build zones with time.FixedZone rather than time.LoadLocation.
// The clock does depends on the offset, not on the host having a zone database.
var utcPlus3 = time.FixedZone("UTC+3", 3*60*60)

func TestNewReadsTheSystemTimeInTheBusinessTimezone(t *testing.T) {
	before := time.Now()
	now := clock.New(utcPlus3).Now()
	after := time.Now()

	if now.Before(before) || now.After(after) {
		t.Errorf("Now = %s, want between %s and %s", now, before, after)
	}
	if now.Location() != utcPlus3 {
		t.Errorf("Now is in %s, want %s", now.Location(), utcPlus3)
	}
}

func TestNewTodayIsTheCurrentBusinessDay(t *testing.T) {
	before := businessDay(time.Now().In(utcPlus3))
	today := clock.New(utcPlus3).Today()
	after := businessDay(time.Now().In(utcPlus3))

	if today != before && today != after {
		t.Errorf("Today = %s, want %s or %s", today, before, after)
	}
}

func TestNewPanicsOnNilTimezone(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("New accepted a nil timezone")
		}
	}()
	clock.New(nil)
}

func businessDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
