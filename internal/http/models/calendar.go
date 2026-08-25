package models

import (
	"errors"
	"net/http"
	"time"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/http/models/views"
	"github.com/paveltessman/pilam/internal/http/paths"
	"github.com/paveltessman/pilam/internal/http/shared"
	sharedviews "github.com/paveltessman/pilam/internal/http/shared/views"
	"github.com/paveltessman/pilam/internal/milestones"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/logging"
	"github.com/paveltessman/pilam/internal/platform/media"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

// This code holds the calendar section of the model screen: the steps of one
// model, with the three dates and the state of each one.

// calendar is everything the section shows, and what the history of the model
// needs to name the step an entry belongs to.
type calendar struct {
	section views.ModelCalendar

	// steps is the short name of every milestone of the model, keyed by the
	// milestone. The trail of the card reads an entry under it.
	steps map[ids.ID]string

	// ids is every milestone of the model, which is the scope the trail reads
	// beside the model itself.
	ids []ids.ID
}

// scopes is what the history of the model covers: the model, and the steps
// under it.
func (c calendar) scopes(modelID ids.ID) []audit.Scope {
	return []audit.Scope{
		{Entity: audit.EntityModel, IDs: []ids.ID{modelID}},
		{Entity: audit.EntityMilestone, IDs: c.ids},
	}
}

// loadCalendar reads the calendar section of one model.
//
// It answers 500 itself and reports false when the section cannot be read.
func loadCalendar(
	w http.ResponseWriter,
	r *http.Request,
	milestoneSvc *milestones.Service,
	modelID ids.ID,
) (calendar, bool) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)

	rows, err := milestoneSvc.Calendar(ctx, modelID)
	if err != nil {
		logger.Error("listing the calendar of a model failed", "model", modelID, "err", err)
		shared.WriteServerError(w)
		return calendar{}, false
	}

	types, err := milestoneSvc.ListTypes(ctx)
	if err != nil {
		logger.Error("listing milestone types failed", "err", err)
		shared.WriteServerError(w)
		return calendar{}, false
	}

	templates, err := milestoneSvc.ListTemplates(ctx)
	if err != nil {
		logger.Error("listing milestone templates failed", "err", err)
		shared.WriteServerError(w)
		return calendar{}, false
	}

	names := make(map[ids.ID]string, len(types))
	for _, milestoneType := range types {
		names[milestoneType.ID] = milestoneType.Name
	}

	held := calendar{
		section: views.NewModelCalendar(rows, names, milestoneSvc.Today()),
		steps:   make(map[ids.ID]string, len(rows)),
		ids:     make([]ids.ID, len(rows)),
	}
	held.section.ModelID = modelID.String()
	held.section.Templates = templateChoices(templates)
	held.section.Types = typeChoices(types, rows)
	for i, row := range rows {
		held.steps[row.ID] = names[row.TypeID]
		held.ids[i] = row.ID
	}
	return held, true
}

// templateChoices is what the critical path control offers: every active
// template, with the default one first.
func templateChoices(templates []milestones.Template) []sharedviews.SelectOption {
	var choices []sharedviews.SelectOption
	for _, template := range templates {
		if !template.Active {
			continue
		}
		choice := sharedviews.SelectOption{Value: template.ID.String(), Label: template.Name}
		if template.Default {
			choices = append([]sharedviews.SelectOption{choice}, choices...)
			continue
		}
		choices = append(choices, choice)
	}
	return choices
}

// typeChoices is what the add control offers: every active step the model does
// not hold yet. A retired step still holds its type, so it stays off the list.
func typeChoices(types []milestones.Type, held []milestones.CalendarRow) []sharedviews.SelectOption {
	onModel := make(map[ids.ID]bool, len(held))
	for _, row := range held {
		onModel[row.TypeID] = true
	}

	var choices []sharedviews.SelectOption
	for _, milestoneType := range types {
		if !milestoneType.Active || onModel[milestoneType.ID] {
			continue
		}
		choices = append(choices, sharedviews.SelectOption{
			Value: milestoneType.ID.String(),
			Label: milestoneType.Name,
		})
	}
	return choices
}

