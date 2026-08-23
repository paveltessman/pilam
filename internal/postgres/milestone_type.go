package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/paveltessman/pilam/internal/milestone"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/postgres/internal/sqlc"
)

var _ milestone.TypesStore = (*MilestoneTypes)(nil)

type MilestoneTypes struct {
	db *DB
}

func NewMilestoneTypes(db *DB) *MilestoneTypes {
	if db == nil {
		panic("postgres: nil database")
	}
	return &MilestoneTypes{db: db}
}

// ByID returns the type, or milestone.ErrNoType when there is no such row.
func (m *MilestoneTypes) ByID(ctx context.Context, id ids.ID) (milestone.Type, error) {
	row, err := m.db.queries(ctx).GetMilestoneType(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return milestone.Type{}, fmt.Errorf("%w: id %s", milestone.ErrNoType, id)
		}
		return milestone.Type{}, fmt.Errorf("postgres: loading milestone type %s: %w", id, err)
	}
	return milestoneType(row), nil
}

// List returns every type, active and inactive, ordered by short name.
func (m *MilestoneTypes) List(ctx context.Context) ([]milestone.Type, error) {
	rows, err := m.db.queries(ctx).ListMilestoneTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: listing milestone types: %w", err)
	}

	types := make([]milestone.Type, len(rows))
	for i, row := range rows {
		types[i] = milestoneType(row)
	}
	return types, nil
}

// Create writes one new row. A taken name returns milestone.ErrNameTaken.
func (m *MilestoneTypes) Create(ctx context.Context, in milestone.Type) error {
	params := sqlc.CreateMilestoneTypeParams{
		ID:          in.ID,
		Name:        in.Name,
		Description: in.Description,
		Active:      in.Active,
	}

	if err := m.db.queries(ctx).CreateMilestoneType(ctx, params); err != nil {
		return fmt.Errorf("postgres: creating milestone type %s: %w", in.ID, duplicateError(err))
	}
	return nil
}

// Update writes every field back to the row.
func (m *MilestoneTypes) Update(ctx context.Context, in milestone.Type) error {
	params := sqlc.UpdateMilestoneTypeParams{
		ID:          in.ID,
		Name:        in.Name,
		Description: in.Description,
		Active:      in.Active,
	}

	written, err := m.db.queries(ctx).UpdateMilestoneType(ctx, params)
	if err != nil {
		return fmt.Errorf("postgres: updating milestone type %s: %w", in.ID, duplicateError(err))
	}
	if written == 0 {
		return fmt.Errorf("%w: id %s", milestone.ErrNoType, in.ID)
	}
	return nil
}

func milestoneType(row sqlc.MilestoneType) milestone.Type {
	return milestone.Type{
		ID:          row.ID,
		Name:        row.Name,
		Description: row.Description,
		Active:      row.Active,
	}
}
