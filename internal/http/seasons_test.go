package http

import (
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

// An id no row holds, for the screens that answer 404.
var missingID = ids.MustParse("01912345-6789-7abc-def0-0000000000ff")

var seasonRoutes = []route{
	{http.MethodGet, seasonsPath},
	{http.MethodPost, seasonsPath},
	{http.MethodGet, seasonsPath + "/new"},
	{http.MethodGet, seasonsPath + "/" + missingID.String()},
	{http.MethodPost, seasonsPath + "/" + missingID.String()},
}

// seasonForm is what the create and the edit screen post.
func seasonForm(name, start string, active bool) url.Values {
	form := url.Values{
		views.FieldSeasonName:  {name},
		views.FieldSeasonStart: {start},
	}
	if active {
		form.Set(views.FieldSeasonActive, "true")
	}
	return form
}

// createdSeason posts the create form and returns the id of the season it wrote.
func createdSeason(t *testing.T, d Deps, cookie *http.Cookie, name, start string) string {
	t.Helper()

	rec := postAs(t, d, seasonsPath, seasonForm(name, start, true), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("creating season %q: status = %d, want %d: %s", name, rec.Code, http.StatusSeeOther, rec.Body)
	}
	return idOfRedirect(t, rec, seasonsPath)
}

// idOfRedirect reads the id out of the location a write redirects to.
func idOfRedirect(t *testing.T, rec *httptest.ResponseRecorder, section string) string {
	t.Helper()

	location := rec.Header().Get("Location")
	id, _, _ := strings.Cut(strings.TrimPrefix(location, section+"/"), "?")
	if _, err := ids.Parse(id); err != nil {
		t.Fatalf("the redirect to %q names no id of %s", location, section)
	}
	return id
}

// entriesOf is what the trail holds about one entity, newest first.
func entriesOf(t *testing.T, trail *recorded, entity, id string) []audit.Entry {
	t.Helper()

	entityID, err := ids.Parse(id)
	if err != nil {
		t.Fatalf("reading the trail of %s %q: %v", entity, id, err)
	}
	entries, err := trail.ByEntity(t.Context(), entity, entityID, audit.DefaultLimit)
	if err != nil {
		t.Fatalf("reading the trail of %s %s: %v", entity, id, err)
	}
	return entries
}

// actionsOf is the actions of a trail, newest first.
func actionsOf(entries []audit.Entry) []string {
	actions := make([]string, len(entries))
	for i, entry := range entries {
		actions[i] = entry.Action
	}
	return actions
}

// message is the sentence a rejection reads as on the screen.
func message(code validate.Code) string {
	return labels.Message(validate.FieldError{Code: code})
}

// wants fails unless the body holds every string given.
func wants(t *testing.T, body string, texts ...string) {
	t.Helper()

	for _, text := range texts {
		if !strings.Contains(body, text) {
			t.Errorf("the screen does not hold %q", text)
		}
	}
}

func TestSeasonScreenNamesFieldsThatServiceRejects(t *testing.T) {
	pairs := [][2]string{
		{views.FieldSeasonName, catalog.FieldName},
		{views.FieldSeasonStart, catalog.FieldStartDate},
		{views.FieldSeasonActive, catalog.FieldActive},
	}
	for _, pair := range pairs {
		if pair[0] != pair[1] {
			t.Errorf("the view posts %q, the service rejects %q", pair[0], pair[1])
		}
	}
}

func TestSeasonsSectionRefusesMember(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, testEmail, testPasswd)

	for _, r := range seasonRoutes {
		rec := call(t, d, r, seasonForm("S1", "2026-11-01", true), cookie)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s status = %d, want %d", r.method, r.path, rec.Code, http.StatusForbidden)
		}
	}
}

