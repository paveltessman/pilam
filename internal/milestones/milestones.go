// Package milestones owns the time and action calendar: the milestone types,
// the templates that order them, and the milestones of one model.
package milestones

import (
	"context"
	"errors"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/validate"
	"github.com/paveltessman/pilam/internal/shared"
)

// The fields a screen posts and a rejection names.
const (
	FieldName        = "name"
	FieldDescription = "description"
	FieldActive      = "active"
	FieldDefault     = "default"
	FieldType        = "type"
	FieldOffset      = "offset"
	FieldGap         = "gap"
	FieldItemOrder   = "item_order"
	FieldTemplate    = "template"
	FieldPlanDate    = "plan_date"
	FieldFactDate    = "fact_date"
	FieldNote        = "note"
)

const (
	MaxNameLen        = 30
	MaxDescriptionLen = 200
	MaxNoteLen        = 500

	MaxOffset = 0
	MinOffset = -3650 // 10 years
)

var (
	ErrNoType      = errors.New("milestone: no such milestone type")
	ErrNoTemplate  = errors.New("milestone: no such milestone template")
	ErrNoItem      = errors.New("milestone: no such template item")
	ErrNoMilestone = errors.New("milestone: no such milestone")

	// ErrNoModel is a model the calendar cannot be read or written for. The
	// catalog owns the row: the calendar only reads the target date of its drop.
	ErrNoModel = errors.New("milestone: no such model")

	ErrNameTaken = errors.New("milestone: another row already holds that name")
	ErrIDTaken   = errors.New("milestone: identifier already taken")

	// ErrTypeInTemplate is the rule that one type appears once per template.
	ErrTypeInTemplate = errors.New("milestone: the template already holds that milestone type")

	// ErrTypeOnModel is the rule that a model holds one milestone per type.
	ErrTypeOnModel = errors.New("milestone: the model already holds that milestone type")

	// ErrItemOrder is a reorder that does not name every item of the template
	// exactly once.
	ErrItemOrder = errors.New("milestone: the order does not name the items of the template")
)

type Atomic interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type Service struct {
	shared.BaseService
	types     TypesStore
	templates TemplatesStore
	atomic    Atomic
	ids       ids.Generator
}

type Store struct {
	Types     TypesStore
	Templates TemplatesStore
	Atomic    Atomic
}

func NewService(store Store, gen ids.Generator, trail *audit.Trail) *Service {
	switch {
	case store.Types == nil:
		panic("milestone: nil types store")
	case store.Templates == nil:
		panic("milestone: nil templates store")
	case store.Atomic == nil:
		panic("milestone: nil transaction runner")
	case gen == nil:
		panic("milestone: nil id generator")
	}
	service := &Service{
		BaseService: shared.NewBaseService(trail),
		types:       store.Types,
		templates:   store.Templates,
		atomic:      store.Atomic,
		ids:         gen,
	}
	return service
}

// checkNaming states the rules a type and a template share: two fields the user
// fills in, each within the length its screen holds.
func checkNaming(name, description string) error {
	var v validate.Validator
	if v.Required(FieldName, name) {
		v.MaxLen(FieldName, name, MaxNameLen)
	}
	if v.Required(FieldDescription, description) {
		v.MaxLen(FieldDescription, description, MaxDescriptionLen)
	}
	return v.Err()
}
