package models_test

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
	pilamhttp "github.com/paveltessman/pilam/internal/http"
	"github.com/paveltessman/pilam/internal/http/models/views"
	"github.com/paveltessman/pilam/internal/http/paths"
	"github.com/paveltessman/pilam/internal/http/testkit"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

var modelRoutes = []testkit.Route{
	{Method: http.MethodGet, Path: paths.Models},
	{Method: http.MethodPost, Path: paths.Models},
	{Method: http.MethodGet, Path: paths.Models + "/new"},
	{Method: http.MethodGet, Path: paths.Models + "/" + testkit.MissingID.String()},
	{Method: http.MethodGet, Path: paths.Models + "/" + testkit.MissingID.String() + "/edit"},
	{Method: http.MethodPost, Path: paths.Models + "/" + testkit.MissingID.String()},
	{Method: http.MethodGet, Path: paths.Models + "/" + testkit.MissingID.String() + "/photos/edit"},
	{Method: http.MethodPost, Path: paths.Models + "/" + testkit.MissingID.String() + "/photos"},
	{Method: http.MethodPost, Path: paths.Models + "/" + testkit.MissingID.String() + "/photos/order"},
	{Method: http.MethodPost, Path: paths.Models + "/" + testkit.MissingID.String() + "/photos/" + testkit.MissingID.String() + "/remove"},
	{Method: http.MethodPost, Path: paths.Models + "/" + testkit.MissingID.String() + "/milestones"},
	{Method: http.MethodGet, Path: paths.Models + "/" + testkit.MissingID.String() + "/milestones/edit"},
	{Method: http.MethodPost, Path: paths.Models + "/" + testkit.MissingID.String() + "/milestones/template"},
	{Method: http.MethodPost, Path: paths.Models + "/" + testkit.MissingID.String() + "/milestones/" + testkit.MissingID.String()},
	{Method: http.MethodGet, Path: paths.Models + "/" + testkit.MissingID.String() + "/milestones/" + testkit.MissingID.String() + "/edit"},
	{Method: http.MethodGet, Path: paths.Models + "/" + testkit.MissingID.String() + "/milestones/" + testkit.MissingID.String() + "/plan"},
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
func createdModel(t *testing.T, d testkit.Deps, cookie *http.Cookie, dropID, article string) string {
	t.Helper()

	rec := testkit.PostAs(t, d, paths.Models, modelForm(dropID, article, true), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("creating model %q: status = %d, want %d: %s", article, rec.Code, http.StatusSeeOther, rec.Body)
	}
	return testkit.IDOfRedirect(t, rec, paths.Models)
}

// spine writes the season and the drop a model needs to exist. Only a root
// reaches those two sections, so it logs one in of its own.
func spine(t *testing.T, d testkit.Deps) string {
	t.Helper()

	root := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)
	seasonID := testkit.CreatedSeason(t, d, root, "S1", "2026-11-01")
	return testkit.CreatedDrop(t, d, root, seasonID, "Drop 1", "2027-02-15")
}

// upload posts files the way the photo control does: one multipart form, one
// part per file, all under the same field name.
func upload(t *testing.T, d testkit.Deps, path string, files []string, cookie *http.Cookie) *httptest.ResponseRecorder {
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
	pilamhttp.NewRouter(d).ServeHTTP(rec, r)
	return rec
}

// photoIDs is the strip of one model, in order, as the card renders it.
func photoIDs(t *testing.T, d testkit.Deps, cookie *http.Cookie, modelID string) []string {
	t.Helper()

	body := testkit.GetAs(t, d, photosDialog(modelID), cookie).Body.String()

	// Every photo carries a remove form named after it, and the forms stand in
	// the order the strip does. The identifier is what the action ends with.
	const suffix = `/remove"`

	var strip []string
	for rest := body; ; {
		before, remainder, found := strings.Cut(rest, suffix)
		if !found {
			return strip
		}
		id := before[len(before)-len(testkit.MissingID.String()):]
		if _, err := ids.Parse(id); err != nil {
			t.Fatalf("the card names no readable photo id before %q: %q", suffix, id)
		}
		strip = append(strip, id)
		rest = remainder
	}
}

