package milestones_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/http/milestones/views"
	"github.com/paveltessman/pilam/internal/http/paths"
	"github.com/paveltessman/pilam/internal/http/testkit"
	"github.com/paveltessman/pilam/internal/milestones"
	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

var templateRoutes = []testkit.Route{
	{Method: http.MethodPost, Path: paths.MilestoneTemplates},
	{Method: http.MethodGet, Path: paths.MilestoneTemplateNew},
	{Method: http.MethodGet, Path: paths.MilestoneTemplates + "/" + testkit.MissingID.String()},
	{Method: http.MethodPost, Path: paths.MilestoneTemplates + "/" + testkit.MissingID.String()},
	{Method: http.MethodPost, Path: paths.MilestoneTemplates + "/" + testkit.MissingID.String() + "/items"},
	{Method: http.MethodPost, Path: paths.MilestoneTemplates + "/" + testkit.MissingID.String() + "/items/order"},
}

// templateForm is what the create and the edit screen post.
func templateForm(name, description string, isDefault, active bool) url.Values {
	form := url.Values{
		views.FieldName:        {name},
		views.FieldDescription: {description},
	}
	if isDefault {
		form.Set(views.FieldDefault, "true")
	}
	if active {
		form.Set(views.FieldActive, "true")
	}
	return form
}

// createdTemplate posts the create form and returns the id it wrote.
func createdTemplate(t *testing.T, d testkit.Deps, cookie *http.Cookie, name, description string) string {
	t.Helper()

	rec := testkit.PostAs(t, d, paths.MilestoneTemplates, templateForm(name, description, false, true), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("creating template %q: status = %d, want %d: %s",
			name, rec.Code, http.StatusSeeOther, rec.Body)
	}
	return testkit.IDOfRedirect(t, rec, paths.MilestoneTemplates)
}

// addedItem appends one step to a template and returns the id of the item.
func addedItem(t *testing.T, d testkit.Deps, cookie *http.Cookie, templateID, typeID, offset string) string {
	t.Helper()

	form := url.Values{views.FieldType: {typeID}, views.FieldOffset: {offset}}
	path := paths.MilestoneTemplates + "/" + templateID + "/items"
	rec := testkit.PostAs(t, d, path, form, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("adding item %s: status = %d, want %d: %s", typeID, rec.Code, http.StatusSeeOther, rec.Body)
	}
	return lastItemID(t, d, cookie, templateID)
}

// itemIDs reads the identifiers of the item rows out of the editor, in the
// order the table shows them. The edit form of a row names it.
var editFormID = regexp.MustCompile(`id="edit-([0-9a-f-]{36})"`)

func itemIDs(t *testing.T, d testkit.Deps, cookie *http.Cookie, templateID string) []string {
	t.Helper()

	body := testkit.GetAs(t, d, paths.MilestoneTemplates+"/"+templateID, cookie).Body.String()
	var found []string
	for _, match := range editFormID.FindAllStringSubmatch(body, -1) {
		found = append(found, match[1])
	}
	return found
}

func lastItemID(t *testing.T, d testkit.Deps, cookie *http.Cookie, templateID string) string {
	t.Helper()

	held := itemIDs(t, d, cookie, templateID)
	if len(held) == 0 {
		t.Fatal("the editor holds no item rows")
	}
	return held[len(held)-1]
}

// chainOfTwo is a template holding two steps, at -270 and -240.
func chainOfTwo(t *testing.T, d testkit.Deps, cookie *http.Cookie) (string, []string) {
	t.Helper()

	templateID := createdTemplate(t, d, cookie, "Import, rail", "The rail chain")
	first := createdType(t, d, cookie, "tech1", "Tech pack 1")
	second := createdType(t, d, cookie, "sample1", "1st sample")

	addedItem(t, d, cookie, templateID, first, "-270")
	addedItem(t, d, cookie, templateID, second, "-240")
	return templateID, itemIDs(t, d, cookie, templateID)
}

