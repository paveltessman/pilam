package http

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/http/views"
	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

var dropRoutes = []route{
	{http.MethodGet, dropsPath},
	{http.MethodPost, dropsPath},
	{http.MethodGet, dropsPath + "/new"},
	{http.MethodGet, dropsPath + "/" + missingID.String()},
	{http.MethodPost, dropsPath + "/" + missingID.String()},
}

// dropForm is what the create and the edit screen post.
func dropForm(seasonID, name, target string, active bool) url.Values {
	form := url.Values{
		views.FieldDropSeason: {seasonID},
		views.FieldDropName:   {name},
		views.FieldDropTarget: {target},
	}
	if active {
		form.Set(views.FieldDropActive, "true")
	}
	return form
}

// createdDrop posts the create form and returns the id of the drop it wrote.
func createdDrop(t *testing.T, d Deps, cookie *http.Cookie, seasonID, name, target string) string {
	t.Helper()

	rec := postAs(t, d, dropsPath, dropForm(seasonID, name, target, true), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("creating drop %q: status = %d, want %d: %s", name, rec.Code, http.StatusSeeOther, rec.Body)
	}
	return idOfRedirect(t, rec, dropsPath)
}

func TestDropScreenNamesFieldsThatServiceRejects(t *testing.T) {
	pairs := [][2]string{
		{views.FieldDropSeason, catalog.FieldSeason},
		{views.FieldDropName, catalog.FieldName},
		{views.FieldDropTarget, catalog.FieldTargetDate},
		{views.FieldDropActive, catalog.FieldActive},
	}
	for _, pair := range pairs {
		if pair[0] != pair[1] {
			t.Errorf("the view posts %q, the service rejects %q", pair[0], pair[1])
		}
	}
}

func TestDropsSectionRefusesMember(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, testEmail, testPasswd)

	for _, r := range dropRoutes {
		rec := call(t, d, r, dropForm(missingID.String(), "Дроп 1", "2027-02-15", true), cookie)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s status = %d, want %d", r.method, r.path, rec.Code, http.StatusForbidden)
		}
	}
}

func TestDropsSectionNeedsLogin(t *testing.T) {
	d := deps(t)

	for _, r := range dropRoutes {
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

func TestDropsSectionAsksForASeasonFirst(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	list := getAs(t, d, dropsPath, cookie).Body.String()
	wants(t, list, labels.DropsNoSeason)
	if strings.Contains(list, `href="`+dropsPath+`/new"`) {
		t.Error("the list offers a create link with no season to put the drop in")
	}

	form := getAs(t, d, dropsPath+"/new", cookie)
	if form.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", form.Code, http.StatusOK)
	}
	wants(t, form.Body.String(), labels.DropsNoSeason)
}

func TestDropAppearsUnderItsSeason(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	season := createdSeason(t, d, cookie, "S1", "2026-11-01")
	createdDrop(t, d, cookie, season, "Дроп 1", "2027-02-15")

	body := getAs(t, d, dropsPath, cookie).Body.String()
	wants(t, body, "S1", "Дроп 1", "15.02.2027")

	if strings.Index(body, "S1") > strings.Index(body, "Дроп 1") {
		t.Error("the drop stands above the season that holds it")
	}
}

// The drops of one season order by the target date.
func TestDropsOrderByTargetDate(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	season := createdSeason(t, d, cookie, "S1", "2026-11-01")
	createdDrop(t, d, cookie, season, "Поздний", "2027-04-01")
	createdDrop(t, d, cookie, season, "Ранний", "2027-02-15")

	body := getAs(t, d, dropsPath, cookie).Body.String()
	if strings.Index(body, "Ранний") > strings.Index(body, "Поздний") {
		t.Error("the drops order by the name, want the target date")
	}
}

// The create form offers the active seasons alone: a drop goes into a season
// that is still in work.
func TestDropFormOffersTheActiveSeasons(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	open := createdSeason(t, d, cookie, "S1", "2026-11-01")
	closed := createdSeason(t, d, cookie, "AW26", "2026-05-01")

	rec := postAs(t, d, seasonsPath+"/"+closed, seasonForm("AW26", "2026-05-01", false), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("turning a season off: status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	body := getAs(t, d, dropsPath+"/new", cookie).Body.String()
	wants(t, body, `value="`+open+`"`)
	if strings.Contains(body, `value="`+closed+`"`) {
		t.Error("the form offers a season that is turned off")
	}
}

func TestDropCreateWritesItsAuditEntry(t *testing.T) {
	d, trail := auditedDeps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	season := createdSeason(t, d, cookie, "S1", "2026-11-01")
	id := createdDrop(t, d, cookie, season, "Дроп 1", "2027-02-15")

	entries := entriesOf(t, trail, audit.EntityDrop, id)
	if len(entries) != 1 {
		t.Fatalf("the trail holds %d entries, want 1: %+v", len(entries), entries)
	}
	if entries[0].Action != audit.ActionCreated {
		t.Errorf("action = %q, want %q", entries[0].Action, audit.ActionCreated)
	}
}

func TestDropCreateRejectsAnEmptyForm(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)
	createdSeason(t, d, cookie, "S1", "2026-11-01")

	rec := postAs(t, d, dropsPath, dropForm("", "", "", true), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	wants(t, rec.Body.String(), message(validate.Required))
}

// A season the select never offered is refused as a value outside the list,
// whether it is unreadable text or an id no row holds.
func TestDropCreateRejectsASeasonTheFormNeverOffered(t *testing.T) {
	d, trail := auditedDeps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)
	createdSeason(t, d, cookie, "S1", "2026-11-01")

	for _, season := range []string{"not-an-id", missingID.String()} {
		rec := postAs(t, d, dropsPath, dropForm(season, "Дроп 1", "2027-02-15", true), cookie)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("season %q: status = %d, want %d", season, rec.Code, http.StatusUnprocessableEntity)
		}
		wants(t, rec.Body.String(), message(validate.NotAllowed))
	}

	// The season create is the one write the trail holds.
	if actions := trail.actions(); len(actions) != 1 {
		t.Errorf("the trail holds %v, want the season create alone", actions)
	}
}

func TestDropCreateRejectsTextThatIsNotADate(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)
	season := createdSeason(t, d, cookie, "S1", "2026-11-01")

	rec := postAs(t, d, dropsPath, dropForm(season, "Дроп 1", "15.02.2027", true), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}

	body := rec.Body.String()
	wants(t, body, message(validate.NotADate), "Дроп 1")
	if strings.Contains(body, message(validate.Required)) {
		t.Error("the screen reports a missing value, want a date it could not read")
	}
}

