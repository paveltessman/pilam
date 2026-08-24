package milestones

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"slices"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/http/milestones/views"
	"github.com/paveltessman/pilam/internal/http/paths"
	"github.com/paveltessman/pilam/internal/http/shared"
	sharedviews "github.com/paveltessman/pilam/internal/http/shared/views"
	"github.com/paveltessman/pilam/internal/milestones"
	"github.com/paveltessman/pilam/internal/platform/date"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/logging"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

// ShowNewTemplate renders the empty create form.
func ShowNewTemplate() http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		form := views.TemplateForm{Chrome: shared.Chrome(r.Context()), Active: true}
		shared.Render(w, r, http.StatusOK, views.Template(form))
	}
	return handler
}

// CreateTemplate writes the template and opens the editor of it. A new template
// holds no items: the editor adds them.
func CreateTemplate(milestoneSvc *milestones.Service) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		form := submittedTemplate(r)
		form.Chrome = shared.Chrome(ctx)

		template, err := milestoneSvc.CreateTemplate(ctx, milestones.TemplateCreateParams{
			Name:        form.Name,
			Description: form.Description,
			Default:     form.Default,
		})
		if errors.Is(err, milestones.ErrNameTaken) {
			err = validate.Fail(views.FieldName, validate.Taken)
		}
		if err == nil {
			logger.Info("milestone template created", "template", template.ID, "name", template.Name)
			shared.RedirectSaved(w, r, paths.MilestoneTemplates+"/"+template.ID.String())
			return
		}

		errs, ok := shared.Rejections(err)
		if !ok {
			logger.Error("creating a milestone template failed unexpectedly", "err", err)
			shared.WriteServerError(w)
			return
		}

		logger.Info("milestone template create rejected", "reason", errs)
		form.Errors = errs
		shared.Render(w, r, http.StatusUnprocessableEntity, views.Template(form))
	}
	return handler
}

// ShowTemplate renders the editor: the two fields of the template, the ordered
// items with the preview beside them, and the audit trail under it all.
func ShowTemplate(milestoneSvc *milestones.Service, authSvc *auth.Service, log *audit.Log) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		template, ok := loadTemplate(w, r, milestoneSvc)
		if !ok {
			return
		}

		form := views.NewTemplateForm(shared.Chrome(r.Context()), template)
		if !fillEditor(w, r, milestoneSvc, authSvc, log, &form, previewTarget(r)) {
			return
		}
		if shared.IsSaved(r) {
			form.Notice = labels.Saved
		}
		shared.Render(w, r, http.StatusOK, views.Template(form))
	}
	return handler
}

// SaveTemplate writes the name, the description, the default flag and the
// active flag.
func SaveTemplate(milestoneSvc *milestones.Service, authSvc *auth.Service, log *audit.Log) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		template, ok := loadTemplate(w, r, milestoneSvc)
		if !ok {
			return
		}

		form := submittedTemplate(r)
		form.Chrome = shared.Chrome(ctx)
		form.TemplateID = template.ID.String()

		err := milestoneSvc.UpdateTemplate(ctx, template.ID, milestones.TemplateUpdateParams{
			Name:        form.Name,
			Description: form.Description,
			Default:     form.Default,
			Active:      form.Active,
		})
		if errors.Is(err, milestones.ErrNameTaken) {
			err = validate.Fail(views.FieldName, validate.Taken)
		}
		if err == nil {
			logger.Info("milestone template updated", "template", template.ID)
			redirectToTemplate(w, r, form.TemplateID, form.Target)
			return
		}

		errs, ok := shared.Rejections(err)
		if !ok {
			logger.Error("updating a milestone template failed unexpectedly",
				"template", template.ID, "err", err)
			shared.WriteServerError(w)
			return
		}

		logger.Info("milestone template update rejected", "template", template.ID, "reason", errs)
		form.Errors = errs
		refuse(w, r, milestoneSvc, authSvc, log, form)
	}
	return handler
}

