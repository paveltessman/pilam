// Package clock is the one place the current time is read.
//
// Now is an instant — an audit timestamp, a session expiry.
// Today is a calendar day, and which day it is depends on the business timezone,
// not on the timezone of the machine the process happens to run on.
//
// Production uses New, tests and the seed use Fixed.
package clock

import (
	"errors"
	"time"

	"github.com/paveltessman/pilam/internal/platform/date"
)

type Clock interface {
	// Now returns the current instant, in the business timezone.
	Now() time.Time

	// Today returns the current business day, as UTC midnight.
	Today() time.Time
}

// New returns a clock reading the system time and reckoning days in tz, which
// comes from config.
func New(tz *time.Location) Clock {
	return systemClock{tz: checkLocation(tz)}
}

type systemClock struct{ tz *time.Location }

func (c systemClock) Now() time.Time   { return time.Now().In(c.tz) }
func (c systemClock) Today() time.Time { return day(c.Now()) }

// day is the calendar date t falls on in its own zone, as UTC midnight.
func day(t time.Time) time.Time { return date.Of(t.Date()) }

func checkLocation(tz *time.Location) *time.Location {
	if tz == nil {
		panic(errors.New("clock: nil timezone"))
	}
	return tz
}