// The name is unique inside a season, so the same name in another season is
// not a duplicate.
func TestDropNameIsUniqueInsideItsSeasonAlone(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	first := createdSeason(t, d, cookie, "S1", "2026-11-01")
	second := createdSeason(t, d, cookie, "S2", "2027-05-01")
	createdDrop(t, d, cookie, first, "Дроп 1", "2027-02-15")

	rec := postAs(t, d, dropsPath, dropForm(first, "дроп 1", "2027-03-15", true), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	wants(t, rec.Body.String(), message(validate.Taken))

	createdDrop(t, d, cookie, second, "Дроп 1", "2027-08-15")
}

func TestDropEditWritesTheNameAndTheDate(t *testing.T) {
	d, trail := auditedDeps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	season := createdSeason(t, d, cookie, "S1", "2026-11-01")
	id := createdDrop(t, d, cookie, season, "Дроп 1", "2027-02-15")
	path := dropsPath + "/" + id

	rec := postAs(t, d, path, dropForm(season, "Дроп 2", "2027-03-15", true), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	wants(t, getAs(t, d, path, cookie).Body.String(), "Дроп 2", "2027-03-15")

	// The create, then one entry per field that moved.
	if got, want := len(entriesOf(t, trail, audit.EntityDrop, id)), 3; got != want {
		t.Errorf("the trail holds %d entries, want %d", got, want)
	}
}

func TestDropEditTurnsTheDropOff(t *testing.T) {
	d, trail := auditedDeps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	season := createdSeason(t, d, cookie, "S1", "2026-11-01")
	id := createdDrop(t, d, cookie, season, "Дроп 1", "2027-02-15")

	rec := postAs(t, d, dropsPath+"/"+id, dropForm(season, "Дроп 1", "2027-02-15", false), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	wants(t, getAs(t, d, dropsPath, cookie).Body.String(), "Дроп 1", labels.StateInactive)

	actions := actionsOf(entriesOf(t, trail, audit.EntityDrop, id))
	if len(actions) != 2 || actions[0] != audit.ActionDeactivated {
		t.Errorf("the trail holds %v, want a deactivation over the create", actions)
	}
}

// Moving a drop to another season carries every model of it across, which is a
// season transition. The card offers no such control, and a posted season is
// not read.
func TestDropEditDoesNotMoveTheDropToAnotherSeason(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	first := createdSeason(t, d, cookie, "S1", "2026-11-01")
	second := createdSeason(t, d, cookie, "S2", "2027-05-01")
	id := createdDrop(t, d, cookie, first, "Дроп 1", "2027-02-15")
	path := dropsPath + "/" + id

	card := getAs(t, d, path, cookie).Body.String()
	wants(t, card, "S1", labels.DropsSeasonFixed)
	if strings.Contains(card, `value="`+second+`"`) {
		t.Error("the card offers a change of season")
	}

	rec := postAs(t, d, path, dropForm(second, "Дроп 1", "2027-02-15", true), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}
	wants(t, getAs(t, d, path, cookie).Body.String(), "S1")
}

func TestDropScreensAnswer404ForADropThatIsNotThere(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	for _, path := range []string{
		dropsPath + "/" + missingID.String(),
		dropsPath + "/not-an-id",
	} {
		if rec := getAs(t, d, path, cookie); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
		rec := postAs(t, d, path, dropForm(missingID.String(), "Дроп 1", "2027-02-15", true), cookie)
		if rec.Code != http.StatusNotFound {
			t.Errorf("POST %s status = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
	}
}
