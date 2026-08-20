package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/postgres/internal/sqlc"
)

var _ catalog.Drops = (*Drops)(nil)

type Drops struct {
	db *DB
}

func NewDrops(db *DB) *Drops {
	if db == nil {
		panic("postgres: nil database")
	}
	return &Drops{db: db}
}

// ByID returns the drop, or catalog.ErrNoDrop when there is no such row.
func (d *Drops) ByID(ctx context.Context, id ids.ID) (catalog.Drop, error) {
	row, err := d.db.queries(ctx).GetDrop(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return catalog.Drop{}, fmt.Errorf("%w: id %s", catalog.ErrNoDrop, id)
		}
		return catalog.Drop{}, fmt.Errorf("postgres: loading drop %s: %w", id, err)
	}
	return drop(row), nil
}

// ListBySeason returns the drops of one season, active and inactive, ordered by
// target date and then by name.
func (d *Drops) ListBySeason(ctx context.Context, seasonID ids.ID) ([]catalog.Drop, error) {
	rows, err := d.db.queries(ctx).ListDropsBySeason(ctx, seasonID)
	if err != nil {
		return nil, fmt.Errorf("postgres: listing the drops of season %s: %w", seasonID, err)
	}

	drops := make([]catalog.Drop, len(rows))
	for i, row := range rows {
		drops[i] = drop(row)
	}
	return drops, nil
}

// Create writes one new row. A name the season already holds returns
// catalog.ErrDropNameTaken.
func (d *Drops) Create(ctx context.Context, in catalog.Drop) error {
	params := sqlc.CreateDropParams{
		ID:         in.ID,
		SeasonID:   in.SeasonID,
		Name:       in.Name,
		TargetDate: in.TargetDate,
		Active:     in.Active,
	}

	if err := d.db.queries(ctx).CreateDrop(ctx, params); err != nil {
		return fmt.Errorf("postgres: creating drop %s: %w", in.ID, catalogError(err))
	}
	return nil
}

// Update writes back the fields a drop can change. The season is not one of
// them: the statement leaves it as it stands.
func (d *Drops) Update(ctx context.Context, in catalog.Drop) error {
	params := sqlc.UpdateDropParams{
		ID:         in.ID,
		Name:       in.Name,
		TargetDate: in.TargetDate,
		Active:     in.Active,
	}

	written, err := d.db.queries(ctx).UpdateDrop(ctx, params)
	if err != nil {
		return fmt.Errorf("postgres: updating drop %s: %w", in.ID, catalogError(err))
	}
	if written == 0 {
		return fmt.Errorf("%w: id %s", catalog.ErrNoDrop, in.ID)
	}
	return nil
}

func drop(row sqlc.Drop) catalog.Drop {
	drop := catalog.Drop{
		ID:         row.ID,
		SeasonID:   row.SeasonID,
		Name:       row.Name,
		TargetDate: row.TargetDate,
		Active:     row.Active,
	}
	return drop
}
