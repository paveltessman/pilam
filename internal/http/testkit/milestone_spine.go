package testkit

import (
	"context"
	"net/http"
	"net/url"
	"testing"

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

// ParsedID reads an identifier a test carries as the screens carry it.
func ParsedID(t *testing.T, id string) ids.ID {
	t.Helper()

	parsed, err := ids.Parse(id)
	if err != nil {
		t.Fatalf("reading the identifier %q: %v", id, err)
	}
	return parsed
}
