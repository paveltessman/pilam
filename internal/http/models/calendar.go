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

// ShowCalendarEdit answers with the dialog that builds the calendar: it applies
// a whole critical path, or it adds one step outside every path.
func ShowCalendarEdit(catalogSvc *catalog.Service, milestoneSvc *milestones.Service) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		model, _, _, ok := loadOne(w, r, catalogSvc)
		if !ok {
			return
		}
		held, ok := loadCalendar(w, r, milestoneSvc, model.ID)
		if !ok {
			return
		}
		shared.Render(w, r, http.StatusOK, views.CalendarDialog(held.section, nil))
	}
	return handler
}

// ShowMilestoneEdit answers with the dialog that edits one step.
func ShowMilestoneEdit(catalogSvc *catalog.Service, milestoneSvc *milestones.Service) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		model, _, _, ok := loadOne(w, r, catalogSvc)
		if !ok {
			return
		}
		milestoneID, ok := milestoneOf(w, r)
		if !ok {
			return
		}
		held, ok := loadCalendar(w, r, milestoneSvc, model.ID)
		if !ok {
			return
		}

		row, held2 := held.section.Row(milestoneID.String())
		if !held2 {
			logging.FromContext(r.Context()).Info("no such milestone on the model",
				"model", model.ID, "milestone", milestoneID)
			http.NotFound(w, r)
			return
		}
		shared.Render(w, r, http.StatusOK, views.MilestoneDialog(held.section, row, nil))
	}
	return handler
}

// PreviewPlan answers what a plan date would carry with it. It writes nothing:
// the dialog reads it while the user picks a date, and the save that follows
// moves the dates.
func PreviewPlan(catalogSvc *catalog.Service, milestoneSvc *milestones.Service) http.HandlerFunc {
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

		plan, err := shared.QueriedDay(r, views.FieldMilestonePlan)
		if err != nil || plan.IsZero() {
			// A date the box cannot read carries nothing, and the dialog says
			// so by showing no preview at all.
			shared.Render(w, r, http.StatusOK, views.MilestoneShift(views.CalendarShift{}))
			return
		}

		moves, err := milestoneSvc.Moves(ctx, milestoneID, plan)
		if errors.Is(err, milestones.ErrNoMilestone) {
			logger.Info("no such milestone", "model", model.ID, "milestone", milestoneID)
			http.NotFound(w, r)
			return
		}
		if err != nil {
			if _, refused := shared.Rejections(err); refused {
				shared.Render(w, r, http.StatusOK, views.MilestoneShift(views.CalendarShift{}))
				return
			}
			logger.Error("reading what a plan date move carries failed",
				"model", model.ID, "milestone", milestoneID, "err", err)
			shared.WriteServerError(w)
			return
		}

		held, ok := loadCalendar(w, r, milestoneSvc, model.ID)
		if !ok {
			return
		}
		shared.Render(w, r, http.StatusOK,
			views.MilestoneShift(views.NewCalendarShift(held.section, moves)))
	}
	return handler
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
		refuseCalendar(w, r, catalogSvc, milestoneSvc, authSvc, log, store, errs,
			views.DialogCalendar, "")
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
		refuseCalendar(w, r, catalogSvc, milestoneSvc, authSvc, log, store, errs,
			views.DialogCalendar, "")
	}
	return handler
}

// SaveMilestone writes one step of the calendar: the plan date, the fact date,
// the note and the active flag.
//
// A plan date carries the steps after it, so it moves in a write of its own,
// before the rest of the step. The dialog showed what the move carries before
// the user saved, and the switch of that preview says whether to carry them.
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
			Fact:   fact,
			Note:   r.PostFormValue(views.FieldMilestoneNote),
			Active: shared.PostedFlag(r, views.FieldMilestoneActive),
		}
		pressed(r, milestoneSvc.Today(), &in)

		err := errors.Join(planErr, factErr)
		// A button of a row posts no plan date at all, and it moves none.
		if err == nil && !plan.IsZero() {
			_, err = milestoneSvc.MovePlan(ctx, milestoneID, plan,
				shared.PostedFlag(r, views.FieldMilestoneShift))
		}
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
		refuseCalendar(w, r, catalogSvc, milestoneSvc, authSvc, log, store, errs,
			views.DialogMilestone, milestoneID.String())
	}
	return handler
}

// pressed reads the button of the row the user pressed and writes what it means
// onto the edit. A plain save presses nothing, and the boxes stand as they are.
func pressed(r *http.Request, today time.Time, in *milestones.MilestoneUpdateParams) {
	switch r.PostFormValue(views.FieldRowAction) {
	case views.RowFactToday:
		in.Fact = today
	case views.RowRetire:
		in.Active = false
	case views.RowRestore:
		in.Active = true
	}
}

// refuseCalendar renders the card again with the refusal on it. Every write of
// the section answers this way: the screen comes back whole, with the dialog the
// write came from open on the message it was refused with.
func refuseCalendar(
	w http.ResponseWriter,
	r *http.Request,
	catalogSvc *catalog.Service,
	milestoneSvc *milestones.Service,
	authSvc *auth.Service,
	log *audit.Log,
	store media.Store,
	errs validate.FieldErrors,
	open, openRow string,
) {
	card, ok := modelCard(w, r, catalogSvc, milestoneSvc, authSvc, log, store)
	if !ok {
		return
	}
	card.Errors = errs
	card.Open = open
	card.OpenRow = openRow
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
