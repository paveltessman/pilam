package http

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/http/views"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

var modelRoutes = []route{
	{http.MethodGet, modelsPath},
	{http.MethodPost, modelsPath},
	{http.MethodGet, modelsPath + "/new"},
	{http.MethodGet, modelsPath + "/" + missingID.String()},
	{http.MethodPost, modelsPath + "/" + missingID.String()},
	{http.MethodPost, modelsPath + "/" + missingID.String() + "/photos"},
	{http.MethodPost, modelsPath + "/" + missingID.String() + "/photos/order"},
	{http.MethodPost, modelsPath + "/" + missingID.String() + "/photos/" + missingID.String() + "/remove"},
}

// The bytes of the two files the upload tests post. The store settles the type
// by reading them, so the PNG has to start like one.
const (
	pngUpload = "\x89PNG\r\n\x1a\n" + "and the rest of a png"
	textFile  = "this file is not an image at all"
)

// modelForm is what the create screen and the card post.
func modelForm(dropID, article string, active bool) url.Values {
	form := url.Values{
		views.FieldModelDrop:    {dropID},
		views.FieldModelArticle: {article},
	}
	if active {
		form.Set(views.FieldModelActive, "true")
	}
	return form
}

// createdModel posts the create form and returns the id of the model it wrote.
func createdModel(t *testing.T, d Deps, cookie *http.Cookie, dropID, article string) string {
	t.Helper()

	rec := postAs(t, d, modelsPath, modelForm(dropID, article, true), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("creating model %q: status = %d, want %d: %s", article, rec.Code, http.StatusSeeOther, rec.Body)
	}
	return idOfRedirect(t, rec, modelsPath)
}

// spine writes the season and the drop a model needs to exist. Only a root
// reaches those two sections, so it logs one in of its own.
func spine(t *testing.T, d Deps) string {
	t.Helper()

	root := loggedIn(t, d, rootEmail, rootPasswd)
	seasonID := createdSeason(t, d, root, "S1", "2026-11-01")
	return createdDrop(t, d, root, seasonID, "Drop 1", "2027-02-15")
}

// upload posts files the way the photo control does: one multipart form, one
// part per file, all under the same field name.
func upload(t *testing.T, d Deps, path string, files []string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	for i, content := range files {
		part, err := form.CreateFormFile(views.FieldModelPhoto, "photo-"+string(rune('a'+i))+".png")
		if err != nil {
			t.Fatalf("building the upload: %v", err)
		}
		if _, err := part.Write([]byte(content)); err != nil {
			t.Fatalf("building the upload: %v", err)
		}
	}
	if err := form.Close(); err != nil {
		t.Fatalf("building the upload: %v", err)
	}

	r := httptest.NewRequest(http.MethodPost, path, &body)
	r.Header.Set("Content-Type", form.FormDataContentType())
	r.AddCookie(cookie)

	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, r)
	return rec
}

// photoIDs is the strip of one model, in order, as the card renders it.
func photoIDs(t *testing.T, d Deps, cookie *http.Cookie, modelID string) []string {
	t.Helper()

	body := getAs(t, d, modelsPath+"/"+modelID, cookie).Body.String()

	// Every photo carries a remove form named after it, and the forms stand in
	// the order the strip does. The identifier is what the action ends with.
	const suffix = `/remove"`

	var strip []string
	for rest := body; ; {
		before, remainder, found := strings.Cut(rest, suffix)
		if !found {
			return strip
		}
		id := before[len(before)-len(missingID.String()):]
		if _, err := ids.Parse(id); err != nil {
			t.Fatalf("the card names no readable photo id before %q: %q", suffix, id)
		}
		strip = append(strip, id)
		rest = remainder
	}
}

// orderForm posts a strip as the move buttons do.
func orderForm(strip []string) url.Values {
	return url.Values{views.FieldModelOrder: strip}
}

