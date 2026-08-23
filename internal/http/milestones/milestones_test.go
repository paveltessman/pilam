package milestones_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/http/milestones/views"
	"github.com/paveltessman/pilam/internal/http/paths"
	"github.com/paveltessman/pilam/internal/http/testkit"
	"github.com/paveltessman/pilam/internal/milestone"
	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

var typeRoutes = []testkit.Route{
	{Method: http.MethodGet, Path: paths.Milestones},
	{Method: http.MethodPost, Path: paths.Milestones},
	{Method: http.MethodGet, Path: paths.Milestones + "/new"},
	{Method: http.MethodGet, Path: paths.Milestones + "/" + testkit.MissingID.String()},
	{Method: http.MethodPost, Path: paths.Milestones + "/" + testkit.MissingID.String()},
}

// typeForm is what the create and the edit screen post.
func typeForm(name, description string, active bool) url.Values {
	form := url.Values{
		views.FieldName:        {name},
		views.FieldDescription: {description},
	}
	if active {
		form.Set(views.FieldActive, "true")
	}
	return form
}

// createdType posts the create form and returns the id of the type it wrote.
func createdType(t *testing.T, d testkit.Deps, cookie *http.Cookie, name, description string) string {
	t.Helper()

	rec := testkit.PostAs(t, d, paths.Milestones, typeForm(name, description, true), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("creating type %q: status = %d, want %d: %s", name, rec.Code, http.StatusSeeOther, rec.Body)
	}
	return testkit.IDOfRedirect(t, rec, paths.Milestones)
}

func TestMilestoneTypeScreenNamesFieldsThatServiceRejects(t *testing.T) {
	pairs := [][2]string{
		{views.FieldName, milestone.FieldName},
		{views.FieldDescription, milestone.FieldDescription},
		{views.FieldActive, milestone.FieldActive},
	}
	for _, pair := range pairs {
		if pair[0] != pair[1] {
			t.Errorf("the view posts %q, the service rejects %q", pair[0], pair[1])
		}
	}
}

func TestMilestoneTypesSectionRefusesMember(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	for _, r := range typeRoutes {
		rec := testkit.Call(t, d, r, typeForm("Production", "Sample ready", true), cookie)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s status = %d, want %d", r.Method, r.Path, rec.Code, http.StatusForbidden)
		}
	}
}

func TestMilestoneTypesSectionNeedsLogin(t *testing.T) {
	d := testkit.NewDeps(t)

	for _, r := range typeRoutes {
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

func TestNavOffersMilestoneTypesToRootOnly(t *testing.T) {
	d := testkit.NewDeps(t)
	link := `href="` + paths.Milestones + `"`

	root := testkit.GetAs(t, d, paths.Models, testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)).Body.String()
	testkit.Wants(t, root, link)

	member := testkit.GetAs(t, d, paths.Models, testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)).Body.String()
	if strings.Contains(member, link) {
		t.Errorf("a member is offered %s", link)
	}
}