// ApplyTemplate builds the calendar of a model from a critical path. It adds
// the steps the model lacks and moves no date the model already holds.
func ApplyTemplate(
	catalogSvc *catalog.Service,
	milestoneSvc *milestones.Service,
	authSvc *auth.Service,
	log *audit.Log,
	store media.Store,
) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		model, _, _, ok := loadOne(w, r, catalogSvc)
		if !ok {
			return
		}

		templateID, err := shared.PostedID(r, views.FieldMilestoneTemplate)
		if err == nil && templateID == ids.Nil {
			err = validate.Fail(views.FieldMilestoneTemplate, validate.Required)
		}

		var written []milestones.Milestone
		if err == nil {
			written, err = milestoneSvc.ApplyTemplate(ctx, model.ID, templateID)
		}
		if err == nil {
			logger.Info("calendar applied", "model", model.ID, "template", templateID, "steps", len(written))
			shared.RedirectSaved(w, r, paths.Models+"/"+model.ID.String())
			return
		}

		errs, ok := shared.Rejections(err)
		if !ok {
			logger.Error("applying a calendar failed unexpectedly",
				"model", model.ID, "template", templateID, "err", err)
			shared.WriteServerError(w)
			return
		}

		logger.Info("calendar refused", "model", model.ID, "reason", errs)
		refuseCalendar(w, r, catalogSvc, milestoneSvc, authSvc, log, store, errs)
	}
	return handler
}

// AddMilestone writes one step the model does not hold. The user states the
// plan date, because a step outside a critical path carries no offset.
func AddMilestone(
	catalogSvc *catalog.Service,
	milestoneSvc *milestones.Service,
	authSvc *auth.Service,
	log *audit.Log,
	store media.Store,
) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		model, _, _, ok := loadOne(w, r, catalogSvc)
		if !ok {
			return
		}

		typeID, typeErr := shared.PostedID(r, views.FieldMilestoneType)
		plan, planErr := shared.PostedDay(r, views.FieldMilestonePlan)

		err := errors.Join(typeErr, planErr)
		if err == nil {
			_, err = milestoneSvc.AddMilestone(ctx, model.ID, typeID, plan)
		}
		// A step the model already holds is a screen the user left open while
		// somebody else added it.
		if errors.Is(err, milestones.ErrTypeOnModel) {
			err = validate.Fail(views.FieldMilestoneType, validate.Taken)
		}
		if err == nil {
			logger.Info("milestone added", "model", model.ID, "type", typeID)
			shared.RedirectSaved(w, r, paths.Models+"/"+model.ID.String())
			return
		}

		errs, ok := shared.Rejections(typeErr, planErr, err)
		if !ok {
			logger.Error("adding a milestone failed unexpectedly",
				"model", model.ID, "type", typeID, "err", err)
			shared.WriteServerError(w)
			return
		}

		logger.Info("milestone refused", "model", model.ID, "reason", errs)
		refuseCalendar(w, r, catalogSvc, milestoneSvc, authSvc, log, store, errs)
	}
	return handler
}

// SaveMilestone writes one row of the calendar: the plan date, the fact date,
// the note and the active flag.
//
// The row carries several buttons, and the one the user pressed says what to do
// with the fact date and with the flag. The boxes of the row state the rest.
func SaveMilestone(
	catalogSvc *catalog.Service,
	milestoneSvc *milestones.Service,
	authSvc *auth.Service,
	log *audit.Log,
	store media.Store,
) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		model, _, _, ok := loadOne(w, r, catalogSvc)
		if !ok {
			return
		}
		milestoneID, ok := milestoneOf(w, r)
		if !ok {
			return
		}

		plan, planErr := shared.PostedDay(r, views.FieldMilestonePlan)
		fact, factErr := shared.PostedDay(r, views.FieldMilestoneFact)
		in := milestones.MilestoneUpdateParams{
			Plan:   plan,
			Fact:   fact,
			Note:   r.PostFormValue(views.FieldMilestoneNote),
			Active: shared.PostedFlag(r, views.FieldMilestoneActive),
		}
		pressed(r, milestoneSvc.Today(), &in)

		err := errors.Join(planErr, factErr)
		if err == nil {
			err = milestoneSvc.UpdateMilestone(ctx, milestoneID, in)
		}
		if errors.Is(err, milestones.ErrNoMilestone) {
			logger.Info("no such milestone", "model", model.ID, "milestone", milestoneID)
			http.NotFound(w, r)
			return
		}
		if err == nil {
			logger.Info("milestone updated", "model", model.ID, "milestone", milestoneID)
			shared.RedirectSaved(w, r, paths.Models+"/"+model.ID.String())
			return
		}

		errs, ok := shared.Rejections(planErr, factErr, err)
		if !ok {
			logger.Error("updating a milestone failed unexpectedly",
				"model", model.ID, "milestone", milestoneID, "err", err)
			shared.WriteServerError(w)
			return
		}

		logger.Info("milestone update rejected", "model", model.ID, "milestone", milestoneID, "reason", errs)
		refuseCalendar(w, r, catalogSvc, milestoneSvc, authSvc, log, store, errs)
	}
	return handler
}

