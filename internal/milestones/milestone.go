package milestones

import (
	"context"
	"time"

	"github.com/paveltessman/pilam/internal/platform/ids"
)

// Milestone is one step of one model: the type it runs, the three dates, and
// the note that says why a date moved.
type Milestone struct {
	ID      ids.ID
	ModelID ids.ID
	TypeID  ids.ID

	// Baseline is what the calendar promised, and Plan is what the team expects
	// now. A calendar starts with the two equal, and an ordinary edit moves the
	// plan alone.
	Baseline time.Time
	Plan     time.Time

	// Fact is the day the step was done. Zero until somebody stamps it.
	Fact time.Time
	Note string

	// Active is false for a milestone that left the calendar and stays in the
	// history.
	Active bool
}

// Done reports whether the step holds a fact date.
func (m Milestone) Done() bool { return !m.Fact.IsZero() }

// MilestonesStore is the store of the milestones of a model.
type MilestonesStore interface {
	// ByID returns ErrNoMilestone when nothing matches.
	ByID(ctx context.Context, id ids.ID) (Milestone, error)

	// ByModel returns the milestones of one model, active and inactive, in plan
	// date order.
	ByModel(ctx context.Context, modelID ids.ID) ([]Milestone, error)

	// Create writes every milestone it is given, or none of them. A type the
	// model already holds returns ErrTypeOnModel.
	Create(ctx context.Context, in ...Milestone) error

	// Update returns ErrNoMilestone when the row is gone.
	Update(ctx context.Context, in Milestone) error

	// Target returns the target date of the drop that holds the model. It
	// returns ErrNoModel when there is no such model.
	Target(ctx context.Context, modelID ids.ID) (time.Time, error)
}