func TestModelScreenNamesFieldsThatServiceRejects(t *testing.T) {
	pairs := [][2]string{
		{views.FieldModelArticle, catalog.FieldArticle},
		{views.FieldModelSeason, catalog.FieldSeason},
		{views.FieldModelDrop, catalog.FieldDrop},
		{views.FieldModelActive, catalog.FieldActive},
		{views.FieldModelPhoto, catalog.FieldPhoto},
		{views.FieldModelOrder, catalog.FieldPhotoOrder},
	}
	for _, pair := range pairs {
		if pair[0] != pair[1] {
			t.Errorf("the view posts %q, the service rejects %q", pair[0], pair[1])
		}
	}
}

func TestModelSectionNeedsLogin(t *testing.T) {
	d := deps(t)

	for _, r := range modelRoutes {
		rec := call(t, d, r, nil, nil)
		if rec.Code != http.StatusSeeOther {
			t.Errorf("%s %s status = %d, want %d", r.method, r.path, rec.Code, http.StatusSeeOther)
			continue
		}
		if got := rec.Header().Get("Location"); got != loginPath {
			t.Errorf("%s %s Location = %q, want %q", r.method, r.path, got, loginPath)
		}
	}
}

func TestModelSectionLetsAMemberIn(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, testEmail, testPasswd)

	for _, r := range modelRoutes {
		rec := call(t, d, r, modelForm(missingID.String(), "A-100", true), cookie)
		if rec.Code == http.StatusForbidden {
			t.Errorf("%s %s answers %d to a member", r.method, r.path, rec.Code)
		}
	}
}

// The nav offers the list to a member, who reaches no other section.
func TestNavOffersTheModelListToEverybody(t *testing.T) {
	d := deps(t)

	member := getAs(t, d, modelsPath, loggedIn(t, d, testEmail, testPasswd)).Body.String()
	wants(t, member, `href="`+modelsPath+`"`, labels.ModelsTitle)
}

// The root path holds no screen of its own.
func TestRootPathSendsTheBrowserToTheList(t *testing.T) {
	d := deps(t)

	rec := getAs(t, d, "/", loggedIn(t, d, testEmail, testPasswd))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != modelsPath {
		t.Errorf("Location = %q, want %q", got, modelsPath)
	}
}

func TestModelsListIsEmptyBeforeAnyModel(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, testEmail, testPasswd)

	rec := getAs(t, d, modelsPath, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	wants(t, rec.Body.String(), labels.ModelsEmpty)
}

// Without a drop there is nothing to put a model in, so the list offers no
// create link and the form says what to do first.
func TestModelCreateWaitsForADrop(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, testEmail, testPasswd)

	if body := getAs(t, d, modelsPath, cookie).Body.String(); strings.Contains(body, labels.ActionAddModel) {
		t.Error("the list offers a create link with no drop to create into")
	}
	wants(t, getAs(t, d, modelNewPath, cookie).Body.String(), labels.ModelsNoDrop)
}

func TestModelsListShowsTheIdentityColumns(t *testing.T) {
	d := deps(t)
	dropID := spine(t, d)
	cookie := loggedIn(t, d, testEmail, testPasswd)

	createdModel(t, d, cookie, dropID, "A-100")

	body := getAs(t, d, modelsPath, cookie).Body.String()
	wants(t, body, "A-100", "S1", "Drop 1", labels.StateActive, labels.ActionAddModel)
}

func TestModelsListOrdersByArticle(t *testing.T) {
	d := deps(t)
	dropID := spine(t, d)
	cookie := loggedIn(t, d, testEmail, testPasswd)

	createdModel(t, d, cookie, dropID, "A-200")
	createdModel(t, d, cookie, dropID, "A-100")

	body := getAs(t, d, modelsPath, cookie).Body.String()
	if strings.Index(body, "A-100") > strings.Index(body, "A-200") {
		t.Error("the list does not order by the article")
	}
}

