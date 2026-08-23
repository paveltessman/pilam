package http

import (
	"errors"
	"net/http"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/http/views"
	"github.com/paveltessman/pilam/internal/milestone"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/logging"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

const (
	milestoneTypesPath   = views.MilestoneTypesPath
	milestoneTypeNewPath = milestoneTypesPath + "/new"
	milestoneTypePath    = milestoneTypesPath + "/{id}"
)

// showMilestoneTypes lists every type, active and retired, by short name.
func showMilestoneTypes(milestoneSvc *milestone.Service) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		types, err := milestoneSvc.ListTypes(ctx)
		if err != nil {
			logging.FromContext(ctx).Error("listing milestone types failed", "err", err)
			writeServerError(w)
			return
		}

		page := views.MilestoneTypesPage{Chrome: chrome(ctx), Types: types}
		render(w, r, http.StatusOK, views.MilestoneTypes(page))
	}
	return handler
}

// showNewMilestoneType renders the empty create form.
func showNewMilestoneType() http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		form := views.MilestoneTypeForm{Chrome: chrome(r.Context()), Active: true}
		render(w, r, http.StatusOK, views.MilestoneType(form))
	}
	return handler
}

// createMilestoneType writes the type and opens the card of it.
func createMilestoneType(milestoneSvc *milestone.Service) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		form := submittedMilestoneType(r)
		form.Chrome = chrome(ctx)

		milestoneType, err := milestoneSvc.CreateType(ctx, milestone.TypeCreateParams{
			Name:        form.Name,
			Description: form.Description,
		})
		if errors.Is(err, milestone.ErrNameTaken) {
			err = validate.Fail(views.FieldMilestoneTypeName, validate.Taken)
		}
		if err == nil {
			logger.Info("milestone type created", "type", milestoneType.ID, "name", milestoneType.Name)
			redirectSaved(w, r, milestoneTypesPath+"/"+milestoneType.ID.String())
			return
		}

		errs, ok := rejections(err)
		if !ok {
			logger.Error("creating a milestone type failed unexpectedly", "err", err)
			writeServerError(w)
			return
		}

		logger.Info("milestone type create rejected", "reason", errs)
		form.Errors = errs
		render(w, r, http.StatusUnprocessableEntity, views.MilestoneType(form))
	}
	return handler
}

// showMilestoneType renders the edit form of one type, and the audit trail of
// that type under it.
func showMilestoneType(milestoneSvc *milestone.Service, authSvc *auth.Service, log *audit.Log) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		milestoneType, ok := loadMilestoneType(w, r, milestoneSvc)
		if !ok {
			return
		}

		trail, ok := loadTrail(w, r, authSvc, log, audit.EntityMilestoneType, milestoneType.ID)
		if !ok {
			return
		}

		form := views.NewMilestoneTypeForm(chrome(r.Context()), milestoneType)
		form.Trail = trail
		if isSaved(r) {
			form.Notice = labels.Saved
		}
		render(w, r, http.StatusOK, views.MilestoneType(form))
	}
	return handler
}

// saveMilestoneType writes name, description and active flag.
func saveMilestoneType(milestoneSvc *milestone.Service, authSvc *auth.Service, log *audit.Log) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		milestoneType, ok := loadMilestoneType(w, r, milestoneSvc)
		if !ok {
			return
		}

		form := submittedMilestoneType(r)
		form.Chrome = chrome(ctx)
		form.TypeID = milestoneType.ID.String()

		err := milestoneSvc.UpdateType(ctx, milestoneType.ID, milestone.TypeUpdateParams{
			Name:        form.Name,
			Description: form.Description,
			Active:      form.Active,
		})
		if errors.Is(err, milestone.ErrNameTaken) {
			err = validate.Fail(views.FieldMilestoneTypeName, validate.Taken)
		}
		if err == nil {
			logger.Info("milestone type updated", "type", milestoneType.ID)
			redirectSaved(w, r, milestoneTypesPath+"/"+form.TypeID)
			return
		}

		errs, ok := rejections(err)
		if !ok {
			logger.Error("updating a milestone type failed unexpectedly", "type", milestoneType.ID, "err", err)
			writeServerError(w)
			return
		}

		// The screen comes back whole: the refusal, and the trail under it.
		trail, ok := loadTrail(w, r, authSvc, log, audit.EntityMilestoneType, milestoneType.ID)
		if !ok {
			return
		}

		logger.Info("milestone type update rejected", "type", milestoneType.ID, "reason", errs)
		form.Errors = errs
		form.Trail = trail
		render(w, r, http.StatusUnprocessableEntity, views.MilestoneType(form))
	}
	return handler
}

// loadMilestoneType reads the {id} of the route and returns the type it names.
// It answers 404 itself for an unreadable id and for a type that is not there,
// and then reports false.
func loadMilestoneType(w http.ResponseWriter, r *http.Request, milestoneSvc *milestone.Service) (milestone.Type, bool) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)

	id, err := ids.Parse(r.PathValue("id"))
	if err != nil {
		logger.Info("the path names no readable milestone type id", "id", r.PathValue("id"))
		http.NotFound(w, r)
		return milestone.Type{}, false
	}

	milestoneType, err := milestoneSvc.Type(ctx, id)
	if errors.Is(err, milestone.ErrNoType) {
		logger.Info("no such milestone type", "type", id)
		http.NotFound(w, r)
		return milestone.Type{}, false
	}
	if err != nil {
		logger.Error("loading a milestone type failed", "type", id, "err", err)
		writeServerError(w)
		return milestone.Type{}, false
	}

	return milestoneType, true
}

// submittedMilestoneType reads the fields the create and the edit form post.
// The service checks the values: this only carries them.
func submittedMilestoneType(r *http.Request) views.MilestoneTypeForm {
	form := views.MilestoneTypeForm{
		Name:        r.PostFormValue(views.FieldMilestoneTypeName),
		Description: r.PostFormValue(views.FieldMilestoneTypeDescription),
		Active:      postedFlag(r, views.FieldMilestoneTypeActive),
	}
	return form
}
