// Package date holds calendar dates: a day, with no time of day and no timezone.
//
// A date is a time.Time at UTC midnight.
//
// Pure — no clock, no I/O. The current day comes from clock.Today.
package date

import (
	"fmt"
	"time"
)

const (
	// The form for HTML's <input type="date"> submits and SQL.
	wire = time.DateOnly

	// The form for the UI: DD.MM.YYYY.
	display = "02.01.2006"
)

// Of returns the date y-m-d. Out-of-range components roll over the way
// time.Date normalizes them: month 13 is January of the next year.
func Of(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// Parse reads the wire form, 2006-01-02. Zero-padding is required and a time of
// day is not accepted, so anything this returns came from a date input.
func Parse(s string) (time.Time, error) {
	t, err := time.Parse(wire, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("not a date in YYYY-MM-DD form: %q", s)
	}
	return Of(t.Date()), nil
}

// MustParse is Parse for dates written into source: fixtures and seed constants.
// Panic means a typo.
func MustParse(s string) time.Time {
	d, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return d
}

// Format renders the display form, DD.MM.YYYY.
func Format(d time.Time) string { return day(d).Format(display) }

// ISO renders the wire form, for a date input's value and anything machine-read.
func ISO(d time.Time) string { return day(d).Format(wire) }

// Days is the signed count of days from one date to another: negative when "to"
// is the earlier of the two.
func Days(from, to time.Time) int {
	return int(day(to).Sub(day(from)) / (24 * time.Hour))
}

// AddDays moves a date by n days, Negative n moves it back.
func AddDays(d time.Time, n int) time.Time {
	y, m, dd := d.Date()
	return Of(y, m, dd+n)
}

func Equal(a, b time.Time) bool { return day(a).Equal(day(b)) }

func Before(a, b time.Time) bool { return day(a).Before(day(b)) }

func After(a, b time.Time) bool { return day(a).After(day(b)) }

// day is Of applied to a value's own calendar date. Every helper goes through
// it, so passing a timestamp that was never normalized still answers about the
// day that timestamp falls on, rather than being quietly off by one.
func day(t time.Time) time.Time { return Of(t.Date()) }
