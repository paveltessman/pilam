package http

import (
	"context"
	"errors"
	"mime/multipart"
	"net/http"

	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/http/middleware"
	"github.com/paveltessman/pilam/internal/http/views"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/logging"
	"github.com/paveltessman/pilam/internal/platform/media"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

const (
	modelsPath      = views.ModelsPath
	modelNewPath    = modelsPath + "/new"
	modelPath       = modelsPath + "/{id}"
	modelPhotosPath = modelPath + "/photos"
	modelOrderPath  = modelPhotosPath + "/order"
	modelPhotoPath  = modelPhotosPath + "/{photoID}/remove"
)

// How much of an upload is held in memory before multipart spills to disk. The
// media store caps a single image at 10 MiB and refuses anything over it.
const photoUploadMemory = 8 << 20

// showModels renders the plain list: the identity columns of every model the
// filter keeps.
//
// htmx asks for the same URL as the filter form.
func showModels(catalogSvc *catalog.Service, store media.Store) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		groups, ok := dropsBySeason(w, r, catalogSvc)
		if !ok {
			return
		}

		filter := submittedFilter(r, groups)
		models, err := catalogSvc.ListModels(ctx, catalog.ModelListParams{
			SeasonID: parsedID(filter.SeasonID),
			DropID:   parsedID(filter.DropID),
			Active:   flagOf(filter.Active),
		})
		if err != nil {
			logger.Error("listing models failed", "err", err)
			writeServerError(w)
			return
		}

		rows, ok := modelRows(w, r, catalogSvc, store, models, groups)
		if !ok {
			return
		}

		page := views.ModelsPage{
			Chrome:    chrome(ctx),
			Rows:      rows,
			Filter:    filter,
			CanCreate: holdsDrop(groups),
		}
		if middleware.IsFragment(ctx) {
			render(w, r, http.StatusOK, views.ModelsResults(page))
			return
		}
		render(w, r, http.StatusOK, views.Models(page))
	}
	return handler
}

// showNewModel renders the 'create' form.
//
// The season control narrows the drops, and htmx asks for this same URL to
// swap that one control.
func showNewModel(catalogSvc *catalog.Service) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		form := views.ModelForm{Chrome: chrome(ctx), SeasonID: r.URL.Query().Get(views.FieldModelSeason)}
		if !offerDrops(w, r, catalogSvc, &form) {
			return
		}

		if middleware.IsFragment(ctx) {
			render(w, r, http.StatusOK, views.ModelDropField(form))
			return
		}
		render(w, r, http.StatusOK, views.NewModel(form))
	}
	return handler
}

// createModel writes the model and opens the card of it.
func createModel(catalogSvc *catalog.Service) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		form := submittedModel(r)
		form.Chrome = chrome(ctx)

		dropID, idErr := postedID(r, views.FieldModelDrop)
		model, err := catalogSvc.CreateModel(ctx, catalog.ModelCreateParams{
			DropID:  dropID,
			Article: form.Article,
		})
		if errors.Is(err, catalog.ErrNoDrop) {
			err = validate.Fail(views.FieldModelDrop, validate.NotAllowed)
		}
		if err == nil && idErr == nil {
			logger.Info("model created", "model", model.ID, "drop", model.DropID, "article", model.Article)
			redirectSaved(w, r, modelsPath+"/"+model.ID.String())
			return
		}

		errs, ok := rejections(idErr, err)
		if !ok {
			logger.Error("creating a model failed unexpectedly", "err", err)
			writeServerError(w)
			return
		}

		if !offerDrops(w, r, catalogSvc, &form) {
			return
		}

		logger.Info("model create rejected", "reason", errs)
		form.Errors = errs
		render(w, r, http.StatusUnprocessableEntity, views.NewModel(form))
	}
	return handler
}

// showModel renders the screen header of one model.
func showModel(catalogSvc *catalog.Service, store media.Store) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		card, ok := modelCard(w, r, catalogSvc, store)
		if !ok {
			return
		}
		if isSaved(r) {
			card.Notice = labels.Saved
		}
		render(w, r, http.StatusOK, views.Model(card))
	}
	return handler
}