// headerDialog and photosDialog are the two dialogs the card edits in.
func headerDialog(modelID string) string { return paths.Models + "/" + modelID + "/edit" }
func photosDialog(modelID string) string { return paths.Models + "/" + modelID + "/photos/edit" }

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
	d := testkit.NewDeps(t)

	for _, r := range modelRoutes {
		rec := testkit.Call(t, d, r, nil, nil)
		if rec.Code != http.StatusSeeOther {
			t.Errorf("%s %s status = %d, want %d", r.Method, r.Path, rec.Code, http.StatusSeeOther)
			continue
		}
		if got := rec.Header().Get("Location"); got != paths.Login {
			t.Errorf("%s %s Location = %q, want %q", r.Method, r.Path, got, paths.Login)
		}
	}
}

func TestModelSectionLetsAMemberIn(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	for _, r := range modelRoutes {
		rec := testkit.Call(t, d, r, modelForm(testkit.MissingID.String(), "A-100", true), cookie)
		if rec.Code == http.StatusForbidden {
			t.Errorf("%s %s answers %d to a member", r.Method, r.Path, rec.Code)
		}
	}
}

// The nav offers the list to a member, who reaches no other section.
func TestNavOffersTheModelListToEverybody(t *testing.T) {
	d := testkit.NewDeps(t)

	member := testkit.GetAs(t, d, paths.Models, testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)).Body.String()
	testkit.Wants(t, member, `href="`+paths.Models+`"`, labels.ModelsTitle)
}

// The root path holds no screen of its own.
func TestRootPathSendsTheBrowserToTheList(t *testing.T) {
	d := testkit.NewDeps(t)

	rec := testkit.GetAs(t, d, "/", testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != paths.Models {
		t.Errorf("Location = %q, want %q", got, paths.Models)
	}
}

func TestModelsListIsEmptyBeforeAnyModel(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	rec := testkit.GetAs(t, d, paths.Models, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	testkit.Wants(t, rec.Body.String(), labels.ModelsEmpty)
}

// Without a drop there is nothing to put a model in, so the list offers no
// create link and the form says what to do first.
func TestModelCreateWaitsForADrop(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	if body := testkit.GetAs(t, d, paths.Models, cookie).Body.String(); strings.Contains(body, labels.ActionAddModel) {
		t.Error("the list offers a create link with no drop to create into")
	}
	testkit.Wants(t, testkit.GetAs(t, d, paths.ModelNew, cookie).Body.String(), labels.ModelsNoDrop)
}

func TestModelsListShowsTheIdentityColumns(t *testing.T) {
	d := testkit.NewDeps(t)
	dropID := spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	createdModel(t, d, cookie, dropID, "A-100")

	body := testkit.GetAs(t, d, paths.Models, cookie).Body.String()
	testkit.Wants(t, body, "A-100", "S1", "Drop 1", labels.StateActive, labels.ActionAddModel)
}

func TestModelsListOrdersByArticle(t *testing.T) {
	d := testkit.NewDeps(t)
	dropID := spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	createdModel(t, d, cookie, dropID, "A-200")
	createdModel(t, d, cookie, dropID, "A-100")

	body := testkit.GetAs(t, d, paths.Models, cookie).Body.String()
	if strings.Index(body, "A-100") > strings.Index(body, "A-200") {
		t.Error("the list does not order by the article")
	}
}

func TestModelsListFiltersOnSeasonDropAndState(t *testing.T) {
	d := testkit.NewDeps(t)
	root := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)
	firstSeason := testkit.CreatedSeason(t, d, root, "S1", "2026-11-01")
	secondSeason := testkit.CreatedSeason(t, d, root, "S2", "2027-05-01")
	firstDrop := testkit.CreatedDrop(t, d, root, firstSeason, "Drop 1", "2027-02-15")
	secondDrop := testkit.CreatedDrop(t, d, root, secondSeason, "Drop 2", "2027-08-15")

	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)
	kept := createdModel(t, d, cookie, firstDrop, "A-100")
	createdModel(t, d, cookie, secondDrop, "A-200")

	// The season filter goes through the drop: the model holds no season.
	testData := map[string]string{
		"by season": "?" + views.FieldModelSeason + "=" + firstSeason,
		"by drop":   "?" + views.FieldModelDrop + "=" + firstDrop,
	}
	for name, query := range testData {
		t.Run(name, func(t *testing.T) {
			body := testkit.GetAs(t, d, paths.Models+query, cookie).Body.String()
			testkit.Wants(t, body, "A-100")
			if strings.Contains(body, "A-200") {
				t.Error("the filter keeps a model of the other season")
			}
		})
	}

	t.Run("by state", func(t *testing.T) {
		rec := testkit.PostAs(t, d, paths.Models+"/"+kept, modelForm(firstDrop, "A-100", false), cookie)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("turning the model off: status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
		}

		off := testkit.GetAs(t, d, paths.Models+"?"+views.FieldModelActive+"=false", cookie).Body.String()
		testkit.Wants(t, off, "A-100")
		if strings.Contains(off, "A-200") {
			t.Error("the state filter keeps a model that is still active")
		}

		on := testkit.GetAs(t, d, paths.Models+"?"+views.FieldModelActive+"=true", cookie).Body.String()
		if strings.Contains(on, "A-100") {
			t.Error("the state filter keeps a model that is turned off")
		}
	})
}

