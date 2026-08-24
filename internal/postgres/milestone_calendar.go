package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/paveltessman/pilam/internal/milestones"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/postgres/internal/sqlc"
)

var _ milestones.MilestonesStore = (*Milestones)(nil)

// Milestones is the store of the calendar of a model.
type Milestones struct {
	db *DB
}

func NewMilestones(db *DB) *Milestones {
	if db == nil {
		panic("postgres: nil database")
	}
	return &Milestones{db: db}
}

// ByID returns the milestone, or milestones.ErrNoMilestone when there is no
// such row.
func (m *Milestones) ByID(ctx context.Context, id ids.ID) (milestones.Milestone, error) {
	row, err := m.db.queries(ctx).GetMilestone(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return milestones.Milestone{}, fmt.Errorf("%w: id %s", milestones.ErrNoMilestone, id)
		}
		return milestones.Milestone{}, fmt.Errorf("postgres: loading milestone %s: %w", id, err)
	}
	return milestone(row), nil
}

// ByModel returns the milestones of one model, active and inactive, in plan
// date order.
func (m *Milestones) ByModel(ctx context.Context, modelID ids.ID) ([]milestones.Milestone, error) {
	rows, err := m.db.queries(ctx).ListMilestonesByModel(ctx, modelID)
	if err != nil {
		return nil, fmt.Errorf("postgres: listing the milestones of model %s: %w", modelID, err)
	}

	calendar := make([]milestones.Milestone, len(rows))
	for i, row := range rows {
		calendar[i] = milestone(row)
	}
	return calendar, nil
}

// Create writes every milestone it is given. A type the model already holds
// returns milestones.ErrTypeOnModel.
//
// The caller runs this inside a transaction, so one refused row leaves none of
// them behind.
func (m *Milestones) Create(ctx context.Context, in ...milestones.Milestone) error {
	for _, one := range in {
		params := sqlc.CreateMilestoneParams{
			ID:           one.ID,
			ModelID:      one.ModelID,
			TypeID:       one.TypeID,
			BaselineDate: one.Baseline,
			PlanDate:     one.Plan,
			FactDate:     factDate(one.Fact),
			Note:         one.Note,
			Active:       one.Active,
		}

		if err := m.db.queries(ctx).CreateMilestone(ctx, params); err != nil {
			return fmt.Errorf("postgres: creating milestone %s of model %s: %w",
				one.ID, one.ModelID, duplicateError(err))
		}
	}
	return nil
}

// Update writes every field back to the row.
func (m *Milestones) Update(ctx context.Context, in milestones.Milestone) error {
	params := sqlc.UpdateMilestoneParams{
		ID:           in.ID,
		BaselineDate: in.Baseline,
		PlanDate:     in.Plan,
		FactDate:     factDate(in.Fact),
		Note:         in.Note,
		Active:       in.Active,
	}

	written, err := m.db.queries(ctx).UpdateMilestone(ctx, params)
	if err != nil {
		return fmt.Errorf("postgres: updating milestone %s: %w", in.ID, duplicateError(err))
	}
	if written == 0 {
		return fmt.Errorf("%w: id %s", milestones.ErrNoMilestone, in.ID)
	}
	return nil
}

// Target returns the target date of the drop that holds the model, or
// milestones.ErrNoModel when there is no such model.
func (m *Milestones) Target(ctx context.Context, modelID ids.ID) (time.Time, error) {
	target, err := m.db.queries(ctx).GetModelTargetDate(ctx, modelID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, fmt.Errorf("%w: id %s", milestones.ErrNoModel, modelID)
		}
		return time.Time{}, fmt.Errorf("postgres: loading the target date of model %s: %w", modelID, err)
	}
	return target, nil
}

func milestone(row sqlc.Milestone) milestones.Milestone {
	one := milestones.Milestone{
		ID:       row.ID,
		ModelID:  row.ModelID,
		TypeID:   row.TypeID,
		Baseline: row.BaselineDate,
		Plan:     row.PlanDate,
		Note:     row.Note,
		Active:   row.Active,
	}
	if row.FactDate != nil {
		one.Fact = *row.FactDate
	}
	return one
}

// factDate is the fact date as the column holds it: a day, or nothing at all
// for a step that is not done.
func factDate(fact time.Time) *time.Time {
	if fact.IsZero() {
		return nil
	}
	return &fact
}