// saveModel writes the article and the active flag.
func saveModel(catalogSvc *catalog.Service, store media.Store) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		model, _, _, ok := loadModel(w, r, catalogSvc)
		if !ok {
			return
		}

		article := r.PostFormValue(views.FieldModelArticle)
		err := catalogSvc.UpdateModel(ctx, model.ID, catalog.ModelUpdateParams{
			Article: article,
			Active:  postedFlag(r, views.FieldModelActive),
		})
		if err == nil {
			logger.Info("model updated", "model", model.ID)
			redirectSaved(w, r, modelsPath+"/"+model.ID.String())
			return
		}

		errs, ok := rejections(err)
		if !ok {
			logger.Error("updating a model failed unexpectedly", "model", model.ID, "err", err)
			writeServerError(w)
			return
		}

		card, ok := modelCard(w, r, catalogSvc, store)
		if !ok {
			return
		}

		logger.Info("model update rejected", "model", model.ID, "reason", errs)
		card.Article = article
		card.Errors = errs
		render(w, r, http.StatusUnprocessableEntity, views.Model(card))
	}
	return handler
}

// addModelPhotos stores every uploaded file and appends it to the strip. The
// first photo of a model is its cover.
//
// One refused file stops the upload there. The files stored before it keep
// their place on the strip, because each photo is a write of its own.
func addModelPhotos(catalogSvc *catalog.Service, store media.Store) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		model, _, _, ok := loadModel(w, r, catalogSvc)
		if !ok {
			return
		}

		if err := r.ParseMultipartForm(photoUploadMemory); err != nil {
			logger.Info("the upload is not a readable form", "model", model.ID, "err", err)
			refusePhotos(w, r, catalogSvc, store, labels.ModelsPhotoNone)
			return
		}
		defer func() { _ = r.MultipartForm.RemoveAll() }()

		files := r.MultipartForm.File[views.FieldModelPhoto]
		if len(files) == 0 {
			logger.Info("the upload names no file", "model", model.ID)
			refusePhotos(w, r, catalogSvc, store, labels.ModelsPhotoNone)
			return
		}

		for _, file := range files {
			alert, err := addOnePhoto(ctx, catalogSvc, store, model.ID, file)
			switch {
			case alert != "":
				logger.Info("photo refused", "model", model.ID, "file", file.Filename, "err", err)
				refusePhotos(w, r, catalogSvc, store, alert)
				return
			case err != nil:
				logger.Error("storing a photo failed", "model", model.ID, "file", file.Filename, "err", err)
				writeServerError(w)
				return
			}
		}

		logger.Info("photos added", "model", model.ID, "count", len(files))
		redirectSaved(w, r, modelsPath+"/"+model.ID.String())
	}
	return handler
}

// addOnePhoto stores one uploaded file and appends it to the strip.
//
// It returns the sentence the screen reports when the file is one the store
// refuses, and an error alone when the failure is ours.
func addOnePhoto(
	ctx context.Context,
	catalogSvc *catalog.Service,
	store media.Store,
	modelID ids.ID,
	file *multipart.FileHeader,
) (string, error) {
	content, err := file.Open()
	if err != nil {
		return "", err
	}
	defer func() { _ = content.Close() }()

	stored, err := store.Put(ctx, media.Blob{Kind: media.Image, Name: file.Filename, Content: content})
	switch {
	case errors.Is(err, media.ErrTooLarge):
		return labels.ModelsPhotoTooLarge, err
	case errors.Is(err, media.ErrTypeRejected):
		return labels.ModelsPhotoRejected, err
	case err != nil:
		return "", err
	}

	if _, err := catalogSvc.AddPhoto(ctx, modelID, stored.Key); err != nil {
		return "", err
	}
	return "", nil
}

// removeModelPhoto deletes one photo and closes the gap it leaves, so the
// strip still names its cover at position 0.
func removeModelPhoto(catalogSvc *catalog.Service) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		model, _, _, ok := loadModel(w, r, catalogSvc)
		if !ok {
			return
		}

		photoID, err := ids.Parse(r.PathValue("photoID"))
		if err != nil {
			logger.Info("the path names no readable photo id", "photo", r.PathValue("photoID"))
			http.NotFound(w, r)
			return
		}

		err = catalogSvc.RemovePhoto(ctx, model.ID, photoID)
		if errors.Is(err, catalog.ErrNoPhoto) {
			logger.Info("no such photo", "model", model.ID, "photo", photoID)
			http.NotFound(w, r)
			return
		}
		if err != nil {
			logger.Error("removing a photo failed", "model", model.ID, "photo", photoID, "err", err)
			writeServerError(w)
			return
		}

		logger.Info("photo removed", "model", model.ID, "photo", photoID)
		redirectSaved(w, r, modelsPath+"/"+model.ID.String())
	}
	return handler
}