// AddItem appends one step to the template. The step lands at the end of the
// list, and the user moves it from there.
func AddItem(milestoneSvc *milestones.Service, authSvc *auth.Service, log *audit.Log) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		template, ok := loadTemplate(w, r, milestoneSvc)
		if !ok {
			return
		}

		typeID, typeErr := shared.PostedID(r, views.FieldType)
		offset, _, offsetErr := shared.PostedNumber(r, views.FieldOffset)

		err := errors.Join(typeErr, offsetErr)
		if err == nil {
			_, err = milestoneSvc.AddTemplateItem(ctx, template.ID, typeID, offset)
		}
		if errors.Is(err, milestones.ErrTypeInTemplate) {
			err = validate.Fail(views.FieldType, validate.Taken)
		}
		if err == nil {
			logger.Info("milestone template item added", "template", template.ID, "type", typeID)
			redirectToTemplate(w, r, template.ID.String(), postedTarget(r))
			return
		}

		errs, ok := shared.Rejections(typeErr, offsetErr, err)
		if !ok {
			logger.Error("adding a milestone template item failed unexpectedly",
				"template", template.ID, "err", err)
			shared.WriteServerError(w)
			return
		}

		logger.Info("milestone template item rejected", "template", template.ID, "reason", errs)
		form := views.NewTemplateForm(shared.Chrome(ctx), template)
		form.Errors = errs
		form.Target = postedTarget(r)
		refuse(w, r, milestoneSvc, authSvc, log, form)
	}
	return handler
}

// SaveItem moves one item of the template. The row carries the offset and the
// gap, and the user edits either one.
//
// The offset wins when both moved: it is what the app stores, and the gap is
// the reading of it.
func SaveItem(milestoneSvc *milestones.Service, authSvc *auth.Service, log *audit.Log) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		template, ok := loadTemplate(w, r, milestoneSvc)
		if !ok {
			return
		}
		itemID, ok := itemOf(w, r)
		if !ok {
			return
		}

		offset, hasOffset, offsetErr := shared.PostedNumber(r, views.FieldOffset)
		gap, hasGap, gapErr := shared.PostedNumber(r, views.FieldGap)

		err := errors.Join(offsetErr, gapErr)
		if err == nil {
			err = moveItem(ctx, milestoneSvc, template.ID, itemID, submittedMove{
				Offset: offset, HasOffset: hasOffset, Gap: gap, HasGap: hasGap,
			})
		}
		if errors.Is(err, milestones.ErrNoItem) {
			logger.Info("no such milestone template item", "template", template.ID, "item", itemID)
			http.NotFound(w, r)
			return
		}
		if err == nil {
			logger.Info("milestone template item moved", "template", template.ID, "item", itemID)
			redirectToTemplate(w, r, template.ID.String(), postedTarget(r))
			return
		}

		errs, ok := shared.Rejections(err)
		if !ok {
			logger.Error("moving a milestone template item failed unexpectedly",
				"template", template.ID, "item", itemID, "err", err)
			shared.WriteServerError(w)
			return
		}

		logger.Info("milestone template item move rejected",
			"template", template.ID, "item", itemID, "reason", errs)
		form := views.NewTemplateForm(shared.Chrome(ctx), template)
		form.Errors = errs
		form.Target = postedTarget(r)
		refuse(w, r, milestoneSvc, authSvc, log, form)
	}
	return handler
}

// submittedMove is what one row of the item table posts.
type submittedMove struct {
	Offset    int
	HasOffset bool
	Gap       int
	HasGap    bool
}

// moveItem picks the column the user edited. The row states both, so the one
// that differs from what the screen rendered is the edit.
func moveItem(
	ctx context.Context,
	milestoneSvc *milestones.Service,
	templateID, itemID ids.ID,
	posted submittedMove,
) error {
	items, err := milestoneSvc.TemplateItems(ctx, templateID)
	if err != nil {
		return err
	}
	at := slices.IndexFunc(items, func(i milestones.TemplateItem) bool { return i.ID == itemID })
	if at < 0 {
		return milestones.ErrNoItem
	}

	if posted.HasOffset && posted.Offset != items[at].Offset {
		return milestoneSvc.SetTemplateItemOffset(ctx, templateID, itemID, posted.Offset)
	}
	if posted.HasGap && at > 0 && posted.Gap != milestones.Gaps(items)[at] {
		return milestoneSvc.SetTemplateItemGap(ctx, templateID, itemID, posted.Gap)
	}
	return nil
}

