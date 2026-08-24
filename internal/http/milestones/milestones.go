// Package milestones holds the screens of the milestone section: the types a
// milestone can be, and the templates built out of them.
package milestones

import (
	"errors"
	"net/http"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/http/milestones/views"
	"github.com/paveltessman/pilam/internal/http/paths"
	"github.com/paveltessman/pilam/internal/http/shared"
	"github.com/paveltessman/pilam/internal/milestones"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/logging"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

// ShowSection lists the templates a calendar is built from, and the types the
// templates are built from. Both lists hold every row, active and retired.
func ShowSection(milestoneSvc *milestones.Service) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		templates, err := milestoneSvc.ListTemplates(ctx)
		if err != nil {
			logger.Error("listing milestone templates failed", "err", err)
			shared.WriteServerError(w)
			return
		}

		types, err := milestoneSvc.ListTypes(ctx)
		if err != nil {
			logger.Error("listing milestone types failed", "err", err)
			shared.WriteServerError(w)
			return
		}

		page := views.MilestonesPage{Chrome: shared.Chrome(ctx), Types: types, Templates: templates}
		shared.Render(w, r, http.StatusOK, views.Milestones(page))
	}
	return handler
}

// ShowNewType renders the empty create form.
func ShowNewType() http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		form := views.MilestoneTypeForm{Chrome: shared.Chrome(r.Context()), Active: true}
		shared.Render(w, r, http.StatusOK, views.MilestoneType(form))
	}
	return handler
}

// CreateType writes the type and opens the card of it.
func CreateType(milestoneSvc *milestones.Service) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		form := submittedType(r)
		form.Chrome = shared.Chrome(ctx)

		milestoneType, err := milestoneSvc.CreateType(ctx, milestones.TypeCreateParams{
			Name:        form.Name,
			Description: form.Description,
		})
		if errors.Is(err, milestones.ErrNameTaken) {
			err = validate.Fail(views.FieldName, validate.Taken)
		}
		if err == nil {
			logger.Info("milestone type created", "type", milestoneType.ID, "name", milestoneType.Name)
			shared.RedirectSaved(w, r, paths.MilestoneTypes+"/"+milestoneType.ID.String())
			return
		}

		errs, ok := shared.Rejections(err)
		if !ok {
			logger.Error("creating a milestone type failed unexpectedly", "err", err)
			shared.WriteServerError(w)
			return
		}

		logger.Info("milestone type create rejected", "reason", errs)
		form.Errors = errs
		shared.Render(w, r, http.StatusUnprocessableEntity, views.MilestoneType(form))
	}
	return handler
}

// ShowType renders the edit form of one type, and the audit trail of
// that type under it.
func ShowType(milestoneSvc *milestones.Service, authSvc *auth.Service, log *audit.Log) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		milestoneType, ok := loadType(w, r, milestoneSvc)
		if !ok {
			return
		}

		trail, ok := shared.LoadTrail(w, r, authSvc, log, audit.EntityMilestoneType, milestoneType.ID)
		if !ok {
			return
		}

		form := views.NewMilestoneTypeForm(shared.Chrome(r.Context()), milestoneType)
		form.Trail = trail
		if shared.IsSaved(r) {
			form.Notice = labels.Saved
		}
		shared.Render(w, r, http.StatusOK, views.MilestoneType(form))
	}
	return handler
}

// SaveType writes name, description and active flag.
func SaveType(milestoneSvc *milestones.Service, authSvc *auth.Service, log *audit.Log) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		milestoneType, ok := loadType(w, r, milestoneSvc)
		if !ok {
			return
		}

		form := submittedType(r)
		form.Chrome = shared.Chrome(ctx)
		form.TypeID = milestoneType.ID.String()

		err := milestoneSvc.UpdateType(ctx, milestoneType.ID, milestones.TypeUpdateParams{
			Name:        form.Name,
			Description: form.Description,
			Active:      form.Active,
		})
		if errors.Is(err, milestones.ErrNameTaken) {
			err = validate.Fail(views.FieldName, validate.Taken)
		}
		if err == nil {
			logger.Info("milestone type updated", "type", milestoneType.ID)
			shared.RedirectSaved(w, r, paths.MilestoneTypes+"/"+form.TypeID)
			return
		}

		errs, ok := shared.Rejections(err)
		if !ok {
			logger.Error("updating a milestone type failed unexpectedly", "type", milestoneType.ID, "err", err)
			shared.WriteServerError(w)
			return
		}

		// The screen comes back whole: the refusal, and the trail under it.
		trail, ok := shared.LoadTrail(w, r, authSvc, log, audit.EntityMilestoneType, milestoneType.ID)
		if !ok {
			return
		}

		logger.Info("milestone type update rejected", "type", milestoneType.ID, "reason", errs)
		form.Errors = errs
		form.Trail = trail
		shared.Render(w, r, http.StatusUnprocessableEntity, views.MilestoneType(form))
	}
	return handler
}

// loadType reads the {id} of the route and returns the type it names.
// It answers 404 itself for an unreadable id and for a type that is not there,
// and then reports false.
func loadType(w http.ResponseWriter, r *http.Request, milestoneSvc *milestones.Service) (milestones.Type, bool) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)

	id, err := ids.Parse(r.PathValue("id"))
	if err != nil {
		logger.Info("the path names no readable milestone type id", "id", r.PathValue("id"))
		http.NotFound(w, r)
		return milestones.Type{}, false
	}

	milestoneType, err := milestoneSvc.Type(ctx, id)
	if errors.Is(err, milestones.ErrNoType) {
		logger.Info("no such milestone type", "type", id)
		http.NotFound(w, r)
		return milestones.Type{}, false
	}
	if err != nil {
		logger.Error("loading a milestone type failed", "type", id, "err", err)
		shared.WriteServerError(w)
		return milestones.Type{}, false
	}

	return milestoneType, true
}

// submittedType reads the fields the create and the edit form post.
// The service checks the values: this only carries them.
func submittedType(r *http.Request) views.MilestoneTypeForm {
	form := views.MilestoneTypeForm{
		Name:        r.PostFormValue(views.FieldName),
		Description: r.PostFormValue(views.FieldDescription),
		Active:      shared.PostedFlag(r, views.FieldActive),
	}
	return form
}
