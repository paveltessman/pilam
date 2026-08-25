package milestones

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/platform/date"
	"github.com/paveltessman/pilam/internal/platform/ids"
)

// step is one row of a calendar the rule reads: the number a test names it by,
// and the plan date it stands on. It is active and it holds no fact date.
func step(n int, plan string) Milestone {
	return Milestone{
		ID:     ids.MustParse(fmt.Sprintf("01912345-0000-7000-8000-%012d", n)),
		Plan:   date.MustParse(plan),
		Active: true,
	}
}

// aCalendar is the calendar every test of the rule moves a row of. Two steps
// stand on the same day, which is the tie the rule leaves where it is.
func aCalendar() []Milestone {
	return []Milestone{
		step(1, "2026-02-16"),
		step(2, "2026-03-02"),
		step(3, "2026-03-02"),
		step(4, "2026-04-01"),
	}
}

// movedTo is the identifier of every row a rule moves, with the date it takes.
func movedTo(moves []Move) []string {
	out := make([]string, len(moves))
	for i, move := range moves {
		out[i] = move.Milestone.ID.String() + " " + date.ISO(move.Plan)
	}
	return out
}

func TestCascadePushesEveryLaterStepByTheSameDays(t *testing.T) {
	held := aCalendar()
	moved := held[1]

	moves := Cascade(held, moved, date.MustParse("2026-03-12"))

	// The moved row comes first. The step on the same day is not later than the
	// row the user moved, and the step above it is earlier, so neither one goes.
	want := []string{
		moved.ID.String() + " 2026-03-12",
		held[3].ID.String() + " 2026-04-11",
	}
	if got := movedTo(moves); !slices.Equal(got, want) {
		t.Errorf("Cascade = %v, want %v", got, want)
	}
	for i, move := range moves {
		if move.Days() != 10 {
			t.Errorf("move %d = %d days, want 10", i, move.Days())
		}
	}
}

func TestCascadeSkipsAStepThatHoldsAFactDate(t *testing.T) {
	held := aCalendar()
	held[3].Fact = date.MustParse("2026-03-30")
	moved := held[1]

	moves := Cascade(held, moved, date.MustParse("2026-03-12"))

	want := []string{moved.ID.String() + " 2026-03-12"}
	if got := movedTo(moves); !slices.Equal(got, want) {
		t.Errorf("Cascade = %v, want %v: work that is done does not move", got, want)
	}
}

func TestCascadeSkipsAnInactiveStep(t *testing.T) {
	held := aCalendar()
	held[3].Active = false
	moved := held[1]

	moves := Cascade(held, moved, date.MustParse("2026-03-12"))

	want := []string{moved.ID.String() + " 2026-03-12"}
	if got := movedTo(moves); !slices.Equal(got, want) {
		t.Errorf("Cascade = %v, want %v: a step off the calendar does not move", got, want)
	}
}

func TestCascadeMovesTheRowAloneWhenTheDateMovesEarlier(t *testing.T) {
	held := aCalendar()
	moved := held[1]

	moves := Cascade(held, moved, date.MustParse("2026-02-01"))

	want := []string{moved.ID.String() + " 2026-02-01"}
	if got := movedTo(moves); !slices.Equal(got, want) {
		t.Errorf("Cascade = %v, want %v: an earlier date moves its own row", got, want)
	}
	if days := moves[0].Days(); days != -29 {
		t.Errorf("the move = %d days, want -29", days)
	}
}

func TestCascadeMovesNothingWhenTheDateDoesNotMove(t *testing.T) {
	held := aCalendar()

	if moves := Cascade(held, held[1], held[1].Plan); len(moves) != 0 {
		t.Errorf("Cascade = %v, want none", movedTo(moves))
	}
}

func TestCascadeReadsTheDayADateFallsOn(t *testing.T) {
	held := aCalendar()
	moved := held[1]

	// A value that carries a time of day still names the day it falls on.
	stamped := date.AddDays(moved.Plan, 10).Add(13 * time.Hour)
	moves := Cascade(held, moved, stamped)

	want := []string{
		moved.ID.String() + " 2026-03-12",
		held[3].ID.String() + " 2026-04-11",
	}
	if got := movedTo(moves); !slices.Equal(got, want) {
		t.Errorf("Cascade = %v, want %v", got, want)
	}
}
