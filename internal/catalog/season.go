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

// Season is one selling season.
type Season struct {
	ID        ids.ID
	Name      string
	StartDate time.Time
	Active    bool
}

// Seasons is the store of season rows. ByID returns ErrNoSeason when nothing
// matches, and writes return ErrSeasonNameTaken on a duplicate name.
type Seasons interface {
	ByID(ctx context.Context, id ids.ID) (Season, error)

	// List returns every season, active and inactive, ordered by start date.
	List(ctx context.Context) ([]Season, error)

	Create(ctx context.Context, season Season) error
	Update(ctx context.Context, season Season) error
}

type SeasonCreateParams struct {
	Name      string
	StartDate time.Time
}

// SeasonUpdateParams is what the seasons section changes on a season that already
// exists.
type SeasonUpdateParams struct {
	Name      string
	StartDate time.Time
	Active    bool
}

// Season returns one season.
func (s *Service) Season(ctx context.Context, seasonID ids.ID) (Season, error) {
	season, err := s.seasons.ByID(ctx, seasonID)
	if err != nil {
		return Season{}, fmt.Errorf("catalog: loading season %s: %w", seasonID, err)
	}
	return season, nil
}

// ListSeasons returns every season, active and inactive, oldest first.
func (s *Service) ListSeasons(ctx context.Context) ([]Season, error) {
	seasons, err := s.seasons.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("catalog: listing seasons: %w", err)
	}
	return seasons, nil
}

// CreateSeason writes a new season and returns the row it wrote.
func (s *Service) CreateSeason(ctx context.Context, in SeasonCreateParams) (Season, error) {
	name := strings.TrimSpace(in.Name)

	var v validate.Validator
	v.Required(FieldName, name)
	v.Check(!in.StartDate.IsZero(), FieldStartDate, validate.Required)
	if err := v.Err(); err != nil {
		return Season{}, err
	}

	season := Season{
		ID:        s.ids.New(),
		Name:      name,
		StartDate: day(in.StartDate),
		Active:    true,
	}

	write := func(ctx context.Context) error {
		if err := s.seasons.Create(ctx, season); err != nil {
			return err
		}
		return s.RecordTrail(ctx, change(audit.EntitySeason, season.ID, audit.ActionCreated, "", "", season.Name))
	}
	if err := s.atomic.InTx(ctx, write); err != nil {
		return Season{}, err
	}
	return season, nil
}

// UpdateSeason writes the edited fields and records one trail entry per field
// that moved. It writes nothing at all when nothing moved.
func (s *Service) UpdateSeason(ctx context.Context, seasonID ids.ID, in SeasonUpdateParams) error {
	name := strings.TrimSpace(in.Name)

	var v validate.Validator
	v.Required(FieldName, name)
	v.Check(!in.StartDate.IsZero(), FieldStartDate, validate.Required)
	if err := v.Err(); err != nil {
		return err
	}

	season, err := s.seasons.ByID(ctx, seasonID)
	if err != nil {
		return fmt.Errorf("catalog: loading season %s: %w", seasonID, err)
	}
	start := day(in.StartDate)

	var changes []audit.Change
	if name != season.Name {
		changes = append(changes, change(audit.EntitySeason, season.ID, audit.ActionChanged, FieldName, season.Name, name))
		season.Name = name
	}
	if !date.Equal(start, season.StartDate) {
		changes = append(changes, change(audit.EntitySeason, season.ID, audit.ActionChanged, FieldStartDate,
			date.ISO(season.StartDate), date.ISO(start)))
		season.StartDate = start
	}
	if in.Active != season.Active {
		changes = append(changes, change(audit.EntitySeason, season.ID, audit.Activation(in.Active), FieldActive,
			strconv.FormatBool(season.Active), strconv.FormatBool(in.Active)))
		season.Active = in.Active
	}

	if len(changes) == 0 {
		return nil
	}
	err = s.atomic.InTx(ctx, func(ctx context.Context) error {
		if err := s.seasons.Update(ctx, season); err != nil {
			return err
		}
		return s.RecordTrail(ctx, changes...)
	})
	return err
}

// CurrentSeason is the season a screen opens on: the one the brand works in
// today.
//
// It is the active season with the latest start date that is not after today. A
// brand whose active seasons all start later takes the earliest of them,
// because that is the one it works towards. A brand with no active season at
// all takes the newest season it holds.
//
// It answers the zero season for no season at all.
func CurrentSeason(seasons []Season, today time.Time) Season {
	var started, coming, newest Season
	for _, season := range seasons {
		if newest.ID == ids.Nil || startOrder(season, newest) > 0 {
			newest = season
		}
		if !season.Active {
			continue
		}
		if date.After(season.StartDate, today) {
			if coming.ID == ids.Nil || startOrder(season, coming) < 0 {
				coming = season
			}
			continue
		}
		if started.ID == ids.Nil || startOrder(season, started) > 0 {
			started = season
		}
	}

	switch {
	case started.ID != ids.Nil:
		return started
	case coming.ID != ids.Nil:
		return coming
	default:
		return newest
	}
}

// startOrder compares two seasons the way the list holds them: by start date,
// and then by name.
func startOrder(a, b Season) int {
	if !date.Equal(a.StartDate, b.StartDate) {
		return a.StartDate.Compare(b.StartDate)
	}
	return strings.Compare(a.Name, b.Name)
}