func TestTemplateScreenNamesFieldsThatServiceRejects(t *testing.T) {
	pairs := [][2]string{
		{views.FieldDefault, milestones.FieldDefault},
		{views.FieldType, milestones.FieldType},
		{views.FieldOffset, milestones.FieldOffset},
		{views.FieldGap, milestones.FieldGap},
	}
	for _, pair := range pairs {
		if pair[0] != pair[1] {
			t.Errorf("the view posts %q, the service rejects %q", pair[0], pair[1])
		}
	}
}

func TestTemplateScreensRefuseMember(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	for _, r := range templateRoutes {
		rec := testkit.Call(t, d, r, templateForm("Import, rail", "Rail", false, true), cookie)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s status = %d, want %d", r.Method, r.Path, rec.Code, http.StatusForbidden)
		}
	}
}

func TestTemplateScreensNeedLogin(t *testing.T) {
	d := testkit.NewDeps(t)

	for _, r := range templateRoutes {
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

func TestMilestoneSectionListsTemplatesAndTypes(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	// Both lists report themselves empty before anything is written.
	body := testkit.GetAs(t, d, paths.Milestones, cookie).Body.String()
	testkit.Wants(t, body, labels.MilestoneTemplatesEmpty, labels.MilestoneTypesEmpty)

	createdTemplate(t, d, cookie, "Import, rail", "The rail chain")
	createdType(t, d, cookie, "tech1", "Tech pack 1")

	body = testkit.GetAs(t, d, paths.Milestones, cookie).Body.String()
	testkit.Wants(t, body, "Import, rail", "The rail chain", "tech1", "Tech pack 1")
}

func TestTemplateCreateWritesItsAuditEntry(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	id := createdTemplate(t, d, cookie, "Import, rail", "The rail chain")

	entries := testkit.EntriesOf(t, trail, audit.EntityMilestoneTemplate, id)
	if len(entries) != 1 {
		t.Fatalf("the trail holds %d entries, want 1: %+v", len(entries), entries)
	}
	if entries[0].Action != audit.ActionCreated || entries[0].New != "Import, rail" {
		t.Errorf("entry = %+v, want the template created", entries[0])
	}
}

func TestTemplateCreateRejectsAnEmptyForm(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	rec := testkit.PostAs(t, d, paths.MilestoneTemplates, templateForm("", "", false, true), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	testkit.Wants(t, rec.Body.String(), testkit.Message(validate.Required))
	if len(trail.Actions()) != 0 {
		t.Errorf("a refused create left %v behind", trail.Actions())
	}
}

func TestTemplateCreateRejectsNameAlreadyHeld(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	createdTemplate(t, d, cookie, "Import, rail", "The rail chain")

	rec := testkit.PostAs(t, d, paths.MilestoneTemplates,
		templateForm("IMPORT, RAIL", "Another", false, true), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	testkit.Wants(t, rec.Body.String(), testkit.Message(validate.Taken))
}

func TestTemplateEditNamesTheDefault(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	id := createdTemplate(t, d, cookie, "Import, rail", "The rail chain")
	path := paths.MilestoneTemplates + "/" + id

	rec := testkit.PostAs(t, d, path, templateForm("Import, rail", "The rail chain", true, true), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	// The list marks the default template.
	testkit.Wants(t, testkit.GetAs(t, d, paths.Milestones, cookie).Body.String(),
		labels.MilestoneTemplatesDefaultMark)
}

func TestTemplateEditRefusesADefaultThatIsRetired(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	id := createdTemplate(t, d, cookie, "Import, rail", "The rail chain")
	path := paths.MilestoneTemplates + "/" + id

	rec := testkit.PostAs(t, d, path, templateForm("Import, rail", "The rail chain", true, false), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	testkit.Wants(t, rec.Body.String(), testkit.Message(validate.NotAllowed))
}

func TestTemplateScreensAnswer404ForATemplateThatIsNotThere(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	for _, path := range []string{
		paths.MilestoneTemplates + "/" + testkit.MissingID.String(),
		paths.MilestoneTemplates + "/not-an-id",
	} {
		if rec := testkit.GetAs(t, d, path, cookie); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
		rec := testkit.PostAs(t, d, path, templateForm("Import, rail", "Rail", false, true), cookie)
		if rec.Code != http.StatusNotFound {
			t.Errorf("POST %s status = %d, want %d", path, rec.Code, http.StatusNotFound)
		}
	}
}

func TestTemplateEditorBuildsAChainFromNothing(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	empty := createdTemplate(t, d, cookie, "Domestic", "The domestic chain")

	// An empty template says so, and offers no step until a type exists.
	body := testkit.GetAs(t, d, paths.MilestoneTemplates+"/"+empty, cookie).Body.String()
	testkit.Wants(t, body, labels.MilestoneItemsEmpty, labels.MilestoneItemsNoType)

	templateID, held := chainOfTwo(t, d, cookie)
	if len(held) != 2 {
		t.Fatalf("the editor holds %d items, want 2", len(held))
	}

	body = testkit.GetAs(t, d, paths.MilestoneTemplates+"/"+templateID, cookie).Body.String()
	// The offset of both rows, and the gap of the second one.
	testkit.Wants(t, body, `value="-270"`, `value="-240"`, `value="30"`)
}

func TestTemplateEditorOffersEachTypeOnce(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	templateID := createdTemplate(t, d, cookie, "Import, rail", "The rail chain")
	typeID := createdType(t, d, cookie, "tech1", "Tech pack 1")
	addedItem(t, d, cookie, templateID, typeID, "-270")

	// The one type the template holds leaves the picker, and it is the only
	// type there is.
	body := testkit.GetAs(t, d, paths.MilestoneTemplates+"/"+templateID, cookie).Body.String()
	testkit.Wants(t, body, labels.MilestoneItemsNoType)

	// Posting it again is refused all the same.
	form := url.Values{views.FieldType: {typeID}, views.FieldOffset: {"-240"}}
	rec := testkit.PostAs(t, d, paths.MilestoneTemplates+"/"+templateID+"/items", form, cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	testkit.Wants(t, rec.Body.String(), testkit.Message(validate.Taken))
}

func TestTemplateEditorRejectsAnOffsetAfterTheTargetDate(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	templateID := createdTemplate(t, d, cookie, "Import, rail", "The rail chain")
	typeID := createdType(t, d, cookie, "tech1", "Tech pack 1")

	form := url.Values{views.FieldType: {typeID}, views.FieldOffset: {"1"}}
	rec := testkit.PostAs(t, d, paths.MilestoneTemplates+"/"+templateID+"/items", form, cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	// The bound is part of the sentence: an offset is zero at the most.
	tooBig := labels.Message(validate.FieldError{Code: validate.TooBig, Arg: "0"})
	testkit.Wants(t, rec.Body.String(), tooBig)
}

func TestTemplateEditorMovesARowByItsOffset(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)
	templateID, items := chainOfTwo(t, d, cookie)

	// The row posts both columns. The offset moved, so the offset is the edit.
	form := url.Values{views.FieldOffset: {"-230"}, views.FieldGap: {"30"}}
	path := paths.MilestoneTemplates + "/" + templateID + "/items/" + items[1]
	if rec := testkit.PostAs(t, d, path, form, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	body := testkit.GetAs(t, d, paths.MilestoneTemplates+"/"+templateID, cookie).Body.String()
	testkit.Wants(t, body, `value="-230"`, `value="40"`)
}

func TestTemplateEditorMovesARowByItsGap(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)
	templateID, items := chainOfTwo(t, d, cookie)

	// The gap moved and the offset did not, so the gap is the edit: the item
	// above holds -270, so a gap of 10 stores -260.
	form := url.Values{views.FieldOffset: {"-240"}, views.FieldGap: {"10"}}
	path := paths.MilestoneTemplates + "/" + templateID + "/items/" + items[1]
	if rec := testkit.PostAs(t, d, path, form, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	body := testkit.GetAs(t, d, paths.MilestoneTemplates+"/"+templateID, cookie).Body.String()
	testkit.Wants(t, body, `value="-260"`, `value="10"`)
}

func TestTemplateEditorReordersTheRows(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)
	templateID, items := chainOfTwo(t, d, cookie)

	form := url.Values{views.FieldItemOrder: {items[1], items[0]}}
	path := paths.MilestoneTemplates + "/" + templateID + "/items/order"
	if rec := testkit.PostAs(t, d, path, form, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	got := itemIDs(t, d, cookie, templateID)
	if len(got) != 2 || got[0] != items[1] || got[1] != items[0] {
		t.Errorf("the order is %v, want %v", got, []string{items[1], items[0]})
	}

	// The item has no card of its own: the trail records it under the template.
	actions := testkit.ActionsOf(testkit.EntriesOf(t, trail, audit.EntityMilestoneTemplate, templateID))
	if !strings.Contains(strings.Join(actions, ","), audit.ActionItemsReordered) {
		t.Errorf("the trail of the template holds %v, want a reorder", actions)
	}
}

func TestTemplateEditorReportsAStaleOrder(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)
	templateID, items := chainOfTwo(t, d, cookie)

	// An order naming one of the two rows is a screen the user left open.
	form := url.Values{views.FieldItemOrder: {items[0]}}
	path := paths.MilestoneTemplates + "/" + templateID + "/items/order"
	rec := testkit.PostAs(t, d, path, form, cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	testkit.Wants(t, rec.Body.String(), labels.MilestoneItemsStale)
}

func TestTemplateEditorRemovesARow(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)
	templateID, items := chainOfTwo(t, d, cookie)

	path := paths.MilestoneTemplates + "/" + templateID + "/items/" + items[0] + "/remove"
	if rec := testkit.PostAs(t, d, path, nil, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	got := itemIDs(t, d, cookie, templateID)
	if len(got) != 1 || got[0] != items[1] {
		t.Errorf("the editor holds %v, want %v", got, []string{items[1]})
	}
}

func TestTemplateEditorPreviewsTheDatesOfATargetDate(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)
	templateID, _ := chainOfTwo(t, d, cookie)
	path := paths.MilestoneTemplates + "/" + templateID

	// No target date, no dates.
	body := testkit.GetAs(t, d, path, cookie).Body.String()
	if strings.Contains(body, "19.10.2025") {
		t.Error("the editor previews dates without a target date")
	}

	// The two dates of §10 of the design, for a drop on 16.07.2026.
	body = testkit.GetAs(t, d, path+"?"+views.FieldTarget+"=2026-07-16", cookie).Body.String()
	testkit.Wants(t, body, "19.10.2025", "18.11.2025")
}

func TestTemplateEditorKeepsThePreviewThroughAWrite(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)
	templateID, items := chainOfTwo(t, d, cookie)

	form := url.Values{
		views.FieldOffset: {"-230"},
		views.FieldGap:    {"30"},
		views.FieldTarget: {"2026-07-16"},
	}
	path := paths.MilestoneTemplates + "/" + templateID + "/items/" + items[1]
	rec := testkit.PostAs(t, d, path, form, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, views.FieldTarget+"=2026-07-16") {
		t.Errorf("the write redirects to %q, want the preview date on it", location)
	}
}

func TestTemplateEditorShowsTheAuditTrail(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)
	templateID, _ := chainOfTwo(t, d, cookie)

	body := testkit.GetAs(t, d, paths.MilestoneTemplates+"/"+templateID, cookie).Body.String()
	testkit.Wants(t, body,
		labels.AuditTitle,
		labels.MilestoneTemplatesAuditCreated,
		labels.MilestoneTemplatesAuditAdded,
		"tech1 (-270)",
	)
}