// MovePlan moves the plan date of one step.
//
// A plan date that moves later carries the steps after it, so the screen shows
// the rule before it writes: the first post answers with the panel that names
// every step the move carries, and the panel posts back the confirmation.
//
// A date that carries nobody is written at once, because the panel asks about a
// choice the user does not have.
func MovePlan(
	catalogSvc *catalog.Service,
	milestoneSvc *milestones.Service,
	authSvc *auth.Service,
	log *audit.Log,
	store media.Store,
) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		model, _, _, ok := loadOne(w, r, catalogSvc)
		if !ok {
			return
		}
		milestoneID, ok := milestoneOf(w, r)
		if !ok {
			return
		}

		plan, err := shared.PostedDay(r, views.FieldMilestonePlan)
		shift := shared.PostedFlag(r, views.FieldMilestoneShift)

		// The panel marks its own post. Everything else is the box of a row,
		// and it gets the question rather than the write.
		if err == nil && !shared.PostedFlag(r, views.FieldMilestoneConfirm) {
			var moves []milestones.Move
			moves, err = milestoneSvc.Moves(ctx, milestoneID, plan)
			if err == nil && len(moves) > 1 {
				logger.Info("the calendar asks about a plan date move",
					"model", model.ID, "milestone", milestoneID, "steps", len(moves))
				askMove(w, r, catalogSvc, milestoneSvc, authSvc, log, store, moves)
				return
			}
			// One step or none: nothing follows it, so the switch says nothing.
			shift = false
		}

		var moved []milestones.Move
		if err == nil {
			moved, err = milestoneSvc.MovePlan(ctx, milestoneID, plan, shift)
		}
		if errors.Is(err, milestones.ErrNoMilestone) {
			logger.Info("no such milestone", "model", model.ID, "milestone", milestoneID)
			http.NotFound(w, r)
			return
		}
		if err == nil {
			logger.Info("plan date moved",
				"model", model.ID, "milestone", milestoneID, "steps", len(moved), "shift", shift)
			shared.RedirectSaved(w, r, paths.Models+"/"+model.ID.String())
			return
		}

		errs, ok := shared.Rejections(err)
		if !ok {
			logger.Error("moving a plan date failed unexpectedly",
				"model", model.ID, "milestone", milestoneID, "err", err)
			shared.WriteServerError(w)
			return
		}

		logger.Info("plan date move refused", "model", model.ID, "milestone", milestoneID, "reason", errs)
		refuseCalendar(w, r, catalogSvc, milestoneSvc, authSvc, log, store, errs)
	}
	return handler
}

// askMove renders the card with the panel that asks about the move. The card
// comes back whole, with the calendar as it still stands, because the write
// waits for the answer.
func askMove(
	w http.ResponseWriter,
	r *http.Request,
	catalogSvc *catalog.Service,
	milestoneSvc *milestones.Service,
	authSvc *auth.Service,
	log *audit.Log,
	store media.Store,
	moves []milestones.Move,
) {
	card, ok := modelCard(w, r, catalogSvc, milestoneSvc, authSvc, log, store)
	if !ok {
		return
	}
	card.Calendar.Move = views.NewCalendarMove(card.Calendar, moves)
	shared.Render(w, r, http.StatusOK, views.Model(card))
}

// pressed reads the button of the row the user pressed and writes what it means
// onto the edit. A plain save presses nothing, and the boxes stand as they are.
func pressed(r *http.Request, today time.Time, in *milestones.MilestoneUpdateParams) {
	switch r.PostFormValue(views.FieldRowAction) {
	case views.RowFactToday:
		in.Fact = today
	case views.RowFactClear:
		in.Fact = time.Time{}
	case views.RowRetire:
		in.Active = false
	case views.RowRestore:
		in.Active = true
	}
}

// refuseCalendar renders the card again with the refusal on it. Every write of
// the section answers this way: the screen comes back whole, with the calendar
// and the trail as they stand.
func refuseCalendar(
	w http.ResponseWriter,
	r *http.Request,
	catalogSvc *catalog.Service,
	milestoneSvc *milestones.Service,
	authSvc *auth.Service,
	log *audit.Log,
	store media.Store,
	errs validate.FieldErrors,
) {
	card, ok := modelCard(w, r, catalogSvc, milestoneSvc, authSvc, log, store)
	if !ok {
		return
	}
	card.Errors = errs
	shared.Render(w, r, http.StatusUnprocessableEntity, views.Model(card))
}

// milestoneOf reads the {milestoneID} of the route. It answers 404 itself for
// an identifier it cannot read, and then reports false.
func milestoneOf(w http.ResponseWriter, r *http.Request) (ids.ID, bool) {
	id, err := ids.Parse(r.PathValue("milestoneID"))
	if err != nil {
		logging.FromContext(r.Context()).Info("the path names no readable milestone id",
			"id", r.PathValue("milestoneID"))
		http.NotFound(w, r)
		return ids.Nil, false
	}
	return id, true
}
