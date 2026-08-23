// Package seasons holds the season screens. A season is the top of the
// catalog: a drop belongs to a season, and a model belongs to a drop.
package seasons

import (
	"errors"
	"net/http"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/http/paths"
	"github.com/paveltessman/pilam/internal/http/seasons/views"
	"github.com/paveltessman/pilam/internal/http/shared"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/logging"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

// ShowList lists every season, active and inactive, oldest first.
func ShowList(catalogSvc *catalog.Service) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		seasons, err := catalogSvc.ListSeasons(ctx)
		if err != nil {
			logging.FromContext(ctx).Error("listing seasons failed", "err", err)
			shared.WriteServerError(w)
			return
		}

		page := views.SeasonsPage{Chrome: shared.Chrome(ctx), Seasons: seasons}
		shared.Render(w, r, http.StatusOK, views.Seasons(page))
	}
	return handler
}

// ShowNew renders the empty create form.
func ShowNew() http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		form := views.SeasonForm{Chrome: shared.Chrome(r.Context()), Active: true}
		shared.Render(w, r, http.StatusOK, views.Season(form))
	}
	return handler
}

// Create writes the season and opens the card of it.
func Create(catalogSvc *catalog.Service) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		form := submitted(r)
		form.Chrome = shared.Chrome(ctx)

		start, dateErr := shared.PostedDay(r, views.FieldSeasonStart)
		season, err := catalogSvc.CreateSeason(ctx, catalog.SeasonCreateParams{
			Name:      form.Name,
			StartDate: start,
		})
		if errors.Is(err, catalog.ErrSeasonNameTaken) {
			err = validate.Fail(views.FieldSeasonName, validate.Taken)
		}
		if err == nil && dateErr == nil {
			logger.Info("season created", "season", season.ID, "name", season.Name)
			shared.RedirectSaved(w, r, paths.Seasons+"/"+season.ID.String())
			return
		}

		errs, ok := shared.Rejections(dateErr, err)
		if !ok {
			logger.Error("creating a season failed unexpectedly", "err", err)
			shared.WriteServerError(w)
			return
		}

		logger.Info("season create rejected", "reason", errs)
		form.Errors = errs
		shared.Render(w, r, http.StatusUnprocessableEntity, views.Season(form))
	}
	return handler
}

// Show renders the edit form of one season, and the audit trail of that
// season under it.
func Show(catalogSvc *catalog.Service, authSvc *auth.Service, log *audit.Log) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		season, ok := loadOne(w, r, catalogSvc)
		if !ok {
			return
		}

		trail, ok := shared.LoadTrail(w, r, authSvc, log, audit.EntitySeason, season.ID)
		if !ok {
			return
		}

		form := views.NewSeasonForm(shared.Chrome(r.Context()), season)
		form.Trail = trail
		if shared.IsSaved(r) {
			form.Notice = labels.Saved
		}
		shared.Render(w, r, http.StatusOK, views.Season(form))
	}
	return handler
}

// Save writes the name, the start date and the active flag.
func Save(catalogSvc *catalog.Service, authSvc *auth.Service, log *audit.Log) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		season, ok := loadOne(w, r, catalogSvc)
		if !ok {
			return
		}

		form := submitted(r)
		form.Chrome = shared.Chrome(ctx)
		form.SeasonID = season.ID.String()

		start, dateErr := shared.PostedDay(r, views.FieldSeasonStart)
		err := catalogSvc.UpdateSeason(ctx, season.ID, catalog.SeasonUpdateParams{
			Name:      form.Name,
			StartDate: start,
			Active:    form.Active,
		})
		if errors.Is(err, catalog.ErrSeasonNameTaken) {
			err = validate.Fail(views.FieldSeasonName, validate.Taken)
		}
		if err == nil && dateErr == nil {
			logger.Info("season updated", "season", season.ID)
			shared.RedirectSaved(w, r, paths.Seasons+"/"+form.SeasonID)
			return
		}

		errs, ok := shared.Rejections(dateErr, err)
		if !ok {
			logger.Error("updating a season failed unexpectedly", "season", season.ID, "err", err)
			shared.WriteServerError(w)
			return
		}

		// The screen comes back whole: the refusal, and the trail under it.
		trail, ok := shared.LoadTrail(w, r, authSvc, log, audit.EntitySeason, season.ID)
		if !ok {
			return
		}

		logger.Info("season update rejected", "season", season.ID, "reason", errs)
		form.Errors = errs
		form.Trail = trail
		shared.Render(w, r, http.StatusUnprocessableEntity, views.Season(form))
	}
	return handler
}

// loadOne reads the {id} of the route and returns the season it names. It
// answers 404 itself for an unreadable id and for a season that is not there,
// and then reports false.
func loadOne(w http.ResponseWriter, r *http.Request, catalogSvc *catalog.Service) (catalog.Season, bool) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)

	id, err := ids.Parse(r.PathValue("id"))
	if err != nil {
		logger.Info("the path names no readable season id", "id", r.PathValue("id"))
		http.NotFound(w, r)
		return catalog.Season{}, false
	}

	season, err := catalogSvc.Season(ctx, id)
	if errors.Is(err, catalog.ErrNoSeason) {
		logger.Info("no such season", "season", id)
		http.NotFound(w, r)
		return catalog.Season{}, false
	}
	if err != nil {
		logger.Error("loading a season failed", "season", id, "err", err)
		shared.WriteServerError(w)
		return catalog.Season{}, false
	}

	return season, true
}

// Active is the choices the drop form offers. A drop goes into a season
// that is still in work, so an inactive season is not on the list.
func Active(w http.ResponseWriter, r *http.Request, catalogSvc *catalog.Service) ([]catalog.Season, bool) {
	ctx := r.Context()

	seasons, err := catalogSvc.ListSeasons(ctx)
	if err != nil {
		logging.FromContext(ctx).Error("listing seasons failed", "err", err)
		shared.WriteServerError(w)
		return nil, false
	}

	active := make([]catalog.Season, 0, len(seasons))
	for _, season := range seasons {
		if season.Active {
			active = append(active, season)
		}
	}
	return active, true
}

// submitted reads the fields the create and the edit form post. The
// service checks the values: this only carries them.
func submitted(r *http.Request) views.SeasonForm {
	form := views.SeasonForm{
		Name:      r.PostFormValue(views.FieldSeasonName),
		StartDate: r.PostFormValue(views.FieldSeasonStart),
		Active:    shared.PostedFlag(r, views.FieldSeasonActive),
	}
	return form
}
