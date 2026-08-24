package milestones

import (
	"time"

	"github.com/paveltessman/pilam/internal/platform/date"
)

// DueWindow is how many days ahead a plan date reads as due.
const DueWindow = 7

// State is what one milestone reads as today. The app computes it on every read.
type State string

const (
	// StateDone holds a fact date.
	StateDone State = "done"

	// StateLate holds no fact date, and its plan date is before today.
	StateLate State = "late"

	// StateDue holds no fact date, and its plan date falls inside the next
	// DueWindow days.
	StateDue State = "due"

	// StatePlanned is anything else.
	StatePlanned State = "planned"
)

// State reads the milestone against today, which is the current business day.
func (m Milestone) State(today time.Time) State {
	switch {
	case m.Done():
		return StateDone
	case date.Before(m.Plan, today):
		return StateLate
	case !date.After(m.Plan, date.AddDays(today, DueWindow)):
		return StateDue
	default:
		return StatePlanned
	}
}

// Slip is the days the plan date moved from the baseline. Positive means the
// step moved later than the calendar promised.
func (m Milestone) Slip() int { return date.Days(m.Baseline, m.Plan) }

// FactSlip is the slip that really happened: the days from the baseline to the
// fact date. It is zero for a step that holds no fact date.
func (m Milestone) FactSlip() int {
	if !m.Done() {
		return 0
	}
	return date.Days(m.Baseline, m.Fact)
}

// OnTime reports whether a done step landed no later than the baseline. A step
// that holds no fact date is not on time.
func (m Milestone) OnTime() bool {
	return m.Done() && !date.After(m.Fact, m.Baseline)
}
