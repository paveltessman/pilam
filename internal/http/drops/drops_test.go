package drops_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/http/drops/views"
	"github.com/paveltessman/pilam/internal/http/paths"
	"github.com/paveltessman/pilam/internal/http/testkit"
	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

var dropRoutes = []testkit.Route{
	{Method: http.MethodGet, Path: paths.Drops},
	{Method: http.MethodPost, Path: paths.Drops},
	{Method: http.MethodGet, Path: paths.Drops + "/new"},
	{Method: http.MethodGet, Path: paths.Drops + "/" + testkit.MissingID.String()},
	{Method: http.MethodPost, Path: paths.Drops + "/" + testkit.MissingID.String()},
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
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	for _, r := range dropRoutes {
		rec := testkit.Call(t, d, r, testkit.DropForm(testkit.MissingID.String(), "Дроп 1", "2027-02-15", true), cookie)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s status = %d, want %d", r.Method, r.Path, rec.Code, http.StatusForbidden)
		}
	}
}

func TestDropsSectionNeedsLogin(t *testing.T) {
	d := testkit.NewDeps(t)

	for _, r := range dropRoutes {
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

func TestDropsSectionAsksForASeasonFirst(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	list := testkit.GetAs(t, d, paths.Drops, cookie).Body.String()
	testkit.Wants(t, list, labels.DropsNoSeason)
	if strings.Contains(list, `href="`+paths.Drops+`/new"`) {
		t.Error("the list offers a create link with no season to put the drop in")
	}

	form := testkit.GetAs(t, d, paths.Drops+"/new", cookie)
	if form.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", form.Code, http.StatusOK)
	}
	testkit.Wants(t, form.Body.String(), labels.DropsNoSeason)
}

func TestDropAppearsUnderItsSeason(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	season := testkit.CreatedSeason(t, d, cookie, "S1", "2026-11-01")
	testkit.CreatedDrop(t, d, cookie, season, "Дроп 1", "2027-02-15")

	body := testkit.GetAs(t, d, paths.Drops, cookie).Body.String()
	testkit.Wants(t, body, "S1", "Дроп 1", "15.02.2027")

	if strings.Index(body, "S1") > strings.Index(body, "Дроп 1") {
		t.Error("the drop stands above the season that holds it")
	}
}

// The drops of one season order by the target date.
func TestDropsOrderByTargetDate(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	season := testkit.CreatedSeason(t, d, cookie, "S1", "2026-11-01")
	testkit.CreatedDrop(t, d, cookie, season, "Поздний", "2027-04-01")
	testkit.CreatedDrop(t, d, cookie, season, "Ранний", "2027-02-15")

	body := testkit.GetAs(t, d, paths.Drops, cookie).Body.String()
	if strings.Index(body, "Ранний") > strings.Index(body, "Поздний") {
		t.Error("the drops order by the name, want the target date")
	}
}

// The create form offers the active seasons alone: a drop goes into a season
// that is still in work.
func TestDropFormOffersTheActiveSeasons(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	open := testkit.CreatedSeason(t, d, cookie, "S1", "2026-11-01")
	closed := testkit.CreatedSeason(t, d, cookie, "AW26", "2026-05-01")

	rec := testkit.PostAs(t, d, paths.Seasons+"/"+closed, testkit.SeasonForm("AW26", "2026-05-01", false), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("turning a season off: status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	body := testkit.GetAs(t, d, paths.Drops+"/new", cookie).Body.String()
	testkit.Wants(t, body, `value="`+open+`"`)
	if strings.Contains(body, `value="`+closed+`"`) {
		t.Error("the form offers a season that is turned off")
	}
}

func TestDropCreateWritesItsAuditEntry(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	season := testkit.CreatedSeason(t, d, cookie, "S1", "2026-11-01")
	id := testkit.CreatedDrop(t, d, cookie, season, "Дроп 1", "2027-02-15")

	entries := testkit.EntriesOf(t, trail, audit.EntityDrop, id)
	if len(entries) != 1 {
		t.Fatalf("the trail holds %d entries, want 1: %+v", len(entries), entries)
	}
	if entries[0].Action != audit.ActionCreated {
		t.Errorf("action = %q, want %q", entries[0].Action, audit.ActionCreated)
	}
}

func TestDropCreateRejectsAnEmptyForm(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)
	testkit.CreatedSeason(t, d, cookie, "S1", "2026-11-01")

	rec := testkit.PostAs(t, d, paths.Drops, testkit.DropForm("", "", "", true), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	testkit.Wants(t, rec.Body.String(), testkit.Message(validate.Required))
}

// A season the select never offered is refused as a value outside the list,
// whether it is unreadable text or an id no row holds.
func TestDropCreateRejectsASeasonTheFormNeverOffered(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)
	testkit.CreatedSeason(t, d, cookie, "S1", "2026-11-01")

	for _, season := range []string{"not-an-id", testkit.MissingID.String()} {
		rec := testkit.PostAs(t, d, paths.Drops, testkit.DropForm(season, "Дроп 1", "2027-02-15", true), cookie)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("season %q: status = %d, want %d", season, rec.Code, http.StatusUnprocessableEntity)
		}
		testkit.Wants(t, rec.Body.String(), testkit.Message(validate.NotAllowed))
	}

	// The season create is the one write the trail holds.
	if actions := trail.Actions(); len(actions) != 1 {
		t.Errorf("the trail holds %v, want the season create alone", actions)
	}
}

func TestDropCreateRejectsTextThatIsNotADate(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)
	season := testkit.CreatedSeason(t, d, cookie, "S1", "2026-11-01")

	rec := testkit.PostAs(t, d, paths.Drops, testkit.DropForm(season, "Дроп 1", "15.02.2027", true), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}

	body := rec.Body.String()
	testkit.Wants(t, body, testkit.Message(validate.NotADate), "Дроп 1")
	if strings.Contains(body, testkit.Message(validate.Required)) {
		t.Error("the screen reports a missing value, want a date it could not read")
	}
}

