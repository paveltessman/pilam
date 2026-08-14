package date_test

import (
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/platform/date"
)

// The invariant everything else rests on.
func TestOfIsUTCMidnight(t *testing.T) {
	d := date.Of(2026, time.March, 10)

	if d.Location() != time.UTC {
		t.Errorf("location is %s, want UTC", d.Location())
	}
	if h, m, s := d.Clock(); h != 0 || m != 0 || s != 0 || d.Nanosecond() != 0 {
		t.Errorf("Of = %s, want midnight", d)
	}
}

func TestOfRollsOverOutOfRangeComponents(t *testing.T) {
	for name, tc := range map[string]struct{ got, want time.Time }{
		"month 13": {date.Of(2026, 13, 1), date.Of(2027, time.January, 1)},
		"day 32":   {date.Of(2026, time.January, 32), date.Of(2026, time.February, 1)},
		"day 0":    {date.Of(2026, time.March, 0), date.Of(2026, time.February, 28)},
	} {
		t.Run(name, func(t *testing.T) {
			if !tc.got.Equal(tc.want) {
				t.Errorf("got %s, want %s", tc.got, tc.want)
			}
		})
	}
}

func TestParseReadsTheWireForm(t *testing.T) {
	got, err := date.Parse("2026-03-10")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if want := date.Of(2026, time.March, 10); !got.Equal(want) {
		t.Errorf("Parse = %s, want %s", got, want)
	}
	if got.Location() != time.UTC {
		t.Errorf("Parse returned a %s value, want UTC", got.Location())
	}
}

func TestParseRejects(t *testing.T) {
	for name, in := range map[string]string{
		"empty":            "",
		"display form":     "10.03.2026",
		"unpadded":         "2026-3-10",
		"with a time":      "2026-03-10T00:00:00Z",
		"trailing space":   "2026-03-10 ",
		"day out of range": "2026-02-30",
		"slashes":          "2026/03/10",
		"words":            "tomorrow",
	} {
		t.Run(name, func(t *testing.T) {
			got, err := date.Parse(in)
			if err == nil {
				t.Fatalf("Parse(%q) = %s, want an error", in, got)
			}
			if !got.IsZero() {
				t.Errorf("Parse(%q) returned %s beside the error, want the zero time", in, got)
			}
		})
	}
}

func TestMustParsePanicsOnJunk(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("MustParse accepted a value Parse rejects")
		}
	}()
	date.MustParse("10.03.2026")
}

func TestFormatAndISO(t *testing.T) {
	d := date.Of(2026, time.March, 4)

	if got, want := date.Format(d), "04.03.2026"; got != want {
		t.Errorf("Format = %q, want %q", got, want)
	}
	if got, want := date.ISO(d), "2026-03-04"; got != want {
		t.Errorf("ISO = %q, want %q", got, want)
	}
}

func TestISORoundTripsParse(t *testing.T) {
	const in = "2027-11-30"

	if got := date.ISO(date.MustParse(in)); got != in {
		t.Errorf("ISO(MustParse(%q)) = %q", in, got)
	}
}

// A timestamp that never went through Of is reported as the day it falls on in
// its own zone — 01:00 in UTC+3 is the 10th there, though it is the 9th in UTC.
func TestHelpersUseTheValuesOwnCalendarDate(t *testing.T) {
	tz := time.FixedZone("UTC+3", 3*60*60)
	at := time.Date(2026, time.March, 10, 1, 0, 0, 0, tz)

	if got, want := date.Format(at), "10.03.2026"; got != want {
		t.Errorf("Format = %q, want %q", got, want)
	}
	if got := date.Days(date.Of(2026, time.March, 10), at); got != 0 {
		t.Errorf("Days to the same calendar day = %d, want 0", got)
	}
}

func TestDays(t *testing.T) {
	for name, tc := range map[string]struct {
		from, to time.Time
		want     int
	}{
		"same day":         {date.Of(2026, time.March, 10), date.Of(2026, time.March, 10), 0},
		"forward":          {date.Of(2026, time.March, 10), date.Of(2026, time.March, 17), 7},
		"backward":         {date.Of(2026, time.March, 17), date.Of(2026, time.March, 10), -7},
		"across a month":   {date.Of(2026, time.January, 28), date.Of(2026, time.February, 3), 6},
		"across a year":    {date.Of(2025, time.December, 30), date.Of(2026, time.January, 2), 3},
		"across leap day":  {date.Of(2028, time.February, 28), date.Of(2028, time.March, 1), 2},
		"a transit offset": {date.Of(2026, time.March, 10), date.Of(2026, time.January, 9), -60},
	} {
		t.Run(name, func(t *testing.T) {
			if got := date.Days(tc.from, tc.to); got != tc.want {
				t.Errorf("Days = %d, want %d", got, tc.want)
			}
		})
	}
}

// Times of day cannot make a whole-day count come out fractional and truncate
// to the wrong number: 23:00 to 01:00 the next morning is one day.
func TestDaysIgnoresTimeOfDay(t *testing.T) {
	from := time.Date(2026, time.March, 10, 23, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.March, 11, 1, 0, 0, 0, time.UTC)

	if got := date.Days(from, to); got != 1 {
		t.Errorf("Days = %d, want 1", got)
	}
	if got := date.Days(to, from); got != -1 {
		t.Errorf("Days reversed = %d, want -1", got)
	}
}

func TestAddDays(t *testing.T) {
	base := date.Of(2026, time.March, 10)

	for name, tc := range map[string]struct {
		n    int
		want time.Time
	}{
		"none":           {0, base},
		"forward":        {7, date.Of(2026, time.March, 17)},
		"back":           {-14, date.Of(2026, time.February, 24)},
		"over a month":   {30, date.Of(2026, time.April, 9)},
		"over a year":    {-90, date.Of(2025, time.December, 10)},
		"transit offset": {-60, date.Of(2026, time.January, 9)},
	} {
		t.Run(name, func(t *testing.T) {
			got := date.AddDays(base, tc.n)
			if !got.Equal(tc.want) {
				t.Errorf("AddDays(%d) = %s, want %s", tc.n, got, tc.want)
			}
			if got.Location() != time.UTC {
				t.Errorf("AddDays returned a %s value, want UTC", got.Location())
			}
		})
	}
}

// Propagation applies an offset and later measures it; the two have to agree.
func TestAddDaysAndDaysAreInverse(t *testing.T) {
	base := date.Of(2026, time.July, 1)

	for _, n := range []int{-365, -60, -45, -30, -14, -1, 0, 1, 7, 45, 200} {
		if got := date.Days(base, date.AddDays(base, n)); got != n {
			t.Errorf("Days(base, AddDays(base, %d)) = %d", n, got)
		}
	}
}

func TestComparisons(t *testing.T) {
	early := date.Of(2026, time.March, 10)
	late := date.Of(2026, time.March, 11)
	// Same day as early, but a timestamp late in the evening.
	sameDay := time.Date(2026, time.March, 10, 23, 59, 0, 0, time.UTC)

	for name, tc := range map[string]struct{ got, want bool }{
		"before":                    {date.Before(early, late), true},
		"before, reversed":          {date.Before(late, early), false},
		"before is strict":          {date.Before(early, sameDay), false},
		"after":                     {date.After(late, early), true},
		"after, reversed":           {date.After(early, late), false},
		"after is strict":           {date.After(early, sameDay), false},
		"equal":                     {date.Equal(early, sameDay), true},
		"equal across a day change": {date.Equal(early, late), false},
	} {
		t.Run(name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("got %t, want %t", tc.got, tc.want)
			}
		})
	}
}