// A filter that names nothing keeps every row, rather than answering an error.
func TestModelsListIgnoresAFilterThatNamesNothing(t *testing.T) {
	d := testkit.NewDeps(t)
	dropID := spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)
	createdModel(t, d, cookie, dropID, "A-100")

	query := "?" + views.FieldModelSeason + "=not-an-id&" + views.FieldModelActive + "=maybe"
	rec := testkit.GetAs(t, d, paths.Models+query, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	testkit.Wants(t, rec.Body.String(), "A-100")
}

// htmx asks for the same URL as the filter form, and gets the table alone.
func TestModelsListAnswersTheTableAloneToHTMX(t *testing.T) {
	d := testkit.NewDeps(t)
	dropID := spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)
	createdModel(t, d, cookie, dropID, "A-100")

	rec := testkit.GetAsHTMX(t, d, paths.Models, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	testkit.Wants(t, body, "A-100")
	if strings.Contains(body, "<title>") {
		t.Error("a fragment request got the whole page")
	}
}

// The create screen offers the drops of the chosen season, and htmx swaps that
// one control.
func TestModelCreateNarrowsTheDropsToTheChosenSeason(t *testing.T) {
	d := testkit.NewDeps(t)
	root := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)
	firstSeason := testkit.CreatedSeason(t, d, root, "S1", "2026-11-01")
	secondSeason := testkit.CreatedSeason(t, d, root, "S2", "2027-05-01")
	testkit.CreatedDrop(t, d, root, firstSeason, "Drop 1", "2027-02-15")
	testkit.CreatedDrop(t, d, root, secondSeason, "Drop 2", "2027-08-15")

	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)
	testkit.Wants(t, testkit.GetAs(t, d, paths.ModelNew, cookie).Body.String(), "Drop 1", "Drop 2")

	rec := testkit.GetAsHTMX(t, d, paths.ModelNew+"?"+views.FieldModelSeason+"="+firstSeason, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	testkit.Wants(t, body, "Drop 1")
	if strings.Contains(body, "Drop 2") {
		t.Error("the drop control offers a drop of another season")
	}
	if strings.Contains(body, "<title>") {
		t.Error("a fragment request got the whole page")
	}
}

func TestModelCreateWritesItsAuditEntry(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	dropID := spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")

	entries := testkit.EntriesOf(t, trail, audit.EntityModel, id)
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
	d, trail := testkit.AuditedDeps(t)
	dropID := spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	before := len(trail.Actions())
	rec := testkit.PostAs(t, d, paths.Models, modelForm(dropID, "", true), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	testkit.Wants(t, rec.Body.String(), testkit.Message(validate.Required))

	if got := len(trail.Actions()); got != before {
		t.Errorf("a refused create left %d entries behind", got-before)
	}
}

func TestModelCreateRejectsADropThatIsNotThere(t *testing.T) {
	d := testkit.NewDeps(t)
	spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	for name, dropID := range map[string]string{
		"no such drop":   testkit.MissingID.String(),
		"not an id":      "not-an-id",
		"nothing at all": "",
	} {
		t.Run(name, func(t *testing.T) {
			rec := testkit.PostAs(t, d, paths.Models, modelForm(dropID, "A-100", true), cookie)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusUnprocessableEntity, rec.Body)
			}
			testkit.Wants(t, rec.Body.String(), "A-100")
		})
	}
}

