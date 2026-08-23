package http

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/http/views"
	"github.com/paveltessman/pilam/internal/milestone"
	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

var milestoneTypeRoutes = []route{
	{http.MethodGet, milestoneTypesPath},
	{http.MethodPost, milestoneTypesPath},
	{http.MethodGet, milestoneTypesPath + "/new"},
	{http.MethodGet, milestoneTypesPath + "/" + missingID.String()},
	{http.MethodPost, milestoneTypesPath + "/" + missingID.String()},
}

// milestoneTypeForm is what the create and the edit screen post.
func milestoneTypeForm(name, description string, active bool) url.Values {
	form := url.Values{
		views.FieldMilestoneTypeName:        {name},
		views.FieldMilestoneTypeDescription: {description},
	}
	if active {
		form.Set(views.FieldMilestoneTypeActive, "true")
	}
	return form
}

// createdMilestoneType posts the create form and returns the id of the type it wrote.
func createdMilestoneType(t *testing.T, d Deps, cookie *http.Cookie, name, description string) string {
	t.Helper()

	rec := postAs(t, d, milestoneTypesPath, milestoneTypeForm(name, description, true), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("creating type %q: status = %d, want %d: %s", name, rec.Code, http.StatusSeeOther, rec.Body)
	}
	return idOfRedirect(t, rec, milestoneTypesPath)
}

func TestMilestoneTypeScreenNamesFieldsThatServiceRejects(t *testing.T) {
	pairs := [][2]string{
		{views.FieldMilestoneTypeName, milestone.FieldName},
		{views.FieldMilestoneTypeDescription, milestone.FieldDescription},
		{views.FieldMilestoneTypeActive, milestone.FieldActive},
	}
	for _, pair := range pairs {
		if pair[0] != pair[1] {
			t.Errorf("the view posts %q, the service rejects %q", pair[0], pair[1])
		}
	}
}

func TestMilestoneTypesSectionRefusesMember(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, testEmail, testPasswd)

	for _, r := range milestoneTypeRoutes {
		rec := call(t, d, r, milestoneTypeForm("Production", "Sample ready", true), cookie)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s status = %d, want %d", r.method, r.path, rec.Code, http.StatusForbidden)
		}
	}
}

