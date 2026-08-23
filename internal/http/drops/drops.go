// Package drops holds the drop screens. A drop belongs to a season, and a
// model belongs to a drop.
package drops

import (
	"errors"
	"net/http"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/http/drops/views"
	"github.com/paveltessman/pilam/internal/http/paths"
	"github.com/paveltessman/pilam/internal/http/seasons"
	"github.com/paveltessman/pilam/internal/http/shared"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/logging"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

// ShowList lists the drops of every season, under the season that holds them.
func ShowList(catalogSvc *catalog.Service) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		groups, ok := BySeason(w, r, catalogSvc)
		if !ok {
			return
		}

		page := views.DropsPage{Chrome: shared.Chrome(r.Context()), Groups: groups}
		shared.Render(w, r, http.StatusOK, views.Drops(page))
	}
	return handler
}

// BySeason is every drop, under the season that holds it, in the order the
// two lists show them. It answers 500 itself and reports false on a failure.
func BySeason(w http.ResponseWriter, r *http.Request, catalogSvc *catalog.Service) ([]views.DropGroup, bool) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)

	all, err := catalogSvc.ListSeasons(ctx)
	if err != nil {
		logger.Error("listing seasons failed", "err", err)
		shared.WriteServerError(w)
		return nil, false
	}

	groups := make([]views.DropGroup, 0, len(all))
	for _, season := range all {
		drops, err := catalogSvc.ListDrops(ctx, season.ID)
		if err != nil {
			logger.Error("listing the drops of a season failed", "season", season.ID, "err", err)
			shared.WriteServerError(w)
			return nil, false
		}
		groups = append(groups, views.DropGroup{Season: season, Drops: drops})
	}
	return groups, true
}

// ShowNew renders the empty create form, with the active seasons to choose
// from.
func ShowNew(catalogSvc *catalog.Service) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		active, ok := seasons.Active(w, r, catalogSvc)
		if !ok {
			return
		}

		form := views.DropForm{Chrome: shared.Chrome(r.Context()), Seasons: active, Active: true}
		shared.Render(w, r, http.StatusOK, views.Drop(form))
	}
	return handler
}

// Create writes the drop and opens the card of it.
func Create(catalogSvc *catalog.Service) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		form := submitted(r)
		form.Chrome = shared.Chrome(ctx)

		seasonID, idErr := shared.PostedID(r, views.FieldDropSeason)
		target, dateErr := shared.PostedDay(r, views.FieldDropTarget)

		drop, err := catalogSvc.CreateDrop(ctx, catalog.DropCreateParams{
			SeasonID:   seasonID,
			Name:       form.Name,
			TargetDate: target,
		})
		switch {
		case errors.Is(err, catalog.ErrDropNameTaken):
			err = validate.Fail(views.FieldDropName, validate.Taken)
		case errors.Is(err, catalog.ErrNoSeason):
			err = validate.Fail(views.FieldDropSeason, validate.NotAllowed)
		}
		if err == nil && idErr == nil && dateErr == nil {
			logger.Info("drop created", "drop", drop.ID, "season", drop.SeasonID, "name", drop.Name)
			shared.RedirectSaved(w, r, paths.Drops+"/"+drop.ID.String())
			return
		}

		errs, ok := shared.Rejections(idErr, dateErr, err)
		if !ok {
			logger.Error("creating a drop failed unexpectedly", "err", err)
			shared.WriteServerError(w)
			return
		}

		if form.Seasons, ok = seasons.Active(w, r, catalogSvc); !ok {
			return
		}

		logger.Info("drop create rejected", "reason", errs)
		form.Errors = errs
		shared.Render(w, r, http.StatusUnprocessableEntity, views.Drop(form))
	}
	return handler
}

