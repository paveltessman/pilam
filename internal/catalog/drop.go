package catalog

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/platform/date"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

// Drop is one delivery window inside a season.
type Drop struct {
	ID         ids.ID
	SeasonID   ids.ID
	Name       string
	TargetDate time.Time
	Active     bool
}

// Drops is the store of drop rows. ByID returns ErrNoDrop when nothing matches,
// and the writes return ErrDropNameTaken when the season already holds the name.
type Drops interface {
	ByID(ctx context.Context, id ids.ID) (Drop, error)

	// ListBySeason returns the drops of one season, active and inactive,
	// ordered by target date.
	ListBySeason(ctx context.Context, seasonID ids.ID) ([]Drop, error)

	Create(ctx context.Context, drop Drop) error
	Update(ctx context.Context, drop Drop) error
}

// DropCreateParams is the arguments for CreateDrop.
type DropCreateParams struct {
	SeasonID   ids.ID
	Name       string
	TargetDate time.Time
}

// DropUpdateParams is the arguments for UpdateDrop.
type DropUpdateParams struct {
	Name       string
	TargetDate time.Time
	Active     bool
}

// Drop returns one drop.
func (s *Service) Drop(ctx context.Context, dropID ids.ID) (Drop, error) {
	drop, err := s.drops.ByID(ctx, dropID)
	if err != nil {
		return Drop{}, fmt.Errorf("catalog: loading drop %s: %w", dropID, err)
	}
	return drop, nil
}

// ListDrops returns the drops of one season, active and inactive, oldest first.
func (s *Service) ListDrops(ctx context.Context, seasonID ids.ID) ([]Drop, error) {
	drops, err := s.drops.ListBySeason(ctx, seasonID)
	if err != nil {
		return nil, fmt.Errorf("catalog: listing the drops of season %s: %w", seasonID, err)
	}
	return drops, nil
}

// CreateDrop writes a new drop inside a season that already exists.
func (s *Service) CreateDrop(ctx context.Context, in DropCreateParams) (Drop, error) {
	name := strings.TrimSpace(in.Name)

	var v validate.Validator
	v.Required(FieldName, name)
	v.Check(in.SeasonID != ids.Nil, FieldSeason, validate.Required)
	v.Check(!in.TargetDate.IsZero(), FieldTargetDate, validate.Required)
	if err := v.Err(); err != nil {
		return Drop{}, err
	}

	if _, err := s.seasons.ByID(ctx, in.SeasonID); err != nil {
		return Drop{}, fmt.Errorf("catalog: loading season %s: %w", in.SeasonID, err)
	}

	drop := Drop{
		ID:         s.ids.New(),
		SeasonID:   in.SeasonID,
		Name:       name,
		TargetDate: day(in.TargetDate),
		Active:     true,
	}

	write := func(ctx context.Context) error {
		if err := s.drops.Create(ctx, drop); err != nil {
			return err
		}
		return s.record(ctx, change(audit.EntityDrop, drop.ID, audit.ActionCreated, "", "", drop.Name))
	}
	if err := s.atomic.InTx(ctx, write); err != nil {
		return Drop{}, err
	}
	return drop, nil
}

// UpdateDrop writes the edited fields and records one trail entry per field
// that moved. It writes nothing at all when nothing moved.
func (s *Service) UpdateDrop(ctx context.Context, dropID ids.ID, in DropUpdateParams) error {
	name := strings.TrimSpace(in.Name)

	var v validate.Validator
	v.Required(FieldName, name)
	v.Check(!in.TargetDate.IsZero(), FieldTargetDate, validate.Required)
	if err := v.Err(); err != nil {
		return err
	}

	drop, err := s.drops.ByID(ctx, dropID)
	if err != nil {
		return fmt.Errorf("catalog: loading drop %s: %w", dropID, err)
	}
	target := day(in.TargetDate)

	var changes []audit.Change
	if name != drop.Name {
		changes = append(changes, change(audit.EntityDrop, drop.ID, audit.ActionChanged, FieldName, drop.Name, name))
		drop.Name = name
	}
	if !date.Equal(target, drop.TargetDate) {
		changes = append(changes, change(audit.EntityDrop, drop.ID, audit.ActionChanged, FieldTargetDate,
			date.ISO(drop.TargetDate), date.ISO(target)))
		drop.TargetDate = target
	}
	if in.Active != drop.Active {
		changes = append(changes, change(audit.EntityDrop, drop.ID, activation(in.Active), FieldActive,
			strconv.FormatBool(drop.Active), strconv.FormatBool(in.Active)))
		drop.Active = in.Active
	}

	if len(changes) == 0 {
		return nil
	}
	return s.atomic.InTx(ctx, func(ctx context.Context) error {
		if err := s.drops.Update(ctx, drop); err != nil {
			return err
		}
		return s.record(ctx, changes...)
	})
}
