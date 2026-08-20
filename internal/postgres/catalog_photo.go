package postgres

import (
	"context"
	"fmt"

	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/media"
	"github.com/paveltessman/pilam/internal/postgres/internal/sqlc"
)

var _ catalog.Photos = (*Photos)(nil)

type Photos struct {
	db *DB
}

func NewPhotos(db *DB) *Photos {
	if db == nil {
		panic("postgres: nil database")
	}
	return &Photos{db: db}
}

// ByModel returns the photo strip of one model, thumbnail first.
func (p *Photos) ByModel(ctx context.Context, modelID ids.ID) ([]catalog.Photo, error) {
	rows, err := p.db.queries(ctx).ListPhotosByModel(ctx, modelID)
	if err != nil {
		return nil, fmt.Errorf("postgres: listing the photos of model %s: %w", modelID, err)
	}

	photos := make([]catalog.Photo, len(rows))
	for i, row := range rows {
		photos[i] = photo(row)
	}
	return photos, nil
}

func (p *Photos) Add(ctx context.Context, in catalog.Photo) error {
	params := sqlc.CreatePhotoParams{
		ID:       in.ID,
		ModelID:  in.ModelID,
		MediaKey: string(in.MediaKey),
		Position: int32(in.Position),
	}

	if err := p.db.queries(ctx).CreatePhoto(ctx, params); err != nil {
		return fmt.Errorf("postgres: adding photo %s to model %s: %w", in.ID, in.ModelID, catalogError(err))
	}
	return nil
}

// Remove deletes one row, and catalog.ErrNoPhoto when there is nothing to delete.
func (p *Photos) Remove(ctx context.Context, photoID ids.ID) error {
	written, err := p.db.queries(ctx).DeletePhoto(ctx, photoID)
	if err != nil {
		return fmt.Errorf("postgres: removing photo %s: %w", photoID, err)
	}
	if written == 0 {
		return fmt.Errorf("%w: id %s", catalog.ErrNoPhoto, photoID)
	}
	return nil
}

// Reorder writes position i to the i-th identifier of ordered.
func (p *Photos) Reorder(ctx context.Context, ordered []ids.ID) error {
	if len(ordered) == 0 {
		return nil
	}

	write := func(ctx context.Context) error {
		queries := p.db.queries(ctx)
		for position, photoID := range ordered {
			params := sqlc.SetPhotoPositionParams{ID: photoID, Position: int32(position)}
			written, err := queries.SetPhotoPosition(ctx, params)
			if err != nil {
				return fmt.Errorf("postgres: moving photo %s to position %d: %w", photoID, position, err)
			}
			if written == 0 {
				return fmt.Errorf("%w: id %s", catalog.ErrNoPhoto, photoID)
			}
		}
		return nil
	}
	return p.db.InTx(ctx, write)
}

func photo(row sqlc.ModelPhoto) catalog.Photo {
	return catalog.Photo{
		ID:       row.ID,
		ModelID:  row.ModelID,
		MediaKey: media.Key(row.MediaKey),
		Position: int(row.Position),
	}
}
