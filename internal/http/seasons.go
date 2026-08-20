package http

import (
	"errors"
	"net/http"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/http/views"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/logging"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

const (
	seasonsPath   = views.SeasonsPath
	seasonNewPath = seasonsPath + "/new"
	seasonPath    = seasonsPath + "/{id}"
)

// showSeasons lists every season, active and inactive, oldest first.
func showSeasons(catalogSvc *catalog.Service) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		seasons, err := catalogSvc.ListSeasons(ctx)
		if err != nil {
			logging.FromContext(ctx).Error("listing seasons failed", "err", err)
			writeServerError(w)
			return
		}

		page := views.SeasonsPage{Chrome: chrome(ctx), Seasons: seasons}
		render(w, r, http.StatusOK, views.Seasons(page))
	}
	return handler
}

// showNewSeason renders the empty create form.
func showNewSeason() http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		form := views.SeasonForm{Chrome: chrome(r.Context()), Active: true}
		render(w, r, http.StatusOK, views.Season(form))
	}
	return handler
}

// createSeason writes the season and opens the card of it.
func createSeason(catalogSvc *catalog.Service) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		form := submittedSeason(r)
		form.Chrome = chrome(ctx)

		start, dateErr := postedDay(r, views.FieldSeasonStart)
		season, err := catalogSvc.CreateSeason(ctx, catalog.SeasonCreateParams{
			Name:      form.Name,
			StartDate: start,
		})
		if errors.Is(err, catalog.ErrSeasonNameTaken) {
			err = validate.Fail(views.FieldSeasonName, validate.Taken)
		}
		if err == nil && dateErr == nil {
			logger.Info("season created", "season", season.ID, "name", season.Name)
			redirectSaved(w, r, seasonsPath+"/"+season.ID.String())
			return
		}

		errs, ok := rejections(dateErr, err)
		if !ok {
			logger.Error("creating a season failed unexpectedly", "err", err)
			writeServerError(w)
			return
		}

		logger.Info("season create rejected", "reason", errs)
		form.Errors = errs
		render(w, r, http.StatusUnprocessableEntity, views.Season(form))
	}
	return handler
}

// showSeason renders the edit form of one season, and the audit trail of that
// season under it.
func showSeason(catalogSvc *catalog.Service, authSvc *auth.Service, log *audit.Log) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		season, ok := loadSeason(w, r, catalogSvc)
		if !ok {
			return
		}

		trail, ok := loadTrail(w, r, authSvc, log, audit.EntitySeason, season.ID)
		if !ok {
			return
		}

		form := views.NewSeasonForm(chrome(r.Context()), season)
		form.Trail = trail
		if isSaved(r) {
			form.Notice = labels.Saved
		}
		render(w, r, http.StatusOK, views.Season(form))
	}
	return handler
}

// saveSeason writes the name, the start date and the active flag.
func saveSeason(catalogSvc *catalog.Service, authSvc *auth.Service, log *audit.Log) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		season, ok := loadSeason(w, r, catalogSvc)
		if !ok {
			return
		}

		form := submittedSeason(r)
		form.Chrome = chrome(ctx)
		form.SeasonID = season.ID.String()

		start, dateErr := postedDay(r, views.FieldSeasonStart)
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
			redirectSaved(w, r, seasonsPath+"/"+form.SeasonID)
			return
		}

		errs, ok := rejections(dateErr, err)
		if !ok {
			logger.Error("updating a season failed unexpectedly", "season", season.ID, "err", err)
			writeServerError(w)
			return
		}

		// The screen comes back whole: the refusal, and the trail under it.
		trail, ok := loadTrail(w, r, authSvc, log, audit.EntitySeason, season.ID)
		if !ok {
			return
		}

		logger.Info("season update rejected", "season", season.ID, "reason", errs)
		form.Errors = errs
		form.Trail = trail
		render(w, r, http.StatusUnprocessableEntity, views.Season(form))
	}
	return handler
}

// loadSeason reads the {id} of the route and returns the season it names. It
// answers 404 itself for an unreadable id and for a season that is not there,
// and then reports false.
func loadSeason(w http.ResponseWriter, r *http.Request, catalogSvc *catalog.Service) (catalog.Season, bool) {
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
		writeServerError(w)
		return catalog.Season{}, false
	}

	return season, true
}

// activeSeasons is the choices the drop form offers. A drop goes into a season
// that is still in work, so an inactive season is not on the list.
func activeSeasons(w http.ResponseWriter, r *http.Request, catalogSvc *catalog.Service) ([]catalog.Season, bool) {
	ctx := r.Context()

	seasons, err := catalogSvc.ListSeasons(ctx)
	if err != nil {
		logging.FromContext(ctx).Error("listing seasons failed", "err", err)
		writeServerError(w)
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

// submittedSeason reads the fields the create and the edit form post. The
// service checks the values: this only carries them.
func submittedSeason(r *http.Request) views.SeasonForm {
	form := views.SeasonForm{
		Name:      r.PostFormValue(views.FieldSeasonName),
		StartDate: r.PostFormValue(views.FieldSeasonStart),
		Active:    postedFlag(r, views.FieldSeasonActive),
	}
	return form
}