func TestModelsListFiltersOnSeasonDropAndState(t *testing.T) {
	d := deps(t)
	root := loggedIn(t, d, rootEmail, rootPasswd)
	firstSeason := createdSeason(t, d, root, "S1", "2026-11-01")
	secondSeason := createdSeason(t, d, root, "S2", "2027-05-01")
	firstDrop := createdDrop(t, d, root, firstSeason, "Drop 1", "2027-02-15")
	secondDrop := createdDrop(t, d, root, secondSeason, "Drop 2", "2027-08-15")

	cookie := loggedIn(t, d, testEmail, testPasswd)
	kept := createdModel(t, d, cookie, firstDrop, "A-100")
	createdModel(t, d, cookie, secondDrop, "A-200")

	// The season filter goes through the drop: the model holds no season.
	testData := map[string]string{
		"by season": "?" + views.FieldModelSeason + "=" + firstSeason,
		"by drop":   "?" + views.FieldModelDrop + "=" + firstDrop,
	}
	for name, query := range testData {
		t.Run(name, func(t *testing.T) {
			body := getAs(t, d, modelsPath+query, cookie).Body.String()
			wants(t, body, "A-100")
			if strings.Contains(body, "A-200") {
				t.Error("the filter keeps a model of the other season")
			}
		})
	}

	t.Run("by state", func(t *testing.T) {
		rec := postAs(t, d, modelsPath+"/"+kept, modelForm(firstDrop, "A-100", false), cookie)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("turning the model off: status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
		}

		off := getAs(t, d, modelsPath+"?"+views.FieldModelActive+"=false", cookie).Body.String()
		wants(t, off, "A-100")
		if strings.Contains(off, "A-200") {
			t.Error("the state filter keeps a model that is still active")
		}

		on := getAs(t, d, modelsPath+"?"+views.FieldModelActive+"=true", cookie).Body.String()
		if strings.Contains(on, "A-100") {
			t.Error("the state filter keeps a model that is turned off")
		}
	})
}