func TestMilestoneTypesListIsEmptyBeforeAnyType(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	rec := testkit.GetAs(t, d, paths.Milestones, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	testkit.Wants(t, rec.Body.String(), labels.MilestoneTypesEmpty)
}

func TestMilestoneTypesListOrdersByShortName(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	createdType(t, d, cookie, "Production", "Sample ready")
	createdType(t, d, cookie, "Fabric order", "Fabric ordered")

	body := testkit.GetAs(t, d, paths.Milestones, cookie).Body.String()
	testkit.Wants(t, body, "Production", "Fabric order", "Sample ready", "Fabric ordered")

	if strings.Index(body, "Fabric order") > strings.Index(body, "Production") {
		t.Error("the list does not order by the short name")
	}
}

func TestMilestoneTypeCreateWritesItsAuditEntry(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	id := createdType(t, d, cookie, "Production", "Sample ready")

	entries := testkit.EntriesOf(t, trail, audit.EntityMilestoneType, id)
	if len(entries) != 1 {
		t.Fatalf("the trail holds %d entries, want 1: %+v", len(entries), entries)
	}
	if entries[0].Action != audit.ActionCreated {
		t.Errorf("action = %q, want %q", entries[0].Action, audit.ActionCreated)
	}
	if entries[0].New != "Production" {
		t.Errorf("new value = %q, want %q", entries[0].New, "Production")
	}
}

func TestMilestoneTypeCreateRejectsAnEmptyForm(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	rec := testkit.PostAs(t, d, paths.Milestones, typeForm("", "", true), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	testkit.Wants(t, rec.Body.String(), testkit.Message(validate.Required))
	if len(trail.Actions()) != 0 {
		t.Errorf("a refused create left %v behind", trail.Actions())
	}
}

func TestMilestoneTypeCreateRejectsNameAlreadyHeld(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	createdType(t, d, cookie, "Production", "Sample ready")

	rec := testkit.PostAs(t, d, paths.Milestones, typeForm("production", "Second sample", true), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	testkit.Wants(t, rec.Body.String(), testkit.Message(validate.Taken))
}

func TestMilestoneTypeCardReportsWhatItSaved(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	id := createdType(t, d, cookie, "Production", "Sample ready")
	path := paths.Milestones + "/" + id

	testkit.Wants(t, testkit.GetAs(t, d, path+"?saved=1", cookie).Body.String(), labels.Saved)
	if body := testkit.GetAs(t, d, path, cookie).Body.String(); strings.Contains(body, labels.Saved) {
		t.Error("the card reports saved changes without a write before it")
	}
}

func TestMilestoneTypeEditWritesNameAndDescription(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	id := createdType(t, d, cookie, "Production", "Sample ready")
	path := paths.Milestones + "/" + id

	rec := testkit.PostAs(t, d, path, typeForm("Sample", "Sample accepted", true), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	testkit.Wants(t, testkit.GetAs(t, d, path, cookie).Body.String(), "Sample", "Sample accepted")

	// The create, then one entry per field that moved.
	if got, want := len(testkit.EntriesOf(t, trail, audit.EntityMilestoneType, id)), 3; got != want {
		t.Errorf("the trail holds %d entries, want %d", got, want)
	}
}

// A retired type keeps its row on the list, marked as inactive.
func TestMilestoneTypeEditRetiresType(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	id := createdType(t, d, cookie, "Production", "Sample ready")

	rec := testkit.PostAs(t, d, paths.Milestones+"/"+id, typeForm("Production", "Sample ready", false), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	testkit.Wants(t, testkit.GetAs(t, d, paths.Milestones, cookie).Body.String(), "Production", labels.StateInactive)

	actions := testkit.ActionsOf(testkit.EntriesOf(t, trail, audit.EntityMilestoneType, id))
	if len(actions) != 2 || actions[0] != audit.ActionDeactivated {
		t.Errorf("the trail holds %v, want a deactivation over the create", actions)
	}
}

// An edit that moves nothing writes nothing, and the card still reports itself
// as saved.
func TestMilestoneTypeEditThatMovesNothingWritesNothing(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	id := createdType(t, d, cookie, "Production", "Sample ready")

	rec := testkit.PostAs(t, d, paths.Milestones+"/"+id, typeForm("Production", "Sample ready", true), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got, want := len(testkit.EntriesOf(t, trail, audit.EntityMilestoneType, id)), 1; got != want {
		t.Errorf("the trail holds %d entries, want %d", got, want)
	}
}

func TestMilestoneTypeScreensAnswer404ForATypeThatIsNotThere(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	for _, path := range []string{
		paths.Milestones + "/" + testkit.MissingID.String(),
		paths.Milestones + "/not-an-id",
	} {
		if rec := testkit.GetAs(t, d, path, cookie); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
		rec := testkit.PostAs(t, d, path, typeForm("Production", "Sample ready", true), cookie)
		if rec.Code != http.StatusNotFound {
			t.Errorf("POST %s status = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
	}
}

// The card of one type carries what the trail recorded about that type: when,
// who, and what changed.
func TestMilestoneTypeCardShowsTheAuditTrail(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	id := createdType(t, d, cookie, "Production", "Sample ready")
	path := paths.Milestones + "/" + id

	rec := testkit.PostAs(t, d, path, typeForm("Sample", "Sample accepted", false), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	body := testkit.GetAs(t, d, path, cookie).Body.String()
	name := labels.AuditName + ": Production → Sample"
	description := labels.MilestoneTypesAuditDescription + ": Sample ready → Sample accepted"

	testkit.Wants(t, body,
		labels.AuditTitle,
		labels.DateTime(testkit.Now),
		labels.Name("Barbara", "Liskov"),
		labels.MilestoneTypesAuditCreated,
		labels.AuditOff,
		name,
		description,
	)

	// Newest first: the create is the oldest of the four entries.
	if strings.Index(body, labels.MilestoneTypesAuditCreated) < strings.Index(body, name) {
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
func TestRefusedMilestoneTypeEditKeepsTheTrailOnTheCard(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	id := createdType(t, d, cookie, "Production", "Sample ready")
	path := paths.Milestones + "/" + id

	rec := testkit.PostAs(t, d, path, typeForm("", "Sample ready", true), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	testkit.Wants(t, rec.Body.String(), labels.AuditTitle, labels.MilestoneTypesAuditCreated)
}
