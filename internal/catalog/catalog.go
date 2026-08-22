// Package catalog owns the main entities: seasons, drops, models, and model photos.
package catalog

import (
	"context"
	"errors"
	"time"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/platform/date"
	"github.com/paveltessman/pilam/internal/platform/ids"
)

const (
	FieldName       = "name"
	FieldStartDate  = "start_date"
	FieldTargetDate = "target_date"
	FieldSeason     = "season_id"
	FieldDrop       = "drop_id"
	FieldArticle    = "article"
	FieldActive     = "active"
	FieldPhoto      = "photo"
	FieldPhotoOrder = "photo_order"
)

var (
	ErrNoSeason = errors.New("catalog: no such season")
	ErrNoDrop   = errors.New("catalog: no such drop")
	ErrNoModel  = errors.New("catalog: no such model")
	ErrNoPhoto  = errors.New("catalog: no such photo")

	ErrSeasonNameTaken = errors.New("catalog: another season already holds that name")
	ErrDropNameTaken   = errors.New("catalog: another drop of the season already holds that name")
	ErrIDTaken         = errors.New("catalog: identifier already taken")

	ErrPhotoOrder = errors.New("catalog: the order must name every photo of the model once")
)

type Atomic interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type Service struct {
	seasons Seasons
	drops   Drops
	models  Models
	photos  Photos
	atomic  Atomic
	ids     ids.Generator
	trail   *audit.Trail
}

// Stores is what the service reads and writes through.
// Implemented in the postgres package.
type Stores struct {
	Seasons Seasons
	Drops   Drops
	Models  Models
	Photos  Photos
	Atomic  Atomic
}

func NewService(stores Stores, gen ids.Generator, trail *audit.Trail) *Service {
	switch {
	case stores.Seasons == nil:
		panic("catalog: nil seasons store")
	case stores.Drops == nil:
		panic("catalog: nil drops store")
	case stores.Models == nil:
		panic("catalog: nil models store")
	case stores.Photos == nil:
		panic("catalog: nil photos store")
	case stores.Atomic == nil:
		panic("catalog: nil transaction runner")
	case gen == nil:
		panic("catalog: nil id generator")
	case trail == nil:
		panic("catalog: nil audit trail")
	}
	service := &Service{
		seasons: stores.Seasons,
		drops:   stores.Drops,
		models:  stores.Models,
		photos:  stores.Photos,
		atomic:  stores.Atomic,
		ids:     gen,
		trail:   trail,
	}
	return service
}

// change is one stated fact about a catalog row.
func change(entity string, entityID ids.ID, action, field, old, next string) audit.Change {
	c := audit.Change{
		Entity:   entity,
		EntityID: entityID,
		Action:   action,
		FieldKey: field,
		Old:      old,
		New:      next,
	}
	return c
}

// record hands the changes to the trail, under the user the request is
// authenticated as.
//
// A context with no identity carries no actor, and the trail refuses the entry.
func (s *Service) record(ctx context.Context, changes ...audit.Change) error {
	identity, _ := auth.FromContext(ctx)
	return s.trail.Record(ctx, identity.UserID, changes...)
}

func day(t time.Time) time.Time { return date.Of(t.Date()) }