func TestMilestoneTypesSectionNeedsLogin(t *testing.T) {
	d := deps(t)

	for _, r := range milestoneTypeRoutes {
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

func TestNavOffersMilestoneTypesToRootOnly(t *testing.T) {
	d := deps(t)
	link := `href="` + milestoneTypesPath + `"`

	root := getAs(t, d, successPath, loggedIn(t, d, rootEmail, rootPasswd)).Body.String()
	wants(t, root, link)

	member := getAs(t, d, successPath, loggedIn(t, d, testEmail, testPasswd)).Body.String()
	if strings.Contains(member, link) {
		t.Errorf("a member is offered %s", link)
	}
}

func TestMilestoneTypesListIsEmptyBeforeAnyType(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	rec := getAs(t, d, milestoneTypesPath, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	wants(t, rec.Body.String(), labels.MilestoneTypesEmpty)
}

func TestMilestoneTypesListOrdersByShortName(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	createdMilestoneType(t, d, cookie, "Production", "Sample ready")
	createdMilestoneType(t, d, cookie, "Fabric order", "Fabric ordered")

	body := getAs(t, d, milestoneTypesPath, cookie).Body.String()
	wants(t, body, "Production", "Fabric order", "Sample ready", "Fabric ordered")

	if strings.Index(body, "Fabric order") > strings.Index(body, "Production") {
		t.Error("the list does not order by the short name")
	}
}

func TestMilestoneTypeCreateWritesItsAuditEntry(t *testing.T) {
	d, trail := auditedDeps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	id := createdMilestoneType(t, d, cookie, "Production", "Sample ready")

	entries := entriesOf(t, trail, audit.EntityMilestoneType, id)
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
	d, trail := auditedDeps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	rec := postAs(t, d, milestoneTypesPath, milestoneTypeForm("", "", true), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	wants(t, rec.Body.String(), message(validate.Required))
	if len(trail.actions()) != 0 {
		t.Errorf("a refused create left %v behind", trail.actions())
	}
}

func TestMilestoneTypeCreateRejectsNameAlreadyHeld(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	createdMilestoneType(t, d, cookie, "Production", "Sample ready")

	rec := postAs(t, d, milestoneTypesPath, milestoneTypeForm("production", "Second sample", true), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	wants(t, rec.Body.String(), message(validate.Taken))
}

func TestMilestoneTypeCardReportsWhatItSaved(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	id := createdMilestoneType(t, d, cookie, "Production", "Sample ready")
	path := milestoneTypesPath + "/" + id

	wants(t, getAs(t, d, path+"?saved=1", cookie).Body.String(), labels.Saved)
	if body := getAs(t, d, path, cookie).Body.String(); strings.Contains(body, labels.Saved) {
		t.Error("the card reports saved changes without a write before it")
	}
}

func TestMilestoneTypeEditWritesNameAndDescription(t *testing.T) {
	d, trail := auditedDeps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	id := createdMilestoneType(t, d, cookie, "Production", "Sample ready")
	path := milestoneTypesPath + "/" + id

	rec := postAs(t, d, path, milestoneTypeForm("Sample", "Sample accepted", true), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	wants(t, getAs(t, d, path, cookie).Body.String(), "Sample", "Sample accepted")

	// The create, then one entry per field that moved.
	if got, want := len(entriesOf(t, trail, audit.EntityMilestoneType, id)), 3; got != want {
		t.Errorf("the trail holds %d entries, want %d", got, want)
	}
}

// A retired type keeps its row on the list, marked as inactive.
func TestMilestoneTypeEditRetiresType(t *testing.T) {
	d, trail := auditedDeps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	id := createdMilestoneType(t, d, cookie, "Production", "Sample ready")

	rec := postAs(t, d, milestoneTypesPath+"/"+id, milestoneTypeForm("Production", "Sample ready", false), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	wants(t, getAs(t, d, milestoneTypesPath, cookie).Body.String(), "Production", labels.StateInactive)

	actions := actionsOf(entriesOf(t, trail, audit.EntityMilestoneType, id))
	if len(actions) != 2 || actions[0] != audit.ActionDeactivated {
		t.Errorf("the trail holds %v, want a deactivation over the create", actions)
	}
}

// An edit that moves nothing writes nothing, and the card still reports itself
// as saved.
func TestMilestoneTypeEditThatMovesNothingWritesNothing(t *testing.T) {
	d, trail := auditedDeps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	id := createdMilestoneType(t, d, cookie, "Production", "Sample ready")

	rec := postAs(t, d, milestoneTypesPath+"/"+id, milestoneTypeForm("Production", "Sample ready", true), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got, want := len(entriesOf(t, trail, audit.EntityMilestoneType, id)), 1; got != want {
		t.Errorf("the trail holds %d entries, want %d", got, want)
	}
}

func TestMilestoneTypeScreensAnswer404ForATypeThatIsNotThere(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	for _, path := range []string{
		milestoneTypesPath + "/" + missingID.String(),
		milestoneTypesPath + "/not-an-id",
	} {
		if rec := getAs(t, d, path, cookie); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
		rec := postAs(t, d, path, milestoneTypeForm("Production", "Sample ready", true), cookie)
		if rec.Code != http.StatusNotFound {
			t.Errorf("POST %s status = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
	}
}

// The card of one type carries what the trail recorded about that type: when,
// who, and what changed.
func TestMilestoneTypeCardShowsTheAuditTrail(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	id := createdMilestoneType(t, d, cookie, "Production", "Sample ready")
	path := milestoneTypesPath + "/" + id

	rec := postAs(t, d, path, milestoneTypeForm("Sample", "Sample accepted", false), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	body := getAs(t, d, path, cookie).Body.String()
	name := labels.AuditName + ": Production → Sample"
	description := labels.MilestoneTypesAuditDescription + ": Sample ready → Sample accepted"

	wants(t, body,
		labels.AuditTitle,
		labels.DateTime(testNow),
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
	d := deps(t)
	cookie := loggedIn(t, d, rootEmail, rootPasswd)

	id := createdMilestoneType(t, d, cookie, "Production", "Sample ready")
	path := milestoneTypesPath + "/" + id

	rec := postAs(t, d, path, milestoneTypeForm("", "Sample ready", true), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	wants(t, rec.Body.String(), labels.AuditTitle, labels.MilestoneTypesAuditCreated)
}
