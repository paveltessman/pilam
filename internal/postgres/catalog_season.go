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

var _ catalog.Seasons = (*Seasons)(nil)

type Seasons struct {
	db *DB
}

func NewSeasons(db *DB) *Seasons {
	if db == nil {
		panic("postgres: nil database")
	}
	return &Seasons{db: db}
}

// ByID returns the season, or catalog.ErrNoSeason when there is no such row.
func (s *Seasons) ByID(ctx context.Context, id ids.ID) (catalog.Season, error) {
	row, err := s.db.queries(ctx).GetSeason(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return catalog.Season{}, fmt.Errorf("%w: id %s", catalog.ErrNoSeason, id)
		}
		return catalog.Season{}, fmt.Errorf("postgres: loading season %s: %w", id, err)
	}
	return season(row), nil
}

// List returns every season, active and inactive, ordered by start date and
// then by name.
func (s *Seasons) List(ctx context.Context) ([]catalog.Season, error) {
	rows, err := s.db.queries(ctx).ListSeasons(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: listing seasons: %w", err)
	}

	seasons := make([]catalog.Season, len(rows))
	for i, row := range rows {
		seasons[i] = season(row)
	}
	return seasons, nil
}

// Create writes one new row. A taken name returns catalog.ErrSeasonNameTaken.
func (s *Seasons) Create(ctx context.Context, in catalog.Season) error {
	params := sqlc.CreateSeasonParams{
		ID:        in.ID,
		Name:      in.Name,
		StartDate: in.StartDate,
		Active:    in.Active,
	}

	if err := s.db.queries(ctx).CreateSeason(ctx, params); err != nil {
		return fmt.Errorf("postgres: creating season %s: %w", in.ID, catalogError(err))
	}
	return nil
}

// Update writes every field back to the row.
func (s *Seasons) Update(ctx context.Context, in catalog.Season) error {
	params := sqlc.UpdateSeasonParams{
		ID:        in.ID,
		Name:      in.Name,
		StartDate: in.StartDate,
		Active:    in.Active,
	}

	written, err := s.db.queries(ctx).UpdateSeason(ctx, params)
	if err != nil {
		return fmt.Errorf("postgres: updating season %s: %w", in.ID, catalogError(err))
	}
	if written == 0 {
		return fmt.Errorf("%w: id %s", catalog.ErrNoSeason, in.ID)
	}
	return nil
}

func season(row sqlc.Season) catalog.Season {
	return catalog.Season{
		ID:        row.ID,
		Name:      row.Name,
		StartDate: row.StartDate,
		Active:    row.Active,
	}
}
