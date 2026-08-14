package clock_test

import (
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/platform/clock"
)

var utcMinus5 = time.FixedZone("EST", -5*60*60)

func TestFixedStaysAtTheGivenInstant(t *testing.T) {
	at := time.Date(2026, time.March, 10, 9, 30, 0, 0, time.UTC)

	c := clock.Fixed(at, utcPlus3)

	for range 3 {
		if got := c.Now(); !got.Equal(at) {
			t.Errorf("Now = %s, want %s", got, at)
		}
	}
	if got := c.Now().Location(); got != utcPlus3 {
		t.Errorf("Now is in %s, want %s", got, utcPlus3)
	}
}

func TestFixedTodayIsTheBusinessDayNotTheUTCDay(t *testing.T) {
	for name, tc := range map[string]struct {
		at        time.Time
		tz        *time.Location
		wantToday time.Time
		wantUTC   time.Time
	}{
		"zone ahead, small hours": {
			// 01:30 in UTC+3 on the 1st is still 22:30 UTC on the 31st.
			at:        time.Date(2025, time.December, 31, 22, 30, 0, 0, time.UTC),
			tz:        utcPlus3,
			wantToday: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
			wantUTC:   time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC),
		},
		"zone behind, late evening": {
			// 20:00 in UTC-5 on the 1st is already 01:00 UTC on the 2nd.
			at:        time.Date(2026, time.January, 2, 1, 0, 0, 0, time.UTC),
			tz:        utcMinus5,
			wantToday: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
			wantUTC:   time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC),
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := clock.Fixed(tc.at, tc.tz).Today()

			if !got.Equal(tc.wantToday) {
				t.Errorf("Today = %s, want %s", got, tc.wantToday)
			}
			if got.Equal(tc.wantUTC) {
				t.Errorf("Today = %s, which is the UTC day, not the business day", got)
			}
		})
	}
}

// Days are carried as UTC midnight
func TestFixedTodayIsUTCMidnight(t *testing.T) {
	at := time.Date(2026, time.March, 10, 16, 45, 12, 999, time.UTC)

	today := clock.Fixed(at, utcPlus3).Today()

	if today.Location() != time.UTC {
		t.Errorf("Today is in %s, want UTC", today.Location())
	}
	if h, m, s := today.Clock(); h != 0 || m != 0 || s != 0 || today.Nanosecond() != 0 {
		t.Errorf("Today = %s, want midnight", today)
	}
}

func TestFixedAdvanceMovesTheClock(t *testing.T) {
	// 23:00 in UTC+3: seven hours tips the business day over, ten hours back
	// returns to it.
	at := time.Date(2026, time.March, 10, 20, 0, 0, 0, time.UTC)
	tenth := time.Date(2026, time.March, 10, 0, 0, 0, 0, time.UTC)
	eleventh := time.Date(2026, time.March, 11, 0, 0, 0, 0, time.UTC)

	c := clock.Fixed(at, utcPlus3)

	if got := c.Today(); !got.Equal(tenth) {
		t.Fatalf("Today = %s, want %s", got, tenth)
	}

	c.Advance(7 * time.Hour)
	if got := c.Now(); !got.Equal(at.Add(7 * time.Hour)) {
		t.Errorf("Now = %s, want %s", got, at.Add(7*time.Hour))
	}
	if got := c.Today(); !got.Equal(eleventh) {
		t.Errorf("Today = %s, want %s", got, eleventh)
	}

	c.Advance(-10 * time.Hour)
	if got := c.Today(); !got.Equal(tenth) {
		t.Errorf("after moving back, Today = %s, want %s", got, tenth)
	}
}

func TestFixedPanicsOnNilTimezone(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Fixed accepted a nil timezone")
		}
	}()
	clock.Fixed(time.Now(), nil)
}