// RemoveItem deletes one step of the template.
func RemoveItem(milestoneSvc *milestones.Service) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		template, ok := loadTemplate(w, r, milestoneSvc)
		if !ok {
			return
		}
		itemID, ok := itemOf(w, r)
		if !ok {
			return
		}

		err := milestoneSvc.RemoveTemplateItem(ctx, template.ID, itemID)
		if errors.Is(err, milestones.ErrNoItem) {
			logger.Info("no such milestone template item", "template", template.ID, "item", itemID)
			http.NotFound(w, r)
			return
		}
		if err != nil {
			logger.Error("removing a milestone template item failed",
				"template", template.ID, "item", itemID, "err", err)
			shared.WriteServerError(w)
			return
		}

		logger.Info("milestone template item removed", "template", template.ID, "item", itemID)
		redirectToTemplate(w, r, template.ID.String(), postedTarget(r))
	}
	return handler
}

// ReorderItems writes the items of the template in the order the move button
// posted.
func ReorderItems(milestoneSvc *milestones.Service, authSvc *auth.Service, log *audit.Log) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)

		template, ok := loadTemplate(w, r, milestoneSvc)
		if !ok {
			return
		}

		ordered, err := shared.PostedOrder(r, views.FieldItemOrder)
		if err == nil {
			err = milestoneSvc.ReorderTemplateItems(ctx, template.ID, ordered)
		}
		if err == nil {
			logger.Info("milestone template items reordered", "template", template.ID)
			redirectToTemplate(w, r, template.ID.String(), postedTarget(r))
			return
		}

		// An order that names the wrong items is a screen the user left open
		// while the template moved on. The editor comes back whole, with the
		// items as they stand now.
		if !errors.Is(err, milestones.ErrItemOrder) && !errors.Is(err, ids.ErrInvalid) {
			logger.Error("reordering the milestone template items failed",
				"template", template.ID, "err", err)
			shared.WriteServerError(w)
			return
		}

		logger.Info("milestone template item order refused", "template", template.ID, "err", err)
		form := views.NewTemplateForm(shared.Chrome(ctx), template)
		form.Alert = labels.MilestoneItemsStale
		form.Target = postedTarget(r)
		refuse(w, r, milestoneSvc, authSvc, log, form)
	}
	return handler
}

// refuse renders the editor with the refusal on it. Every write of the section
// answers this way: the screen comes back whole, with the items and the trail
// as they stand.
func refuse(
	w http.ResponseWriter,
	r *http.Request,
	milestoneSvc *milestones.Service,
	authSvc *auth.Service,
	log *audit.Log,
	form views.TemplateForm,
) {
	if !fillEditor(w, r, milestoneSvc, authSvc, log, &form, form.Target) {
		return
	}
	shared.Render(w, r, http.StatusUnprocessableEntity, views.Template(form))
}

// fillEditor loads everything the editor shows beside the two fields: the
// items, the types the add control offers, the preview, and the trail.
//
// It answers 500 itself and reports false when any of it cannot be read.
func fillEditor(
	w http.ResponseWriter,
	r *http.Request,
	milestoneSvc *milestones.Service,
	authSvc *auth.Service,
	log *audit.Log,
	form *views.TemplateForm,
	target string,
) bool {
	ctx := r.Context()
	logger := logging.FromContext(ctx)

	templateID, err := ids.Parse(form.TemplateID)
	if err != nil {
		logger.Error("the editor names no readable template id", "id", form.TemplateID)
		shared.WriteServerError(w)
		return false
	}

	items, err := milestoneSvc.TemplateItems(ctx, templateID)
	if err != nil {
		logger.Error("listing the items of a milestone template failed", "template", templateID, "err", err)
		shared.WriteServerError(w)
		return false
	}

	types, err := milestoneSvc.ListTypes(ctx)
	if err != nil {
		logger.Error("listing milestone types failed", "err", err)
		shared.WriteServerError(w)
		return false
	}

	trail, ok := shared.LoadTrail(w, r, authSvc, log, audit.EntityMilestoneTemplate, templateID)
	if !ok {
		return false
	}

	day, _ := date.Parse(target)
	form.Target = target
	form.Items = views.NewTemplateRows(items, typeNames(types), day)
	form.Choices = typeChoices(types, items)
	form.Trail = trail
	return true
}