// A filter that names nothing keeps every row, rather than answering an error.
func TestModelsListIgnoresAFilterThatNamesNothing(t *testing.T) {
	d := deps(t)
	dropID := spine(t, d)
	cookie := loggedIn(t, d, testEmail, testPasswd)
	createdModel(t, d, cookie, dropID, "A-100")

	query := "?" + views.FieldModelSeason + "=not-an-id&" + views.FieldModelActive + "=maybe"
	rec := getAs(t, d, modelsPath+query, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	wants(t, rec.Body.String(), "A-100")
}

// htmx asks for the same URL as the filter form, and gets the table alone.
func TestModelsListAnswersTheTableAloneToHTMX(t *testing.T) {
	d := deps(t)
	dropID := spine(t, d)
	cookie := loggedIn(t, d, testEmail, testPasswd)
	createdModel(t, d, cookie, dropID, "A-100")

	rec := getAsHTMX(t, d, modelsPath, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	wants(t, body, "A-100")
	if strings.Contains(body, "<title>") {
		t.Error("a fragment request got the whole page")
	}
}

// The create screen offers the drops of the chosen season, and htmx swaps that
// one control.
func TestModelCreateNarrowsTheDropsToTheChosenSeason(t *testing.T) {
	d := deps(t)
	root := loggedIn(t, d, rootEmail, rootPasswd)
	firstSeason := createdSeason(t, d, root, "S1", "2026-11-01")
	secondSeason := createdSeason(t, d, root, "S2", "2027-05-01")
	createdDrop(t, d, root, firstSeason, "Drop 1", "2027-02-15")
	createdDrop(t, d, root, secondSeason, "Drop 2", "2027-08-15")

	cookie := loggedIn(t, d, testEmail, testPasswd)
	wants(t, getAs(t, d, modelNewPath, cookie).Body.String(), "Drop 1", "Drop 2")

	rec := getAsHTMX(t, d, modelNewPath+"?"+views.FieldModelSeason+"="+firstSeason, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	wants(t, body, "Drop 1")
	if strings.Contains(body, "Drop 2") {
		t.Error("the drop control offers a drop of another season")
	}
	if strings.Contains(body, "<title>") {
		t.Error("a fragment request got the whole page")
	}
}

func TestModelCreateWritesItsAuditEntry(t *testing.T) {
	d, trail := auditedDeps(t)
	dropID := spine(t, d)
	cookie := loggedIn(t, d, testEmail, testPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")

	entries := entriesOf(t, trail, audit.EntityModel, id)
	if len(entries) != 1 {
		t.Fatalf("the trail holds %d entries, want 1: %+v", len(entries), entries)
	}
	if entries[0].Action != audit.ActionCreated {
		t.Errorf("action = %q, want %q", entries[0].Action, audit.ActionCreated)
	}
	if entries[0].New != "A-100" {
		t.Errorf("new value = %q, want %q", entries[0].New, "A-100")
	}
}

func TestModelCreateRejectsAnEmptyArticle(t *testing.T) {
	d, trail := auditedDeps(t)
	dropID := spine(t, d)
	cookie := loggedIn(t, d, testEmail, testPasswd)

	before := len(trail.actions())
	rec := postAs(t, d, modelsPath, modelForm(dropID, "", true), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	wants(t, rec.Body.String(), message(validate.Required))

	if got := len(trail.actions()); got != before {
		t.Errorf("a refused create left %d entries behind", got-before)
	}
}

func TestModelCreateRejectsADropThatIsNotThere(t *testing.T) {
	d := deps(t)
	spine(t, d)
	cookie := loggedIn(t, d, testEmail, testPasswd)

	for name, dropID := range map[string]string{
		"no such drop":   missingID.String(),
		"not an id":      "not-an-id",
		"nothing at all": "",
	} {
		t.Run(name, func(t *testing.T) {
			rec := postAs(t, d, modelsPath, modelForm(dropID, "A-100", true), cookie)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusUnprocessableEntity, rec.Body)
			}
			wants(t, rec.Body.String(), "A-100")
		})
	}
}

func TestModelCardShowsTheSpineAndReportsWhatItSaved(t *testing.T) {
	d := deps(t)
	dropID := spine(t, d)
	cookie := loggedIn(t, d, testEmail, testPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")
	path := modelsPath + "/" + id

	wants(t, getAs(t, d, path+"?saved=1", cookie).Body.String(), labels.Saved, "A-100", "S1", "Drop 1")
	if body := getAs(t, d, path, cookie).Body.String(); strings.Contains(body, labels.Saved) {
		t.Error("the card reports saved changes without a write before it")
	}
}

func TestModelEditWritesTheArticle(t *testing.T) {
	d, trail := auditedDeps(t)
	dropID := spine(t, d)
	cookie := loggedIn(t, d, testEmail, testPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")
	path := modelsPath + "/" + id

	rec := postAs(t, d, path, modelForm(dropID, "A-200", true), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}
	wants(t, getAs(t, d, path, cookie).Body.String(), "A-200")

	actions := actionsOf(entriesOf(t, trail, audit.EntityModel, id))
	if len(actions) != 2 || actions[0] != audit.ActionChanged {
		t.Errorf("the trail holds %v, want a change over the create", actions)
	}
}

func TestModelEditRejectsAnEmptyArticle(t *testing.T) {
	d := deps(t)
	dropID := spine(t, d)
	cookie := loggedIn(t, d, testEmail, testPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")

	rec := postAs(t, d, modelsPath+"/"+id, modelForm(dropID, "", true), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	wants(t, rec.Body.String(), message(validate.Required), "S1", "Drop 1")
}

func TestModelEditTurnsTheModelOffAndDeletesNothing(t *testing.T) {
	d, trail := auditedDeps(t)
	dropID := spine(t, d)
	cookie := loggedIn(t, d, testEmail, testPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")

	rec := postAs(t, d, modelsPath+"/"+id, modelForm(dropID, "A-100", false), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}
	wants(t, getAs(t, d, modelsPath, cookie).Body.String(), "A-100", labels.StateInactive)

	actions := actionsOf(entriesOf(t, trail, audit.EntityModel, id))
	if len(actions) != 2 || actions[0] != audit.ActionDeactivated {
		t.Errorf("the trail holds %v, want a deactivation over the create", actions)
	}
}

func TestModelPhotosUploadReorderAndCover(t *testing.T) {
	d, trail := auditedDeps(t)
	dropID := spine(t, d)
	cookie := loggedIn(t, d, testEmail, testPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")
	path := modelsPath + "/" + id

	// Three distinct images: the key is the content, so they must differ.
	rec := upload(t, d, path+"/photos", []string{pngUpload + "1", pngUpload + "2", pngUpload + "3"}, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("uploading: status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	strip := photoIDs(t, d, cookie, id)
	if len(strip) != 3 {
		t.Fatalf("the strip holds %d photos, want 3", len(strip))
	}
	wants(t, getAs(t, d, path, cookie).Body.String(), labels.ModelsPhotoCover)

	// The list renders the cover of the model, and nothing of the other two.
	cover := coverOf(t, d, cookie, id)
	if cover == "" {
		t.Fatal("the list shows no thumbnail for a model that holds three photos")
	}

	// Move the last photo one place earlier, twice, and it becomes the cover.
	moved := []string{strip[0], strip[2], strip[1]}
	if rec := postAs(t, d, path+"/photos/order", orderForm(moved), cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("reordering: status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}
	moved = []string{strip[2], strip[0], strip[1]}
	if rec := postAs(t, d, path+"/photos/order", orderForm(moved), cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("reordering: status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	if got := photoIDs(t, d, cookie, id); got[0] != strip[2] {
		t.Errorf("the strip starts with %q, want the moved photo %q", got[0], strip[2])
	}
	if got := coverOf(t, d, cookie, id); got == cover {
		t.Error("the thumbnail of the list did not follow the reorder")
	}

	actions := actionsOf(entriesOf(t, trail, audit.EntityModel, id))
	want := []string{
		audit.ActionPhotoReordered, audit.ActionPhotoReordered,
		audit.ActionPhotoAdded, audit.ActionPhotoAdded, audit.ActionPhotoAdded,
		audit.ActionCreated,
	}
	if strings.Join(actions, ",") != strings.Join(want, ",") {
		t.Errorf("the trail holds %v, want %v", actions, want)
	}
}

// coverOf is the address the list renders as the thumbnail of one model.
func coverOf(t *testing.T, d Deps, cookie *http.Cookie, modelID string) string {
	t.Helper()

	body := getAs(t, d, modelsPath+"/"+modelID, cookie).Body.String()
	_, after, found := strings.Cut(body, `<img src="/media/`)
	if !found {
		return ""
	}
	key, _, _ := strings.Cut(after, `"`)
	return key
}

func TestModelPhotoRemoveClosesTheGap(t *testing.T) {
	d, trail := auditedDeps(t)
	dropID := spine(t, d)
	cookie := loggedIn(t, d, testEmail, testPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")
	path := modelsPath + "/" + id

	upload(t, d, path+"/photos", []string{pngUpload + "1", pngUpload + "2"}, cookie)
	strip := photoIDs(t, d, cookie, id)
	if len(strip) != 2 {
		t.Fatalf("the strip holds %d photos, want 2", len(strip))
	}

	rec := postAs(t, d, path+"/photos/"+strip[0]+"/remove", nil, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	left := photoIDs(t, d, cookie, id)
	if len(left) != 1 || left[0] != strip[1] {
		t.Errorf("the strip is %v, want the second photo alone", left)
	}

	actions := actionsOf(entriesOf(t, trail, audit.EntityModel, id))
	if actions[0] != audit.ActionPhotoRemoved {
		t.Errorf("the trail holds %v, want a removal on top", actions)
	}
}

func TestModelPhotoRemoveIs404ForAPhotoThatIsNotThere(t *testing.T) {
	d := deps(t)
	dropID := spine(t, d)
	cookie := loggedIn(t, d, testEmail, testPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")

	for _, photoID := range []string{missingID.String(), "not-an-id"} {
		path := modelsPath + "/" + id + "/photos/" + photoID + "/remove"
		if rec := postAs(t, d, path, nil, cookie); rec.Code != http.StatusNotFound {
			t.Errorf("POST %s status = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
	}
}

func TestModelPhotoUploadRefusesWhatIsNotAnImage(t *testing.T) {
	d, trail := auditedDeps(t)
	dropID := spine(t, d)
	cookie := loggedIn(t, d, testEmail, testPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")
	before := len(trail.actions())

	rec := upload(t, d, modelsPath+"/"+id+"/photos", []string{textFile}, cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	wants(t, rec.Body.String(), labels.ModelsPhotoRejected)

	if got := len(trail.actions()); got != before {
		t.Errorf("a refused upload left %d entries behind", got-before)
	}
}

func TestModelPhotoUploadRefusesAFormWithNoFile(t *testing.T) {
	d := deps(t)
	dropID := spine(t, d)
	cookie := loggedIn(t, d, testEmail, testPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")

	rec := upload(t, d, modelsPath+"/"+id+"/photos", nil, cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	wants(t, rec.Body.String(), labels.ModelsPhotoNone)
}

// An order naming a strip the model no longer holds writes nothing, and the
// card says so.
func TestModelPhotoOrderRefusesAStaleStrip(t *testing.T) {
	d, trail := auditedDeps(t)
	dropID := spine(t, d)
	cookie := loggedIn(t, d, testEmail, testPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")
	path := modelsPath + "/" + id

	upload(t, d, path+"/photos", []string{pngUpload + "1", pngUpload + "2"}, cookie)
	strip := photoIDs(t, d, cookie, id)
	before := len(trail.actions())

	for name, order := range map[string][]string{
		"a photo of no model": {strip[0], missingID.String()},
		"half of the strip":   {strip[0]},
		"not an id":           {strip[0], "not-an-id"},
	} {
		t.Run(name, func(t *testing.T) {
			rec := postAs(t, d, path+"/photos/order", orderForm(order), cookie)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
			}
			wants(t, rec.Body.String(), labels.ModelsPhotoOrder)
		})
	}

	if got := photoIDs(t, d, cookie, id); len(got) != 2 || got[0] != strip[0] {
		t.Errorf("the strip is %v, want it unmoved at %v", got, strip)
	}
	if got := len(trail.actions()); got != before {
		t.Errorf("a refused order left %d entries behind", got-before)
	}
}

func TestModelScreensAnswer404ForAModelThatIsNotThere(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, testEmail, testPasswd)

	for _, id := range []string{missingID.String(), "not-an-id"} {
		path := modelsPath + "/" + id
		if rec := getAs(t, d, path, cookie); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
		rec := postAs(t, d, path, modelForm(ids.Nil.String(), "A-100", true), cookie)
		if rec.Code != http.StatusNotFound {
			t.Errorf("POST %s status = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
	}
}

// The card of one model carries what the trail recorded about that model.
func TestModelCardShowsTheAuditTrail(t *testing.T) {
	d := deps(t)
	dropID := spine(t, d)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")
	path := modelsPath + "/" + id

	if rec := upload(t, d, path+"/photos", []string{pngUpload + "1"}, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("upload status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}
	if rec := postAs(t, d, path, modelForm(dropID, "A-200", true), cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("edit status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	body := getAs(t, d, path, cookie).Body.String()
	article := labels.ModelsAuditArticle + ": A-100 → A-200"

	wants(t, body,
		labels.AuditTitle,
		labels.DateTime(testNow),
		labels.Name("Barbara", "Liskov"),
		labels.ModelsAuditCreated,
		labels.ModelsAuditPhotoAdded,
		article,
	)

	// Newest first: the create is the oldest of the three entries.
	if strings.Index(body, labels.ModelsAuditCreated) < strings.Index(body, article) {
		t.Error("the trail does not show the newest change first")
	}
	// The media key names a file, and the trail reads the action alone.
	if key := coverOf(t, d, cookie, id); key == "" || strings.Contains(body, ": "+key) {
		t.Errorf("the trail on the card reads out the media key %q", key)
	}
}
