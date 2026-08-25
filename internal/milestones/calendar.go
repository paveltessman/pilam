package milestones

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/platform/date"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

// This code owns the calendar of one model: how it starts, how a step joins it,
// and how one row moves.

// CalendarRow is one milestone as a screen reads it: the row, and the state it
// holds today.
type CalendarRow struct {
	Milestone
	State State
}

type MilestoneUpdateParams struct {
	Plan time.Time

	// Fact is the day the step was done. The zero date clears a fact date.
	Fact time.Time

	Note   string
	Active bool
}

// Today is the current business day. A screen offers it as the fact date of a
// step that is done, and it bounds the fact date box.
func (s *Service) Today() time.Time { return s.clock.Today() }

// Calendar returns every milestone of one model, active and inactive, in plan
// date order, with the state each of them holds today.
//
// An inactive milestone leaves the calendar and stays in the history, so the
// caller shows the active rows and nothing else.
func (s *Service) Calendar(ctx context.Context, modelID ids.ID) ([]CalendarRow, error) {
	held, err := s.milestones.ByModel(ctx, modelID)
	if err != nil {
		return nil, fmt.Errorf("milestone: listing the calendar of model %s: %w", modelID, err)
	}

	today := s.clock.Today()
	rows := make([]CalendarRow, len(held))
	for i, one := range held {
		rows[i] = CalendarRow{Milestone: one, State: one.State(today)}
	}
	return rows, nil
}

// ApplyTemplate builds the calendar of a model from a template.
//
// The dates come from the target date of the drop and the offsets of the
// template. The baseline date and the plan date start equal.
//
// A model that already holds a calendar takes a template again: the write adds
// only the milestone types the model lacks, and it never touches a date that is
// already there. That is how a new step reaches the models of a season after
// the season started.
//
// It returns the milestones it wrote, which is none when the model already
// holds every type of the template.
func (s *Service) ApplyTemplate(ctx context.Context, modelID, templateID ids.ID) ([]Milestone, error) {
	// A template that is not there and a template that is retired are the same
	// answer: the picker does not offer either one.
	template, err := s.templates.ByID(ctx, templateID)
	if errors.Is(err, ErrNoTemplate) {
		return nil, validate.Fail(FieldTemplate, validate.NotAllowed)
	}
	if err != nil {
		return nil, fmt.Errorf("milestone: loading template %s: %w", templateID, err)
	}
	if !template.Active {
		return nil, validate.Fail(FieldTemplate, validate.NotAllowed)
	}

	target, err := s.milestones.Target(ctx, modelID)
	if err != nil {
		return nil, fmt.Errorf("milestone: loading the target date of model %s: %w", modelID, err)
	}

	items, err := s.templates.Items(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("milestone: listing the items of template %s: %w", templateID, err)
	}

	var written []Milestone
	write := func(ctx context.Context) error {
		held, err := s.milestones.ByModel(ctx, modelID)
		if err != nil {
			return fmt.Errorf("milestone: listing the calendar of model %s: %w", modelID, err)
		}

		written = s.missing(items, held, modelID, target)
		if len(written) == 0 {
			return nil
		}
		if err := s.milestones.Create(ctx, written...); err != nil {
			return err
		}
		return s.RecordTrail(ctx, audit.Change{
			Entity:   audit.EntityModel,
			EntityID: modelID,
			Action:   audit.ActionTemplateApplied,
			FieldKey: FieldTemplate,
			New:      appliedValue(template.Name, len(written)),
		})
	}
	if err := s.atomic.InTx(ctx, write); err != nil {
		return nil, err
	}
	return written, nil
}

// missing is the milestones a template writes onto a model that already holds
// held: one per item of a type the model lacks, dated from the target date.
//
// An inactive milestone still holds its type, so a retired step is not written
// a second time.
func (s *Service) missing(items []TemplateItem, held []Milestone, modelID ids.ID, target time.Time) []Milestone {
	onModel := make(map[ids.ID]bool, len(held))
	for _, one := range held {
		onModel[one.TypeID] = true
	}

	var out []Milestone
	for _, planned := range Apply(items, target) {
		if onModel[planned.TypeID] {
			continue
		}
		out = append(out, Milestone{
			ID:       s.ids.New(),
			ModelID:  modelID,
			TypeID:   planned.TypeID,
			Baseline: planned.Baseline,
			Plan:     planned.Plan,
			Active:   true,
		})
	}
	return out
}

