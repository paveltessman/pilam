package milestones

import (
	"time"

	"github.com/paveltessman/pilam/internal/platform/date"
)

// This code owns the rule of one plan date: what moves with it.

// Move is one milestone a plan date change carries: the row as it stands, and
// the plan date it takes.
type Move struct {
	Milestone Milestone
	Plan      time.Time
}

// Days is how far the row moves. It is positive for a row that moves later.
func (m Move) Days() int { return date.Days(m.Milestone.Plan, m.Plan) }

// Cascade is the rule: a plan date that moves later pushes every later
// milestone of the same model by the same number of days.
//
// held is the calendar of the model, moved is the row the user typed into, and
// plan is the date that row takes. The result names the moved row first, and
// then the rows the shift carries, in the order held holds them.
//
// Later means the plan date of a row is after the date the moved row leaves. A
// row that holds a fact date does not move, because work that is done does not
// move, and neither does an inactive row.
//
// A plan date that moves earlier moves its own row alone. Pulling a whole chain
// forward is a plan, not a side effect.
//
// It returns no move at all for a date that does not move.
func Cascade(held []Milestone, moved Milestone, plan time.Time) []Move {
	plan = date.Of(plan.Date())
	days := date.Days(moved.Plan, plan)
	if days == 0 {
		return nil
	}

	moves := []Move{{Milestone: moved, Plan: plan}}
	if days < 0 {
		return moves
	}
	for _, one := range held {
		if one.ID == moved.ID || !one.Active || one.Done() {
			continue
		}
		if !date.After(one.Plan, moved.Plan) {
			continue
		}
		moves = append(moves, Move{Milestone: one, Plan: date.AddDays(one.Plan, days)})
	}
	return moves
}
