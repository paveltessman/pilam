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

var _ catalog.Models = (*Models)(nil)

type Models struct {
	db *DB
}

func NewModels(db *DB) *Models {
	if db == nil {
		panic("postgres: nil database")
	}
	return &Models{db: db}
}

// ByID returns the model, or catalog.ErrNoModel when there is no such row.
func (m *Models) ByID(ctx context.Context, id ids.ID) (catalog.Model, error) {
	row, err := m.db.queries(ctx).GetModel(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return catalog.Model{}, fmt.Errorf("%w: id %s", catalog.ErrNoModel, id)
		}
		return catalog.Model{}, fmt.Errorf("postgres: loading model %s: %w", id, err)
	}
	return model(row), nil
}

// List returns the models the filter keeps, ordered by article.
func (m *Models) List(ctx context.Context, filter catalog.ModelListParams) ([]catalog.Model, error) {
	params := sqlc.ListModelsParams{
		SeasonID: identifier(filter.SeasonID),
		DropID:   identifier(filter.DropID),
		Active:   filter.Active,
	}

	rows, err := m.db.queries(ctx).ListModels(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("postgres: listing models: %w", err)
	}

	models := make([]catalog.Model, len(rows))
	for i, row := range rows {
		models[i] = model(row)
	}
	return models, nil
}

func (m *Models) Create(ctx context.Context, in catalog.Model) error {
	params := sqlc.CreateModelParams{
		ID:      in.ID,
		DropID:  in.DropID,
		Article: in.Article,
		Active:  in.Active,
	}

	if err := m.db.queries(ctx).CreateModel(ctx, params); err != nil {
		return fmt.Errorf("postgres: creating model %s: %w", in.ID, catalogError(err))
	}
	return nil
}

// Update writes back the fields a model can change.
func (m *Models) Update(ctx context.Context, in catalog.Model) error {
	params := sqlc.UpdateModelParams{
		ID:      in.ID,
		Article: in.Article,
		Active:  in.Active,
	}

	written, err := m.db.queries(ctx).UpdateModel(ctx, params)
	if err != nil {
		return fmt.Errorf("postgres: updating model %s: %w", in.ID, catalogError(err))
	}
	if written == 0 {
		return fmt.Errorf("%w: id %s", catalog.ErrNoModel, in.ID)
	}
	return nil
}

func model(row sqlc.Model) catalog.Model {
	return catalog.Model{
		ID:      row.ID,
		DropID:  row.DropID,
		Article: row.Article,
		Active:  row.Active,
	}
}