// typeNames is the short name of every type, keyed by the identifier the view
// reads it by.
func typeNames(types []milestones.Type) map[string]string {
	names := make(map[string]string, len(types))
	for _, milestoneType := range types {
		names[milestoneType.ID.String()] = milestoneType.Name
	}
	return names
}

// typeChoices is what the add control offers: every active type the template
// does not hold yet. A retired type leaves the pickers.
func typeChoices(types []milestones.Type, held []milestones.TemplateItem) []sharedviews.SelectOption {
	inTemplate := make(map[ids.ID]bool, len(held))
	for _, item := range held {
		inTemplate[item.TypeID] = true
	}

	var choices []sharedviews.SelectOption
	for _, milestoneType := range types {
		if !milestoneType.Active || inTemplate[milestoneType.ID] {
			continue
		}
		choices = append(choices, sharedviews.SelectOption{
			Value: milestoneType.ID.String(),
			Label: milestoneType.Name,
		})
	}
	return choices
}

// loadTemplate reads the {id} of the route and returns the template it names.
// It answers 404 itself for an unreadable id and for a template that is not
// there, and then reports false.
func loadTemplate(
	w http.ResponseWriter,
	r *http.Request,
	milestoneSvc *milestones.Service,
) (milestones.Template, bool) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)

	id, err := ids.Parse(r.PathValue("id"))
	if err != nil {
		logger.Info("the path names no readable milestone template id", "id", r.PathValue("id"))
		http.NotFound(w, r)
		return milestones.Template{}, false
	}

	template, err := milestoneSvc.Template(ctx, id)
	if errors.Is(err, milestones.ErrNoTemplate) {
		logger.Info("no such milestone template", "template", id)
		http.NotFound(w, r)
		return milestones.Template{}, false
	}
	if err != nil {
		logger.Error("loading a milestone template failed", "template", id, "err", err)
		shared.WriteServerError(w)
		return milestones.Template{}, false
	}

	return template, true
}

// itemOf reads the {itemID} of the route. It answers 404 itself for an
// identifier it cannot read, and then reports false.
func itemOf(w http.ResponseWriter, r *http.Request) (ids.ID, bool) {
	id, err := ids.Parse(r.PathValue("itemID"))
	if err != nil {
		logging.FromContext(r.Context()).Info("the path names no readable template item id",
			"id", r.PathValue("itemID"))
		http.NotFound(w, r)
		return ids.Nil, false
	}
	return id, true
}

// submittedTemplate reads the fields the create and the edit form post.
// The service checks the values: this only carries them.
func submittedTemplate(r *http.Request) views.TemplateForm {
	form := views.TemplateForm{
		Name:        r.PostFormValue(views.FieldName),
		Description: r.PostFormValue(views.FieldDescription),
		Default:     shared.PostedFlag(r, views.FieldDefault),
		Active:      shared.PostedFlag(r, views.FieldActive),
		Target:      postedTarget(r),
	}
	return form
}

// previewTarget is the day the preview reckons from, as the query names it. A
// value that is not a date reads as no date at all.
func previewTarget(r *http.Request) string {
	return readableDay(r.URL.Query().Get(views.FieldTarget))
}

// postedTarget is the preview date a form of the editor carries back.
func postedTarget(r *http.Request) string {
	return readableDay(r.PostFormValue(views.FieldTarget))
}

func readableDay(raw string) string {
	if _, err := date.Parse(raw); err != nil {
		return ""
	}
	return raw
}

// redirectToTemplate sends the browser back to the editor, keeping the preview
// date it was reading.
func redirectToTemplate(w http.ResponseWriter, r *http.Request, templateID, target string) {
	path := paths.MilestoneTemplates + "/" + templateID
	if target != "" {
		path += "?" + views.FieldTarget + "=" + url.QueryEscape(target)
	}
	shared.RedirectSaved(w, r, path)
}
