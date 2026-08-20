package catalog

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/media"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

// Photo is one image of a model.
type Photo struct {
	ID       ids.ID
	ModelID  ids.ID
	MediaKey media.Key
	Position int
}

// Photos is the store of photo rows.
type Photos interface {
	// ByModel returns the photo strip of one model, thumbnail first.
	ByModel(ctx context.Context, modelID ids.ID) ([]Photo, error)

	// Thumbnails returns the photo at position 0 of every model named, keyed
	// by the model. A model that holds no photo is not in the map.
	Thumbnails(ctx context.Context, modelIDs []ids.ID) (map[ids.ID]Photo, error)

	Add(ctx context.Context, photo Photo) error

	// Remove deletes one row. It returns ErrNoPhoto when the row is gone.
	Remove(ctx context.Context, photoID ids.ID) error

	// Reorder writes position i to the i-th identifier of ordered. The caller
	// states a whole strip.
	Reorder(ctx context.Context, ordered []ids.ID) error
}

// ListPhotos returns the photo strip of one model, thumbnail first.
func (s *Service) ListPhotos(ctx context.Context, modelID ids.ID) ([]Photo, error) {
	photos, err := s.photos.ByModel(ctx, modelID)
	if err != nil {
		return nil, fmt.Errorf("catalog: listing the photos of model %s: %w", modelID, err)
	}
	return photos, nil
}

// Thumbnails returns the cover of every model named, keyed by the model. A
// model that holds no photo is not in the map.
func (s *Service) Thumbnails(ctx context.Context, modelIDs ...ids.ID) (map[ids.ID]Photo, error) {
	if len(modelIDs) == 0 {
		return map[ids.ID]Photo{}, nil
	}

	covers, err := s.photos.Thumbnails(ctx, modelIDs)
	if err != nil {
		return nil, fmt.Errorf("catalog: listing the thumbnails of %d models: %w", len(modelIDs), err)
	}
	return covers, nil
}

// AddPhoto appends one photo to the strip of a model. The first photo of a
// model lands at position 0, which makes it the thumbnail.
func (s *Service) AddPhoto(ctx context.Context, modelID ids.ID, key media.Key) (Photo, error) {
	var v validate.Validator
	v.Check(key.Valid(), FieldPhoto, validate.NotAllowed)
	if err := v.Err(); err != nil {
		return Photo{}, err
	}

	if _, err := s.models.ByID(ctx, modelID); err != nil {
		return Photo{}, fmt.Errorf("catalog: loading model %s: %w", modelID, err)
	}

	var photo Photo
	write := func(ctx context.Context) error {
		held, err := s.photos.ByModel(ctx, modelID)
		if err != nil {
			return fmt.Errorf("catalog: listing the photos of model %s: %w", modelID, err)
		}

		photo = Photo{ID: s.ids.New(), ModelID: modelID, MediaKey: key, Position: len(held)}
		if err := s.photos.Add(ctx, photo); err != nil {
			return err
		}
		return s.record(ctx, change(audit.EntityModel, modelID, audit.ActionPhotoAdded, FieldPhoto, "", string(key)))
	}
	if err := s.atomic.InTx(ctx, write); err != nil {
		return Photo{}, err
	}
	return photo, nil
}

// RemovePhoto deletes one photo of a model and closes the gap it leaves, so
// position 0 still names the thumbnail.
func (s *Service) RemovePhoto(ctx context.Context, modelID, photoID ids.ID) error {
	write := func(ctx context.Context) error {
		held, err := s.photos.ByModel(ctx, modelID)
		if err != nil {
			return fmt.Errorf("catalog: listing the photos of model %s: %w", modelID, err)
		}

		at := slices.IndexFunc(held, func(p Photo) bool { return p.ID == photoID })
		if at < 0 {
			return fmt.Errorf("%w: id %s on model %s", ErrNoPhoto, photoID, modelID)
		}
		removed := held[at]

		if err := s.photos.Remove(ctx, photoID); err != nil {
			return err
		}

		left := slices.Delete(slices.Clone(held), at, at+1)
		if err := s.photos.Reorder(ctx, identifiers(left)); err != nil {
			return err
		}
		return s.record(ctx, change(audit.EntityModel, modelID, audit.ActionPhotoRemoved, FieldPhoto,
			string(removed.MediaKey), ""))
	}
	return s.atomic.InTx(ctx, write)
}

// ReorderPhotos writes the strip of a model in the stated order. The first
// identifier becomes the thumbnail.
//
// ordered must name every photo of the model exactly once, and nothing else.
// Anything else returns ErrPhotoOrder and writes nothing.
func (s *Service) ReorderPhotos(ctx context.Context, modelID ids.ID, ordered []ids.ID) error {
	write := func(ctx context.Context) error {
		held, err := s.photos.ByModel(ctx, modelID)
		if err != nil {
			return fmt.Errorf("catalog: listing the photos of model %s: %w", modelID, err)
		}

		was := identifiers(held)
		if !rearranges(was, ordered) {
			return fmt.Errorf("%w: model %s holds %d photos, the order names %d",
				ErrPhotoOrder, modelID, len(was), len(ordered))
		}
		if slices.Equal(was, ordered) {
			return nil
		}

		if err := s.photos.Reorder(ctx, ordered); err != nil {
			return err
		}
		return s.record(ctx, change(audit.EntityModel, modelID, audit.ActionPhotoReordered, FieldPhotoOrder,
			join(was), join(ordered)))
	}
	return s.atomic.InTx(ctx, write)
}

// rearranges reports whether ordered holds the same identifiers as was, each of
// them once.
func rearranges(was, ordered []ids.ID) bool {
	if len(was) != len(ordered) {
		return false
	}
	seen := make(map[ids.ID]bool, len(ordered))
	for _, id := range ordered {
		if seen[id] || !slices.Contains(was, id) {
			return false
		}
		seen[id] = true
	}
	return true
}

func identifiers(photos []Photo) []ids.ID {
	out := make([]ids.ID, len(photos))
	for i, photo := range photos {
		out[i] = photo.ID
	}
	return out
}

// join renders a strip for the trail: the identifiers, in order, comma separated.
func join(order []ids.ID) string {
	parts := make([]string, len(order))
	for i, id := range order {
		parts[i] = id.String()
	}
	return strings.Join(parts, ",")
}