// AddMilestone writes one step of a type the model lacks.
//
// The user states the plan date, because a step outside a template carries no
// offset of its own. The baseline date starts equal to the plan date.
func (s *Service) AddMilestone(ctx context.Context, modelID, typeID ids.ID, plan time.Time) (Milestone, error) {
	if plan.IsZero() {
		return Milestone{}, validate.Fail(FieldPlanDate, validate.Required)
	}

	// A type that is not there and a type that is not active are the same
	// answer: the picker does not offer either one.
	milestoneType, err := s.types.ByID(ctx, typeID)
	if errors.Is(err, ErrNoType) {
		return Milestone{}, validate.Fail(FieldType, validate.NotAllowed)
	}
	if err != nil {
		return Milestone{}, fmt.Errorf("milestone: loading type %s: %w", typeID, err)
	}
	if !milestoneType.Active {
		return Milestone{}, validate.Fail(FieldType, validate.NotAllowed)
	}

	if _, err := s.milestones.Target(ctx, modelID); err != nil {
		return Milestone{}, fmt.Errorf("milestone: loading the target date of model %s: %w", modelID, err)
	}

	day := date.Of(plan.Date())
	milestone := Milestone{
		ID:       s.ids.New(),
		ModelID:  modelID,
		TypeID:   typeID,
		Baseline: day,
		Plan:     day,
		Active:   true,
	}

	write := func(ctx context.Context) error {
		held, err := s.milestones.ByModel(ctx, modelID)
		if err != nil {
			return fmt.Errorf("milestone: listing the calendar of model %s: %w", modelID, err)
		}
		if slices.ContainsFunc(held, func(one Milestone) bool { return one.TypeID == typeID }) {
			return fmt.Errorf("%w: type %s on model %s", ErrTypeOnModel, typeID, modelID)
		}

		if err := s.milestones.Create(ctx, milestone); err != nil {
			return err
		}
		return s.RecordTrail(ctx, audit.Change{
			Entity:   audit.EntityMilestone,
			EntityID: milestone.ID,
			Action:   audit.ActionCreated,
			FieldKey: FieldPlanDate,
			New:      date.ISO(milestone.Plan),
		})
	}
	if err := s.atomic.InTx(ctx, write); err != nil {
		return Milestone{}, err
	}
	return milestone, nil
}

// UpdateMilestone writes the edited fields of one row and records one trail
// entry per field that moved. It writes nothing when nothing moved.
//
// A plan date moves this one milestone, and MovePlan is where a plan date that
// carries the rows after it goes. The baseline never moves here: it is what the
// calendar promised, and only the drop shift moves it.
func (s *Service) UpdateMilestone(ctx context.Context, milestoneID ids.ID, in MilestoneUpdateParams) error {
	plan, fact := zeroSafeDay(in.Plan), zeroSafeDay(in.Fact)
	note := strings.TrimSpace(in.Note)

	var v validate.Validator
	v.Check(!plan.IsZero(), FieldPlanDate, validate.Required)
	v.Check(fact.IsZero() || !date.After(fact, s.clock.Today()), FieldFactDate, validate.NotAllowed)
	v.MaxLen(FieldNote, note, MaxNoteLen)
	if err := v.Err(); err != nil {
		return err
	}

	milestone, err := s.milestones.ByID(ctx, milestoneID)
	if err != nil {
		return fmt.Errorf("milestone: loading milestone %s: %w", milestoneID, err)
	}

	var changes []audit.Change
	record := func(action, field, old, next string) {
		changes = append(changes, audit.Change{
			Entity:   audit.EntityMilestone,
			EntityID: milestone.ID,
			Action:   action,
			FieldKey: field,
			Old:      old,
			New:      next,
		})
	}

	if !date.Equal(plan, milestone.Plan) {
		record(audit.ActionChanged, FieldPlanDate, date.ISO(milestone.Plan), date.ISO(plan))
		milestone.Plan = plan
	}
	if !sameDay(fact, milestone.Fact) {
		record(factAction(milestone.Fact, fact), FieldFactDate, isoOrNone(milestone.Fact), isoOrNone(fact))
		milestone.Fact = fact
	}
	if note != milestone.Note {
		record(audit.ActionChanged, FieldNote, milestone.Note, note)
		milestone.Note = note
	}
	if in.Active != milestone.Active {
		record(audit.Activation(in.Active), FieldActive,
			strconv.FormatBool(milestone.Active), strconv.FormatBool(in.Active))
		milestone.Active = in.Active
	}

	if len(changes) == 0 {
		return nil
	}
	write := func(ctx context.Context) error {
		if err := s.milestones.Update(ctx, milestone); err != nil {
			return err
		}
		return s.RecordTrail(ctx, changes...)
	}
	return s.atomic.InTx(ctx, write)
}