func TestSeasonsSectionNeedsLogin(t *testing.T) {
	d := deps(t)

	for _, r := range seasonRoutes {
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

func TestSeasonsListIsEmptyBeforeAnySeason(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	rec := getAs(t, d, seasonsPath, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	wants(t, rec.Body.String(), labels.SeasonsEmpty)
}

func TestSeasonsListOrdersByStartDate(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	createdSeason(t, d, cookie, "S2", "2027-05-01")
	createdSeason(t, d, cookie, "S1", "2026-11-01")

	body := getAs(t, d, seasonsPath, cookie).Body.String()
	wants(t, body, "S1", "S2", "01.11.2026", "01.05.2027")

	if strings.Index(body, "S1") > strings.Index(body, "S2") {
		t.Error("the list orders by the name, want the start date")
	}
}

func TestSeasonCreateWritesItsAuditEntry(t *testing.T) {
	d, trail := auditedDeps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	id := createdSeason(t, d, cookie, "S1", "2026-11-01")

	entries := entriesOf(t, trail, audit.EntitySeason, id)
	if len(entries) != 1 {
		t.Fatalf("the trail holds %d entries, want 1: %+v", len(entries), entries)
	}
	if entries[0].Action != audit.ActionCreated {
		t.Errorf("action = %q, want %q", entries[0].Action, audit.ActionCreated)
	}
	if entries[0].New != "S1" {
		t.Errorf("new value = %q, want %q", entries[0].New, "S1")
	}
}

func TestSeasonCreateRejectsAnEmptyForm(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	rec := postAs(t, d, seasonsPath, seasonForm("", "", true), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	wants(t, rec.Body.String(), message(validate.Required))
}

func TestSeasonCreateRejectsTextThatIsNotADate(t *testing.T) {
	d, trail := auditedDeps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	rec := postAs(t, d, seasonsPath, seasonForm("S1", "01.11.2026", true), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}

	body := rec.Body.String()
	wants(t, body, message(validate.NotADate), "S1")
	if strings.Contains(body, message(validate.Required)) {
		t.Error("the screen reports a missing value, want a date it could not read")
	}
	if len(trail.actions()) != 0 {
		t.Errorf("a refused create left %v behind", trail.actions())
	}
}

func TestSeasonCreateRejectsANameAlreadyHeld(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	createdSeason(t, d, cookie, "S1", "2026-11-01")

	rec := postAs(t, d, seasonsPath, seasonForm("s1", "2027-05-01", true), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	wants(t, rec.Body.String(), message(validate.Taken))
}

func TestSeasonCardReportsWhatItSaved(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	id := createdSeason(t, d, cookie, "S1", "2026-11-01")
	path := seasonsPath + "/" + id

	wants(t, getAs(t, d, path+"?saved=1", cookie).Body.String(), labels.Saved)
	if body := getAs(t, d, path, cookie).Body.String(); strings.Contains(body, labels.Saved) {
		t.Error("the card reports saved changes without a write before it")
	}
}

func TestSeasonEditWritesTheNameAndTheDate(t *testing.T) {
	d, trail := auditedDeps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	id := createdSeason(t, d, cookie, "S1", "2026-11-01")
	path := seasonsPath + "/" + id

	rec := postAs(t, d, path, seasonForm("SS28", "2026-12-01", true), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	wants(t, getAs(t, d, path, cookie).Body.String(), "SS28", "2026-12-01")

	// The create, then one entry per field that moved.
	if got, want := len(entriesOf(t, trail, audit.EntitySeason, id)), 3; got != want {
		t.Errorf("the trail holds %d entries, want %d", got, want)
	}
}

func TestSeasonEditTurnsTheSeasonOff(t *testing.T) {
	d, trail := auditedDeps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	id := createdSeason(t, d, cookie, "S1", "2026-11-01")

	rec := postAs(t, d, seasonsPath+"/"+id, seasonForm("S1", "2026-11-01", false), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	wants(t, getAs(t, d, seasonsPath, cookie).Body.String(), "S1", labels.StateInactive)

	actions := actionsOf(entriesOf(t, trail, audit.EntitySeason, id))
	if len(actions) != 2 || actions[0] != audit.ActionDeactivated {
		t.Errorf("the trail holds %v, want a deactivation over the create", actions)
	}
}

// An edit that moves nothing writes nothing, and the card still reports itself
// as saved.
func TestSeasonEditThatMovesNothingWritesNothing(t *testing.T) {
	d, trail := auditedDeps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	id := createdSeason(t, d, cookie, "S1", "2026-11-01")

	rec := postAs(t, d, seasonsPath+"/"+id, seasonForm("S1", "2026-11-01", true), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got, want := len(entriesOf(t, trail, audit.EntitySeason, id)), 1; got != want {
		t.Errorf("the trail holds %d entries, want %d", got, want)
	}
}

func TestSeasonScreensAnswer404ForASeasonThatIsNotThere(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	for _, path := range []string{
		seasonsPath + "/" + missingID.String(),
		seasonsPath + "/not-an-id",
	} {
		if rec := getAs(t, d, path, cookie); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
		rec := postAs(t, d, path, seasonForm("S1", "2026-11-01", true), cookie)
		if rec.Code != http.StatusNotFound {
			t.Errorf("POST %s status = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
	}
}

// The nav offers a root the sections a root reaches, and offers a member none
// of them.
func TestNavOffersTheCatalogSectionsToARootAlone(t *testing.T) {
	d := deps(t)

	root := getAs(t, d, successPath, loggedIn(t, d, rootEmail, rootPasswd)).Body.String()
	wants(t, root, `href="`+seasonsPath+`"`, `href="`+dropsPath+`"`)

	member := getAs(t, d, successPath, loggedIn(t, d, testEmail, testPasswd)).Body.String()
	for _, link := range []string{`href="` + seasonsPath + `"`, `href="` + dropsPath + `"`} {
		if strings.Contains(member, link) {
			t.Errorf("a member is offered %s", link)
		}
	}
}
