package testkit

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/auth"
	milestoneviews "github.com/paveltessman/pilam/internal/http/milestones/views"
	"github.com/paveltessman/pilam/internal/http/paths"
	"github.com/paveltessman/pilam/internal/milestones"
	"github.com/paveltessman/pilam/internal/platform/date"
	"github.com/paveltessman/pilam/internal/platform/ids"
)

// The milestone spine: a calendar needs a milestone type, and a model to hang
// it on. A test of the calendar section builds both through these calls.

// ActorContext is a context carrying an identity.
//
// A test that writes through a service rather than through a screen needs one,
// because the trail records every write against an actor.
func ActorContext(t *testing.T, userID ids.ID) context.Context {
	t.Helper()
	return auth.NewContext(t.Context(), auth.Identity{UserID: userID, Role: auth.RootRole})
}

// MilestoneTypeForm is what the milestone type create and edit screen post.
func MilestoneTypeForm(name, description string, active bool) url.Values {
	form := url.Values{
		milestoneviews.FieldName:        {name},
		milestoneviews.FieldDescription: {description},
	}
	if active {
		form.Set(milestoneviews.FieldActive, "true")
	}
	return form
}

// CreatedMilestoneType posts the create form and returns the id of the type it
// wrote. The screen is root only.
func CreatedMilestoneType(t *testing.T, d Deps, cookie *http.Cookie, name, description string) string {
	t.Helper()

	rec := PostAs(t, d, paths.MilestoneTypes, MilestoneTypeForm(name, description, true), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("creating milestone type %q: status = %d, want %d: %s",
			name, rec.Code, http.StatusSeeOther, rec.Body)
	}
	return IDOfRedirect(t, rec, paths.MilestoneTypes)
}

// AddedMilestone writes one step onto a model and returns it. plan is the wire
// form of a date, and the baseline starts on the same day.
func AddedMilestone(t *testing.T, d Deps, modelID, typeID, plan string) milestones.Milestone {
	t.Helper()

	ctx := ActorContext(t, RootUserID)
	written, err := d.MilestoneSvc.AddMilestone(ctx, ParsedID(t, modelID), ParsedID(t, typeID), date.MustParse(plan))
	if err != nil {
		t.Fatalf("adding a milestone of type %s: %v", typeID, err)
	}
	return written
}

// EditedMilestone writes one row of a calendar, the way the section does.
func EditedMilestone(t *testing.T, d Deps, one milestones.Milestone, in milestones.MilestoneUpdateParams) {
	t.Helper()

	if err := d.MilestoneSvc.UpdateMilestone(ActorContext(t, RootUserID), one.ID, in); err != nil {
		t.Fatalf("editing milestone %s: %v", one.ID, err)
	}
}

// MovedPlan writes the plan date of one step, with the shift turned off, so it
// moves that step and nothing else.
func MovedPlan(t *testing.T, d Deps, one milestones.Milestone, plan time.Time) {
	t.Helper()

	if _, err := d.MilestoneSvc.MovePlan(ActorContext(t, RootUserID), one.ID, plan, false); err != nil {
		t.Fatalf("moving the plan date of milestone %s: %v", one.ID, err)
	}
}

// ParsedID reads an identifier a test carries as the screens carry it.
func ParsedID(t *testing.T, id string) ids.ID {
	t.Helper()

	parsed, err := ids.Parse(id)
	if err != nil {
		t.Fatalf("reading the identifier %q: %v", id, err)
	}
	return parsed
}

// MilestoneTemplateForm is what the template create and edit screen post.
func MilestoneTemplateForm(name, description string, isDefault, active bool) url.Values {
	form := url.Values{
		milestoneviews.FieldName:        {name},
		milestoneviews.FieldDescription: {description},
	}
	if isDefault {
		form.Set(milestoneviews.FieldDefault, "true")
	}
	if active {
		form.Set(milestoneviews.FieldActive, "true")
	}
	return form
}

// CreatedMilestoneTemplate posts the create form and returns the id of the
// template it wrote. The screen is root only.
func CreatedMilestoneTemplate(t *testing.T, d Deps, cookie *http.Cookie, name, description string) string {
	t.Helper()
	return createdTemplate(t, d, cookie, name, description, false)
}

// CreatedDefaultMilestoneTemplate writes the template a new model starts on.
// One template is the default, so a second call moves the flag to the template
// it writes.
func CreatedDefaultMilestoneTemplate(t *testing.T, d Deps, cookie *http.Cookie, name, description string) string {
	t.Helper()
	return createdTemplate(t, d, cookie, name, description, true)
}

func createdTemplate(t *testing.T, d Deps, cookie *http.Cookie, name, description string, isDefault bool) string {
	t.Helper()

	form := MilestoneTemplateForm(name, description, isDefault, true)
	rec := PostAs(t, d, paths.MilestoneTemplates, form, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("creating milestone template %q: status = %d, want %d: %s",
			name, rec.Code, http.StatusSeeOther, rec.Body)
	}
	return IDOfRedirect(t, rec, paths.MilestoneTemplates)
}

// AddedTemplateItem appends one step to a template, at the offset given in days
// from the target date of the drop.
func AddedTemplateItem(t *testing.T, d Deps, cookie *http.Cookie, templateID, typeID, offset string) {
	t.Helper()

	form := url.Values{milestoneviews.FieldType: {typeID}, milestoneviews.FieldOffset: {offset}}
	path := paths.MilestoneTemplates + "/" + templateID + "/items"
	if rec := PostAs(t, d, path, form, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("adding item %s at %s: status = %d, want %d: %s",
			typeID, offset, rec.Code, http.StatusSeeOther, rec.Body)
	}
}