func TestModelCardShowsTheSpineAndReportsWhatItSaved(t *testing.T) {
	d := testkit.NewDeps(t)
	dropID := spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")
	path := paths.Models + "/" + id

	testkit.Wants(t, testkit.GetAs(t, d, path+"?saved=1", cookie).Body.String(), labels.Saved, "A-100", "S1", "Drop 1")
	if body := testkit.GetAs(t, d, path, cookie).Body.String(); strings.Contains(body, labels.Saved) {
		t.Error("the card reports saved changes without a write before it")
	}
}

func TestModelEditWritesTheArticle(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	dropID := spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")
	path := paths.Models + "/" + id

	rec := testkit.PostAs(t, d, path, modelForm(dropID, "A-200", true), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}
	testkit.Wants(t, testkit.GetAs(t, d, path, cookie).Body.String(), "A-200")

	actions := testkit.ActionsOf(testkit.EntriesOf(t, trail, audit.EntityModel, id))
	if len(actions) != 2 || actions[0] != audit.ActionChanged {
		t.Errorf("the trail holds %v, want a change over the create", actions)
	}
}

func TestModelEditRejectsAnEmptyArticle(t *testing.T) {
	d := testkit.NewDeps(t)
	dropID := spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")

	rec := testkit.PostAs(t, d, paths.Models+"/"+id, modelForm(dropID, "", true), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	testkit.Wants(t, rec.Body.String(), testkit.Message(validate.Required), "S1", "Drop 1")
}

func TestModelEditTurnsTheModelOffAndDeletesNothing(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	dropID := spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")

	rec := testkit.PostAs(t, d, paths.Models+"/"+id, modelForm(dropID, "A-100", false), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}
	testkit.Wants(t, testkit.GetAs(t, d, paths.Models, cookie).Body.String(), "A-100", labels.StateInactive)

	actions := testkit.ActionsOf(testkit.EntriesOf(t, trail, audit.EntityModel, id))
	if len(actions) != 2 || actions[0] != audit.ActionDeactivated {
		t.Errorf("the trail holds %v, want a deactivation over the create", actions)
	}
}

func TestModelPhotosUploadReorderAndCover(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	dropID := spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")
	path := paths.Models + "/" + id

	// Three distinct images: the key is the content, so they must differ.
	rec := upload(t, d, path+"/photos", []string{pngUpload + "1", pngUpload + "2", pngUpload + "3"}, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("uploading: status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	strip := photoIDs(t, d, cookie, id)
	if len(strip) != 3 {
		t.Fatalf("the strip holds %d photos, want 3", len(strip))
	}
	testkit.Wants(t, testkit.GetAs(t, d, path, cookie).Body.String(), labels.ModelsPhotoCover)

	// The list renders the cover of the model, and nothing of the other two.
	cover := coverOf(t, d, cookie, id)
	if cover == "" {
		t.Fatal("the list shows no thumbnail for a model that holds three photos")
	}

	// Move the last photo one place earlier, twice, and it becomes the cover.
	moved := []string{strip[0], strip[2], strip[1]}
	if rec := testkit.PostAs(t, d, path+"/photos/order", orderForm(moved), cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("reordering: status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}
	moved = []string{strip[2], strip[0], strip[1]}
	if rec := testkit.PostAs(t, d, path+"/photos/order", orderForm(moved), cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("reordering: status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	if got := photoIDs(t, d, cookie, id); got[0] != strip[2] {
		t.Errorf("the strip starts with %q, want the moved photo %q", got[0], strip[2])
	}
	if got := coverOf(t, d, cookie, id); got == cover {
		t.Error("the thumbnail of the list did not follow the reorder")
	}

	actions := testkit.ActionsOf(testkit.EntriesOf(t, trail, audit.EntityModel, id))
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
func coverOf(t *testing.T, d testkit.Deps, cookie *http.Cookie, modelID string) string {
	t.Helper()

	body := testkit.GetAs(t, d, paths.Models+"/"+modelID, cookie).Body.String()
	_, after, found := strings.Cut(body, `<img src="/media/`)
	if !found {
		return ""
	}
	key, _, _ := strings.Cut(after, `"`)
	return key
}

func TestModelPhotoRemoveClosesTheGap(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	dropID := spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")
	path := paths.Models + "/" + id

	upload(t, d, path+"/photos", []string{pngUpload + "1", pngUpload + "2"}, cookie)
	strip := photoIDs(t, d, cookie, id)
	if len(strip) != 2 {
		t.Fatalf("the strip holds %d photos, want 2", len(strip))
	}

	rec := testkit.PostAs(t, d, path+"/photos/"+strip[0]+"/remove", nil, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	left := photoIDs(t, d, cookie, id)
	if len(left) != 1 || left[0] != strip[1] {
		t.Errorf("the strip is %v, want the second photo alone", left)
	}

	actions := testkit.ActionsOf(testkit.EntriesOf(t, trail, audit.EntityModel, id))
	if actions[0] != audit.ActionPhotoRemoved {
		t.Errorf("the trail holds %v, want a removal on top", actions)
	}
}

func TestModelPhotoRemoveIs404ForAPhotoThatIsNotThere(t *testing.T) {
	d := testkit.NewDeps(t)
	dropID := spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")

	for _, photoID := range []string{testkit.MissingID.String(), "not-an-id"} {
		path := paths.Models + "/" + id + "/photos/" + photoID + "/remove"
		if rec := testkit.PostAs(t, d, path, nil, cookie); rec.Code != http.StatusNotFound {
			t.Errorf("POST %s status = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
	}
}

func TestModelPhotoUploadRefusesWhatIsNotAnImage(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	dropID := spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")
	before := len(trail.Actions())

	rec := upload(t, d, paths.Models+"/"+id+"/photos", []string{textFile}, cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	testkit.Wants(t, rec.Body.String(), labels.ModelsPhotoRejected)

	if got := len(trail.Actions()); got != before {
		t.Errorf("a refused upload left %d entries behind", got-before)
	}
}

func TestModelPhotoUploadRefusesAFormWithNoFile(t *testing.T) {
	d := testkit.NewDeps(t)
	dropID := spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")

	rec := upload(t, d, paths.Models+"/"+id+"/photos", nil, cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	testkit.Wants(t, rec.Body.String(), labels.ModelsPhotoNone)
}

// An order naming a strip the model no longer holds writes nothing, and the
// card says so.
func TestModelPhotoOrderRefusesAStaleStrip(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	dropID := spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")
	path := paths.Models + "/" + id

	upload(t, d, path+"/photos", []string{pngUpload + "1", pngUpload + "2"}, cookie)
	strip := photoIDs(t, d, cookie, id)
	before := len(trail.Actions())

	for name, order := range map[string][]string{
		"a photo of no model": {strip[0], testkit.MissingID.String()},
		"half of the strip":   {strip[0]},
		"not an id":           {strip[0], "not-an-id"},
	} {
		t.Run(name, func(t *testing.T) {
			rec := testkit.PostAs(t, d, path+"/photos/order", orderForm(order), cookie)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
			}
			testkit.Wants(t, rec.Body.String(), labels.ModelsPhotoOrder)
		})
	}

	if got := photoIDs(t, d, cookie, id); len(got) != 2 || got[0] != strip[0] {
		t.Errorf("the strip is %v, want it unmoved at %v", got, strip)
	}
	if got := len(trail.Actions()); got != before {
		t.Errorf("a refused order left %d entries behind", got-before)
	}
}

func TestModelScreensAnswer404ForAModelThatIsNotThere(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	for _, id := range []string{testkit.MissingID.String(), "not-an-id"} {
		path := paths.Models + "/" + id
		for _, read := range []string{path, headerDialog(id), photosDialog(id)} {
			if rec := testkit.GetAs(t, d, read, cookie); rec.Code != http.StatusNotFound {
				t.Errorf("GET %s status = %d, want %d", read, rec.Code, http.StatusNotFound)
			}
		}
		rec := testkit.PostAs(t, d, path, modelForm(ids.Nil.String(), "A-100", true), cookie)
		if rec.Code != http.StatusNotFound {
			t.Errorf("POST %s status = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
	}
}

// The card of one model carries what the trail recorded about that model.
func TestModelCardShowsTheAuditTrail(t *testing.T) {
	d := testkit.NewDeps(t)
	dropID := spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")
	path := paths.Models + "/" + id

	if rec := upload(t, d, path+"/photos", []string{pngUpload + "1"}, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("upload status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}
	if rec := testkit.PostAs(t, d, path, modelForm(dropID, "A-200", true), cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("edit status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	body := testkit.GetAs(t, d, path, cookie).Body.String()
	article := labels.ModelsAuditArticle + ": A-100 → A-200"

	testkit.Wants(t, body,
		labels.AuditTitle,
		labels.DateTime(testkit.Now),
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

// The card reads. It names where the model sits in the catalog and what state
// it is in, and it carries no box the user types into.
func TestModelCardReadsTheHeaderAndOffersTheDialog(t *testing.T) {
	d := testkit.NewDeps(t)
	dropID := spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")
	body := testkit.GetAs(t, d, paths.Models+"/"+id, cookie).Body.String()

	testkit.Wants(t, body,
		labels.ModelsCardTitle,
		"A-100",
		labels.StateActive,
		"S1",
		"Drop 1"+labels.Separator+"15.02.2027",
		labels.ActionEdit,
		labels.ActionManagePhotos,
		`hx-get="`+headerDialog(id)+`"`,
		`hx-get="`+photosDialog(id)+`"`,
	)
	if strings.Contains(body, `name="`+views.FieldModelArticle+`"`) {
		t.Error("the card carries the article box that belongs in the dialog")
	}
}

// The dialog that edits the header holds the article and the state, and it
// reads the season and the drop back without offering to move either.
func TestModelHeaderDialogEditsTheArticleAndTheState(t *testing.T) {
	d := testkit.NewDeps(t)
	dropID := spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")
	body := testkit.GetAs(t, d, headerDialog(id), cookie).Body.String()

	testkit.Wants(t, body,
		"<dialog",
		labels.ModelsHeaderHint,
		`name="`+views.FieldModelArticle+`"`,
		`value="A-100"`,
		`name="`+views.FieldModelActive+`"`,
		`action="`+paths.Models+"/"+id+`"`,
		labels.ActionSave,
	)
	for _, field := range []string{views.FieldModelSeason, views.FieldModelDrop} {
		if strings.Contains(body, `name="`+field+`"`) {
			t.Errorf("the dialog posts %q, which a model does not move", field)
		}
	}
}

// A refused article comes back with the dialog open on what the user typed, so
// that the message stands beside the box it belongs to.
func TestModelHeaderDialogComesBackOpenOnARefusedArticle(t *testing.T) {
	d := testkit.NewDeps(t)
	dropID := spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")
	rec := testkit.PostAs(t, d, paths.Models+"/"+id, modelForm(dropID, "", true), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}

	testkit.Wants(t, rec.Body.String(),
		"<dialog",
		labels.ModelsHeaderHint,
		testkit.Message(validate.Required),
	)
}

// The dialog that manages the strip offers the upload, and one row per photo
// with the buttons that move it and the button that removes it.
func TestModelPhotosDialogManagesTheStrip(t *testing.T) {
	d := testkit.NewDeps(t)
	dropID := spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")
	path := paths.Models + "/" + id
	if rec := upload(t, d, path+"/photos", []string{pngUpload + "1", pngUpload + "2"}, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("upload status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	body := testkit.GetAs(t, d, photosDialog(id), cookie).Body.String()

	testkit.Wants(t, body,
		"<dialog",
		labels.ModelsPhotosCoverHint,
		`name="`+views.FieldModelPhoto+`"`,
		labels.ActionAddPhoto,
		labels.ActionMoveDown,
		labels.ActionPhotoRemove,
		`action="`+path+`/photos/order"`,
	)
	// The first photo has nowhere to go up, and the last has nowhere to go
	// down, so two photos carry one move button each.
	if got := strings.Count(body, `aria-label="`+labels.ActionMoveUp+`"`); got != 1 {
		t.Errorf("buttons that move a photo up = %d, want 1", got)
	}
}

// A refused upload comes back with the photo dialog open, so that the message
// reaches the user rather than the page behind the dialog.
func TestModelPhotosDialogComesBackOpenOnARefusedUpload(t *testing.T) {
	d := testkit.NewDeps(t)
	dropID := spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	id := createdModel(t, d, cookie, dropID, "A-100")
	rec := upload(t, d, paths.Models+"/"+id+"/photos", []string{textFile}, cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}

	testkit.Wants(t, rec.Body.String(), "<dialog", labels.ModelsPhotoRejected)
}
