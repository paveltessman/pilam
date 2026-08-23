package seasons_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/http/paths"
	"github.com/paveltessman/pilam/internal/http/seasons/views"
	"github.com/paveltessman/pilam/internal/http/testkit"
	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

var seasonRoutes = []testkit.Route{
	{Method: http.MethodGet, Path: paths.Seasons},
	{Method: http.MethodPost, Path: paths.Seasons},
	{Method: http.MethodGet, Path: paths.Seasons + "/new"},
	{Method: http.MethodGet, Path: paths.Seasons + "/" + testkit.MissingID.String()},
	{Method: http.MethodPost, Path: paths.Seasons + "/" + testkit.MissingID.String()},
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
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	for _, r := range seasonRoutes {
		rec := testkit.Call(t, d, r, testkit.SeasonForm("S1", "2026-11-01", true), cookie)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s status = %d, want %d", r.Method, r.Path, rec.Code, http.StatusForbidden)
		}
	}
}

func TestSeasonsSectionNeedsLogin(t *testing.T) {
	d := testkit.NewDeps(t)

	for _, r := range seasonRoutes {
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

func TestSeasonsListIsEmptyBeforeAnySeason(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	rec := testkit.GetAs(t, d, paths.Seasons, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	testkit.Wants(t, rec.Body.String(), labels.SeasonsEmpty)
}

func TestSeasonsListOrdersByStartDate(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	testkit.CreatedSeason(t, d, cookie, "S2", "2027-05-01")
	testkit.CreatedSeason(t, d, cookie, "S1", "2026-11-01")

	body := testkit.GetAs(t, d, paths.Seasons, cookie).Body.String()
	testkit.Wants(t, body, "S1", "S2", "01.11.2026", "01.05.2027")

	if strings.Index(body, "S1") > strings.Index(body, "S2") {
		t.Error("the list orders by the name, want the start date")
	}
}

func TestSeasonCreateWritesItsAuditEntry(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	id := testkit.CreatedSeason(t, d, cookie, "S1", "2026-11-01")

	entries := testkit.EntriesOf(t, trail, audit.EntitySeason, id)
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
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	rec := testkit.PostAs(t, d, paths.Seasons, testkit.SeasonForm("", "", true), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	testkit.Wants(t, rec.Body.String(), testkit.Message(validate.Required))
}

func TestSeasonCreateRejectsTextThatIsNotADate(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	rec := testkit.PostAs(t, d, paths.Seasons, testkit.SeasonForm("S1", "01.11.2026", true), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}

	body := rec.Body.String()
	testkit.Wants(t, body, testkit.Message(validate.NotADate), "S1")
	if strings.Contains(body, testkit.Message(validate.Required)) {
		t.Error("the screen reports a missing value, want a date it could not read")
	}
	if len(trail.Actions()) != 0 {
		t.Errorf("a refused create left %v behind", trail.Actions())
	}
}

func TestSeasonCreateRejectsANameAlreadyHeld(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	testkit.CreatedSeason(t, d, cookie, "S1", "2026-11-01")

	rec := testkit.PostAs(t, d, paths.Seasons, testkit.SeasonForm("s1", "2027-05-01", true), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	testkit.Wants(t, rec.Body.String(), testkit.Message(validate.Taken))
}

func TestSeasonCardReportsWhatItSaved(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	id := testkit.CreatedSeason(t, d, cookie, "S1", "2026-11-01")
	path := paths.Seasons + "/" + id

	testkit.Wants(t, testkit.GetAs(t, d, path+"?saved=1", cookie).Body.String(), labels.Saved)
	if body := testkit.GetAs(t, d, path, cookie).Body.String(); strings.Contains(body, labels.Saved) {
		t.Error("the card reports saved changes without a write before it")
	}
}

func TestSeasonEditWritesTheNameAndTheDate(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	id := testkit.CreatedSeason(t, d, cookie, "S1", "2026-11-01")
	path := paths.Seasons + "/" + id

	rec := testkit.PostAs(t, d, path, testkit.SeasonForm("SS28", "2026-12-01", true), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	testkit.Wants(t, testkit.GetAs(t, d, path, cookie).Body.String(), "SS28", "2026-12-01")

	// The create, then one entry per field that moved.
	if got, want := len(testkit.EntriesOf(t, trail, audit.EntitySeason, id)), 3; got != want {
		t.Errorf("the trail holds %d entries, want %d", got, want)
	}
}

func TestSeasonEditTurnsTheSeasonOff(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	id := testkit.CreatedSeason(t, d, cookie, "S1", "2026-11-01")

	rec := testkit.PostAs(t, d, paths.Seasons+"/"+id, testkit.SeasonForm("S1", "2026-11-01", false), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	testkit.Wants(t, testkit.GetAs(t, d, paths.Seasons, cookie).Body.String(), "S1", labels.StateInactive)

	actions := testkit.ActionsOf(testkit.EntriesOf(t, trail, audit.EntitySeason, id))
	if len(actions) != 2 || actions[0] != audit.ActionDeactivated {
		t.Errorf("the trail holds %v, want a deactivation over the create", actions)
	}
}

// An edit that moves nothing writes nothing, and the card still reports itself
// as saved.
func TestSeasonEditThatMovesNothingWritesNothing(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	id := testkit.CreatedSeason(t, d, cookie, "S1", "2026-11-01")

	rec := testkit.PostAs(t, d, paths.Seasons+"/"+id, testkit.SeasonForm("S1", "2026-11-01", true), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got, want := len(testkit.EntriesOf(t, trail, audit.EntitySeason, id)), 1; got != want {
		t.Errorf("the trail holds %d entries, want %d", got, want)
	}
}

func TestSeasonScreensAnswer404ForASeasonThatIsNotThere(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	for _, path := range []string{
		paths.Seasons + "/" + testkit.MissingID.String(),
		paths.Seasons + "/not-an-id",
	} {
		if rec := testkit.GetAs(t, d, path, cookie); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
		rec := testkit.PostAs(t, d, path, testkit.SeasonForm("S1", "2026-11-01", true), cookie)
		if rec.Code != http.StatusNotFound {
			t.Errorf("POST %s status = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
	}
}

// The nav offers a root the sections a root reaches, and offers a member none
// of them.
func TestNavOffersTheCatalogSectionsToARootAlone(t *testing.T) {
	d := testkit.NewDeps(t)

	root := testkit.GetAs(t, d, paths.Models, testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)).Body.String()
	testkit.Wants(t, root, `href="`+paths.Seasons+`"`, `href="`+paths.Drops+`"`)

	member := testkit.GetAs(t, d, paths.Models, testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)).Body.String()
	for _, link := range []string{`href="` + paths.Seasons + `"`, `href="` + paths.Drops + `"`} {
		if strings.Contains(member, link) {
			t.Errorf("a member is offered %s", link)
		}
	}
}

// The card of one season carries what the trail recorded about that season:
// when, who, and what changed.
func TestSeasonCardShowsTheAuditTrail(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	id := testkit.CreatedSeason(t, d, cookie, "S1", "2026-11-01")
	path := paths.Seasons + "/" + id

	rec := testkit.PostAs(t, d, path, testkit.SeasonForm("SS28", "2026-12-01", false), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	body := testkit.GetAs(t, d, path, cookie).Body.String()
	name := labels.AuditName + ": S1 → SS28"
	start := labels.SeasonsAuditStart + ": 01.11.2026 → 01.12.2026"

	testkit.Wants(t, body,
		labels.AuditTitle,
		labels.DateTime(testkit.Now),
		labels.Name("Barbara", "Liskov"),
		labels.SeasonsAuditCreated,
		labels.AuditOff,
		name,
		start,
	)

	// Newest first: the create is the oldest of the four entries.
	if strings.Index(body, labels.SeasonsAuditCreated) < strings.Index(body, name) {
		t.Error("the trail does not show the newest change first")
	}
	// The flag says nothing the action does not.
	for _, unwanted := range []string{"true", "false"} {
		if strings.Contains(body, ": "+unwanted) {
			t.Errorf("the trail on the card reads out the stored value %q", unwanted)
		}
	}
}

// A refused edit comes back with the trail still under the form.
func TestRefusedSeasonEditKeepsTheTrailOnTheCard(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	id := testkit.CreatedSeason(t, d, cookie, "S1", "2026-11-01")
	path := paths.Seasons + "/" + id

	rec := testkit.PostAs(t, d, path, testkit.SeasonForm("", "2026-11-01", true), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	testkit.Wants(t, rec.Body.String(), labels.AuditTitle, labels.SeasonsAuditCreated)
}