// Show renders the edit form of one drop, and the audit trail of that drop
// under it.
func Show(catalogSvc *catalog.Service, authSvc *auth.Service, log *audit.Log) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		drop, season, ok := loadOne(w, r, catalogSvc)
		if !ok {
			return
		}

		trail, ok := shared.LoadTrail(w, r, authSvc, log, audit.EntityDrop, drop.ID)
		if !ok {
			return
		}

		form := views.NewDropForm(shared.Chrome(r.Context()), drop, season)
		form.Trail = trail
		if shared.IsSaved(r) {
			form.Notice = labels.Saved
		}
		shared.Render(w, r, http.StatusOK, views.Drop(form))
	}
	return handler
}

// Save writes the name, the target date and the active flag.
func Save(catalogSvc *catalog.Service, authSvc *auth.Service, log *audit.Log) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		drop, season, ok := loadOne(w, r, catalogSvc)
		if !ok {
			return
		}

		form := submitted(r)
		form.Chrome = shared.Chrome(ctx)
		form.DropID = drop.ID.String()
		form.SeasonID = drop.SeasonID.String()
		form.SeasonName = season.Name

		target, dateErr := shared.PostedDay(r, views.FieldDropTarget)
		err := catalogSvc.UpdateDrop(ctx, drop.ID, catalog.DropUpdateParams{
			Name:       form.Name,
			TargetDate: target,
			Active:     form.Active,
		})
		if errors.Is(err, catalog.ErrDropNameTaken) {
			err = validate.Fail(views.FieldDropName, validate.Taken)
		}
		if err == nil && dateErr == nil {
			logger.Info("drop updated", "drop", drop.ID)
			shared.RedirectSaved(w, r, paths.Drops+"/"+form.DropID)
			return
		}

		errs, ok := shared.Rejections(dateErr, err)
		if !ok {
			logger.Error("updating a drop failed unexpectedly", "drop", drop.ID, "err", err)
			shared.WriteServerError(w)
			return
		}

		// The screen comes back whole: the refusal, and the trail under it.
		trail, ok := shared.LoadTrail(w, r, authSvc, log, audit.EntityDrop, drop.ID)
		if !ok {
			return
		}

		logger.Info("drop update rejected", "drop", drop.ID, "reason", errs)
		form.Errors = errs
		form.Trail = trail
		shared.Render(w, r, http.StatusUnprocessableEntity, views.Drop(form))
	}
	return handler
}

// loadOne reads the {id} of the route and returns the drop it names, with the
// season that holds it. It answers 404 itself for an unreadable id and for a
// drop that is not there, and then reports false.
func loadOne(w http.ResponseWriter, r *http.Request, catalogSvc *catalog.Service) (catalog.Drop, catalog.Season, bool) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)

	id, err := ids.Parse(r.PathValue("id"))
	if err != nil {
		logger.Info("the path names no readable drop id", "id", r.PathValue("id"))
		http.NotFound(w, r)
		return catalog.Drop{}, catalog.Season{}, false
	}

	drop, err := catalogSvc.Drop(ctx, id)
	if errors.Is(err, catalog.ErrNoDrop) {
		logger.Info("no such drop", "drop", id)
		http.NotFound(w, r)
		return catalog.Drop{}, catalog.Season{}, false
	}
	if err != nil {
		logger.Error("loading a drop failed", "drop", id, "err", err)
		shared.WriteServerError(w)
		return catalog.Drop{}, catalog.Season{}, false
	}

	// A drop references a season, so the row is there. A missing one is a
	// broken reference, which is a failure rather than a 404.
	season, err := catalogSvc.Season(ctx, drop.SeasonID)
	if err != nil {
		logger.Error("loading the season of a drop failed", "drop", id, "season", drop.SeasonID, "err", err)
		shared.WriteServerError(w)
		return catalog.Drop{}, catalog.Season{}, false
	}

	return drop, season, true
}

// submitted reads the fields the create and the edit form post.
func submitted(r *http.Request) views.DropForm {
	form := views.DropForm{
		SeasonID:   r.PostFormValue(views.FieldDropSeason),
		Name:       r.PostFormValue(views.FieldDropName),
		TargetDate: r.PostFormValue(views.FieldDropTarget),
		Active:     shared.PostedFlag(r, views.FieldDropActive),
	}
	return form
}