// reorderModelPhotos writes the strip in the posted order. The first
// identifier becomes the cover.
func reorderModelPhotos(catalogSvc *catalog.Service, store media.Store) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		model, _, _, ok := loadModel(w, r, catalogSvc)
		if !ok {
			return
		}

		ordered, err := postedOrder(r, views.FieldModelOrder)
		if err == nil {
			err = catalogSvc.ReorderPhotos(ctx, model.ID, ordered)
		}
		switch {
		case err == nil:
			logger.Info("photos reordered", "model", model.ID, "count", len(ordered))
			redirectSaved(w, r, modelsPath+"/"+model.ID.String())
			return

		case errors.Is(err, catalog.ErrPhotoOrder), errors.Is(err, ids.ErrInvalid):
			// The order names a strip the model no longer holds, which is a
			// screen the user opened before somebody else changed it.
			logger.Info("photo order refused", "model", model.ID, "err", err)
			refusePhotos(w, r, catalogSvc, store, labels.ModelsPhotoOrder)
			return

		default:
			logger.Error("reordering the photos failed", "model", model.ID, "err", err)
			writeServerError(w)
		}
	}
	return handler
}

// refusePhotos renders the card again with the alert the refusal reads as.
func refusePhotos(w http.ResponseWriter, r *http.Request, catalogSvc *catalog.Service, store media.Store, alert string) {
	card, ok := modelCard(w, r, catalogSvc, store)
	if !ok {
		return
	}
	card.Alert = alert
	render(w, r, http.StatusUnprocessableEntity, views.Model(card))
}

// modelCard reads everything the model screen header shows. It answers itself and reports
// false when the model is not there or cannot be read.
func modelCard(w http.ResponseWriter, r *http.Request, catalogSvc *catalog.Service, store media.Store) (views.ModelCard, bool) {
	ctx := r.Context()

	model, drop, season, ok := loadModel(w, r, catalogSvc)
	if !ok {
		return views.ModelCard{}, false
	}

	photos, err := catalogSvc.ListPhotos(ctx, model.ID)
	if err != nil {
		logging.FromContext(ctx).Error("listing the photos of a model failed", "model", model.ID, "err", err)
		writeServerError(w)
		return views.ModelCard{}, false
	}

	card := views.ModelCard{
		Chrome:  chrome(ctx),
		ModelID: model.ID.String(),
		Article: model.Article,
		Active:  model.Active,
		Season:  season,
		Drop:    drop,
		Photos:  views.NewModelStrip(photos, store.URL),
	}
	return card, true
}

// loadModel reads the {id} of the route and returns the model it names, with
// the drop and the season that hold it. It answers 404 itself for an unreadable
// id and for a model that is not there, and then reports false.
func loadModel(w http.ResponseWriter, r *http.Request, catalogSvc *catalog.Service) (catalog.Model, catalog.Drop, catalog.Season, bool) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)

	id, err := ids.Parse(r.PathValue("id"))
	if err != nil {
		logger.Info("the path names no readable model id", "id", r.PathValue("id"))
		http.NotFound(w, r)
		return catalog.Model{}, catalog.Drop{}, catalog.Season{}, false
	}

	model, err := catalogSvc.Model(ctx, id)
	if errors.Is(err, catalog.ErrNoModel) {
		logger.Info("no such model", "model", id)
		http.NotFound(w, r)
		return catalog.Model{}, catalog.Drop{}, catalog.Season{}, false
	}
	if err != nil {
		logger.Error("loading a model failed", "model", id, "err", err)
		writeServerError(w)
		return catalog.Model{}, catalog.Drop{}, catalog.Season{}, false
	}

	// A model references a drop and the drop a season, so both rows are there.
	// A missing one is a broken reference, which is a failure rather than a 404.
	drop, err := catalogSvc.Drop(ctx, model.DropID)
	if err != nil {
		logger.Error("loading the drop of a model failed", "model", id, "drop", model.DropID, "err", err)
		writeServerError(w)
		return catalog.Model{}, catalog.Drop{}, catalog.Season{}, false
	}

	season, err := catalogSvc.Season(ctx, drop.SeasonID)
	if err != nil {
		logger.Error("loading the season of a model failed", "model", id, "season", drop.SeasonID, "err", err)
		writeServerError(w)
		return catalog.Model{}, catalog.Drop{}, catalog.Season{}, false
	}

	return model, drop, season, true
}