// The name is unique inside a season, so the same name in another season is
// not a duplicate.
func TestDropNameIsUniqueInsideItsSeasonAlone(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	first := testkit.CreatedSeason(t, d, cookie, "S1", "2026-11-01")
	second := testkit.CreatedSeason(t, d, cookie, "S2", "2027-05-01")
	testkit.CreatedDrop(t, d, cookie, first, "Дроп 1", "2027-02-15")

	rec := testkit.PostAs(t, d, paths.Drops, testkit.DropForm(first, "дроп 1", "2027-03-15", true), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	testkit.Wants(t, rec.Body.String(), testkit.Message(validate.Taken))

	testkit.CreatedDrop(t, d, cookie, second, "Дроп 1", "2027-08-15")
}

func TestDropEditWritesTheNameAndTheDate(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	season := testkit.CreatedSeason(t, d, cookie, "S1", "2026-11-01")
	id := testkit.CreatedDrop(t, d, cookie, season, "Дроп 1", "2027-02-15")
	path := paths.Drops + "/" + id

	rec := testkit.PostAs(t, d, path, testkit.DropForm(season, "Дроп 2", "2027-03-15", true), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	testkit.Wants(t, testkit.GetAs(t, d, path, cookie).Body.String(), "Дроп 2", "2027-03-15")

	// The create, then one entry per field that moved.
	if got, want := len(testkit.EntriesOf(t, trail, audit.EntityDrop, id)), 3; got != want {
		t.Errorf("the trail holds %d entries, want %d", got, want)
	}
}

func TestDropEditTurnsTheDropOff(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	season := testkit.CreatedSeason(t, d, cookie, "S1", "2026-11-01")
	id := testkit.CreatedDrop(t, d, cookie, season, "Дроп 1", "2027-02-15")

	rec := testkit.PostAs(t, d, paths.Drops+"/"+id, testkit.DropForm(season, "Дроп 1", "2027-02-15", false), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	testkit.Wants(t, testkit.GetAs(t, d, paths.Drops, cookie).Body.String(), "Дроп 1", labels.StateInactive)

	actions := testkit.ActionsOf(testkit.EntriesOf(t, trail, audit.EntityDrop, id))
	if len(actions) != 2 || actions[0] != audit.ActionDeactivated {
		t.Errorf("the trail holds %v, want a deactivation over the create", actions)
	}
}

// Moving a drop to another season carries every model of it across, which is a
// season transition. The card offers no such control, and a posted season is
// not read.
func TestDropEditDoesNotMoveTheDropToAnotherSeason(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	first := testkit.CreatedSeason(t, d, cookie, "S1", "2026-11-01")
	second := testkit.CreatedSeason(t, d, cookie, "S2", "2027-05-01")
	id := testkit.CreatedDrop(t, d, cookie, first, "Дроп 1", "2027-02-15")
	path := paths.Drops + "/" + id

	card := testkit.GetAs(t, d, path, cookie).Body.String()
	testkit.Wants(t, card, "S1", labels.DropsSeasonFixed)
	if strings.Contains(card, `value="`+second+`"`) {
		t.Error("the card offers a change of season")
	}

	rec := testkit.PostAs(t, d, path, testkit.DropForm(second, "Дроп 1", "2027-02-15", true), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}
	testkit.Wants(t, testkit.GetAs(t, d, path, cookie).Body.String(), "S1")
}

func TestDropScreensAnswer404ForADropThatIsNotThere(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	for _, path := range []string{
		paths.Drops + "/" + testkit.MissingID.String(),
		paths.Drops + "/not-an-id",
	} {
		if rec := testkit.GetAs(t, d, path, cookie); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
		rec := testkit.PostAs(t, d, path, testkit.DropForm(testkit.MissingID.String(), "Дроп 1", "2027-02-15", true), cookie)
		if rec.Code != http.StatusNotFound {
			t.Errorf("POST %s status = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
	}
}

// The card of one drop carries what the trail recorded about that drop.
func TestDropCardShowsTheAuditTrail(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	seasonID := testkit.CreatedSeason(t, d, cookie, "S1", "2026-11-01")
	id := testkit.CreatedDrop(t, d, cookie, seasonID, "Drop 1", "2027-02-15")
	path := paths.Drops + "/" + id

	rec := testkit.PostAs(t, d, path, testkit.DropForm(seasonID, "Drop 2", "2027-03-15", true), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	body := testkit.GetAs(t, d, path, cookie).Body.String()
	name := labels.AuditName + ": Drop 1 → Drop 2"
	target := labels.DropsAuditTarget + ": 15.02.2027 → 15.03.2027"

	testkit.Wants(t, body,
		labels.AuditTitle,
		labels.DateTime(testkit.Now),
		labels.Name("Barbara", "Liskov"),
		labels.DropsAuditCreated,
		name,
		target,
	)

	// Newest first: the create is the oldest of the three entries.
	if strings.Index(body, labels.DropsAuditCreated) < strings.Index(body, name) {
		t.Error("the trail does not show the newest change first")
	}
}
