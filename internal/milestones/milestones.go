// Package milestone owns the time and action calendar: the milestone types,
// the templates that order them, and the milestones of one model.
package milestones

import (
	"context"
	"errors"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/shared"
)

const (
	FieldName        = "name"
	FieldDescription = "description"
	FieldActive      = "active"
)

var (
	ErrNoType = errors.New("milestone: no such milestone type")

	ErrNameTaken = errors.New("milestone: another milestone type already holds that name")
	ErrIDTaken   = errors.New("milestone: identifier already taken")
)

type Atomic interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type Service struct {
	shared.BaseService
	types  TypesStore
	atomic Atomic
	ids    ids.Generator
}

type Store struct {
	Types  TypesStore
	Atomic Atomic
}

func NewService(store Store, gen ids.Generator, trail *audit.Trail) *Service {
	switch {
	case store.Types == nil:
		panic("milestone: nil types store")
	case store.Atomic == nil:
		panic("milestone: nil transaction runner")
	case gen == nil:
		panic("milestone: nil id generator")
	}
	service := &Service{
		BaseService: shared.NewBaseService(trail),
		types:       store.Types,
		atomic:      store.Atomic,
		ids:         gen,
	}
	return service
}