// Moves is what a plan date change would do: the row the user moved first, and
// then every row the shift carries with it.
//
// It writes nothing. The screen reads it to show the rule before it writes.
func (s *Service) Moves(ctx context.Context, milestoneID ids.ID, plan time.Time) ([]Move, error) {
	if plan.IsZero() {
		return nil, validate.Fail(FieldPlanDate, validate.Required)
	}

	moved, err := s.milestones.ByID(ctx, milestoneID)
	if err != nil {
		return nil, fmt.Errorf("milestone: loading milestone %s: %w", milestoneID, err)
	}
	held, err := s.milestones.ByModel(ctx, moved.ModelID)
	if err != nil {
		return nil, fmt.Errorf("milestone: listing the calendar of model %s: %w", moved.ModelID, err)
	}
	return Cascade(held, moved, plan), nil
}

// MovePlan writes the plan date of one milestone, and the plan dates of the
// rows the shift carries with it.
//
// shift false writes the one row the user moved. The whole move runs in one
// transaction, and the trail holds one entry per row it moved.
//
// It returns the rows it wrote, which is none for a date that does not move.
func (s *Service) MovePlan(ctx context.Context, milestoneID ids.ID, plan time.Time, shift bool) ([]Move, error) {
	var moves []Move
	write := func(ctx context.Context) error {
		found, err := s.Moves(ctx, milestoneID, plan)
		if err != nil {
			return err
		}
		if !shift && len(found) > 1 {
			found = found[:1]
		}
		moves = found

		changes := make([]audit.Change, len(moves))
		for i, move := range moves {
			one := move.Milestone
			changes[i] = audit.Change{
				Entity:   audit.EntityMilestone,
				EntityID: one.ID,
				Action:   audit.ActionChanged,
				FieldKey: FieldPlanDate,
				Old:      date.ISO(one.Plan),
				New:      date.ISO(move.Plan),
			}
			one.Plan = move.Plan
			if err := s.milestones.Update(ctx, one); err != nil {
				return err
			}
		}
		if len(changes) == 0 {
			return nil
		}
		return s.RecordTrail(ctx, changes...)
	}
	if err := s.atomic.InTx(ctx, write); err != nil {
		return nil, err
	}
	return moves, nil
}

// factAction names what happened to a fact date: it was stamped, it was
// cleared, or it moved to another day.
func factAction(was, now time.Time) string {
	switch {
	case was.IsZero():
		return audit.ActionFactStamped
	case now.IsZero():
		return audit.ActionFactCleared
	default:
		return audit.ActionChanged
	}
}

// appliedValue renders a calendar for the trail: the template, and how many
// steps it wrote.
func appliedValue(name string, written int) string {
	return name + " (" + strconv.Itoa(written) + ")"
}

// sameDay reports whether two dates fall on the same day. Two dates that are
// both missing are the same.
func sameDay(a, b time.Time) bool {
	if a.IsZero() || b.IsZero() {
		return a.IsZero() && b.IsZero()
	}
	return date.Equal(a, b)
}

// isoOrNone renders a date the trail holds, and nothing at all for a date that
// is not there.
func isoOrNone(d time.Time) string {
	if d.IsZero() {
		return ""
	}
	return date.ISO(d)
}

// zeroSafeDay is the calendar day a value falls on, and the zero date for a
// value that is not there.
func zeroSafeDay(d time.Time) time.Time {
	if d.IsZero() {
		return time.Time{}
	}
	return date.Of(d.Date())
}