// modelRows names the drop and the season of every model, and reads the cover
// photo of the whole page in one go.
func modelRows(
	w http.ResponseWriter,
	r *http.Request,
	catalogSvc *catalog.Service,
	store media.Store,
	models []catalog.Model,
	groups []views.DropGroup,
) ([]views.ModelRow, bool) {
	ctx := r.Context()

	modelIDs := make([]ids.ID, len(models))
	for i, model := range models {
		modelIDs[i] = model.ID
	}

	covers, err := catalogSvc.Thumbnails(ctx, modelIDs...)
	if err != nil {
		logging.FromContext(ctx).Error("listing the thumbnails of the model list failed", "err", err)
		writeServerError(w)
		return nil, false
	}

	seasonOf := make(map[ids.ID]catalog.Season, len(groups))
	dropOf := make(map[ids.ID]catalog.Drop)
	for _, group := range groups {
		for _, drop := range group.Drops {
			dropOf[drop.ID] = drop
			seasonOf[drop.ID] = group.Season
		}
	}

	rows := make([]views.ModelRow, len(models))
	for i, model := range models {
		rows[i] = views.ModelRow{
			Model:  model,
			Season: seasonOf[model.DropID],
			Drop:   dropOf[model.DropID],
		}
		if cover, held := covers[model.ID]; held {
			rows[i].Thumbnail = store.URL(cover.MediaKey)
		}
	}
	return rows, true
}

// offerDrops fills the two controls of the create form: the active seasons, and
// the drops the chosen season holds. A form with no season chosen offers every
// active drop.
//
// A model goes into a drop that is still in work, so a season or a drop that is
// turned off is not on the list.
func offerDrops(w http.ResponseWriter, r *http.Request, catalogSvc *catalog.Service, form *views.ModelForm) bool {
	groups, ok := dropsBySeason(w, r, catalogSvc)
	if !ok {
		return false
	}

	chosen := parsedID(form.SeasonID)
	for _, group := range groups {
		if !group.Season.Active {
			continue
		}
		form.Seasons = append(form.Seasons, group.Season)

		if chosen != ids.Nil && group.Season.ID != chosen {
			continue
		}
		active := make([]catalog.Drop, 0, len(group.Drops))
		for _, drop := range group.Drops {
			if drop.Active {
				active = append(active, drop)
			}
		}
		form.Drops = append(form.Drops, views.DropGroup{Season: group.Season, Drops: active})
	}
	return true
}

// holdsDrop reports whether any season holds an active drop.
func holdsDrop(groups []views.DropGroup) bool {
	for _, group := range groups {
		if !group.Season.Active {
			continue
		}
		for _, drop := range group.Drops {
			if drop.Active {
				return true
			}
		}
	}
	return false
}

// submittedFilter reads the three controls above the list, and the choices they
// offer. A value that names nothing is dropped, so the control comes back on
// the choice that filters nothing out.
func submittedFilter(r *http.Request, groups []views.DropGroup) views.ModelFilter {
	query := r.URL.Query()
	filter := views.ModelFilter{
		SeasonID: query.Get(views.FieldModelSeason),
		DropID:   query.Get(views.FieldModelDrop),
		Active:   query.Get(views.FieldModelActive),
		Drops:    groups,
	}

	for _, group := range groups {
		filter.Seasons = append(filter.Seasons, group.Season)
	}
	if parsedID(filter.SeasonID) == ids.Nil {
		filter.SeasonID = ""
	}
	if parsedID(filter.DropID) == ids.Nil {
		filter.DropID = ""
	}
	if flagOf(filter.Active) == nil {
		filter.Active = ""
	}
	return filter
}

// submittedModel reads what the create form posts. The service checks the
// values: this only carries them.
func submittedModel(r *http.Request) views.ModelForm {
	form := views.ModelForm{
		SeasonID: r.PostFormValue(views.FieldModelSeason),
		DropID:   r.PostFormValue(views.FieldModelDrop),
		Article:  r.PostFormValue(views.FieldModelArticle),
	}
	return form
}
