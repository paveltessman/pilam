package models_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/http/paths"
	"github.com/paveltessman/pilam/internal/http/testkit"
	"github.com/paveltessman/pilam/internal/milestones"
	"github.com/paveltessman/pilam/internal/platform/date"
	"github.com/paveltessman/pilam/internal/platform/labels"
)

// The day the fixed clock stands on, and the two days either side of the due
// window that a state is read against.
var (
	today     = date.Of(testkit.Now.Date())
	yesterday = date.ISO(date.AddDays(today, -1))
	nextWeek  = date.ISO(date.AddDays(today, milestones.DueWindow))
	nextMonth = date.ISO(date.AddDays(today, 30))
)

// calendarModel writes a model with a card to read, and returns the id of it
// with the cookie every call of the test is made under.
func calendarModel(t *testing.T, d testkit.Deps) (string, *http.Cookie) {
	t.Helper()

	dropID := spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)
	return createdModel(t, d, cookie, dropID, "A-100"), cookie
}

// section is the calendar part of the card alone. The history below it names
// the same steps, so an assertion about the table reads this and not the whole
// screen.
func section(t *testing.T, body string) string {
	t.Helper()

	from := strings.Index(body, labels.MilestonesTitle)
	to := strings.Index(body, labels.AuditTitle)
	if from < 0 || to < from {
		t.Fatalf("the card holds no calendar section: %s", body)
	}
	return body[from:to]
}

func TestCalendarSectionShowsOneRowPerStepInPlanDateOrder(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)

	shipped := testkit.CreatedMilestoneType(t, d, cookie, "ship", "Shipped")
	fit := testkit.CreatedMilestoneType(t, d, cookie, "fit", "Fit approved")
	testkit.AddedMilestone(t, d, modelID, shipped, nextMonth)
	testkit.AddedMilestone(t, d, modelID, fit, nextWeek)

	body := testkit.GetAs(t, d, paths.Models+"/"+modelID, cookie).Body.String()

	testkit.Wants(t, body,
		labels.MilestonesTitle,
		labels.MilestonesStep,
		labels.MilestonesBaseline,
		labels.MilestonesPlan,
		labels.MilestonesFact,
		labels.MilestonesState,
		labels.MilestonesSlip,
		"fit", "ship",
		labels.Date(date.MustParse(nextWeek)),
		labels.Date(date.MustParse(nextMonth)),
	)
	table := section(t, body)
	if strings.Index(table, "fit") > strings.Index(table, "ship") {
		t.Error("the section does not show the steps in plan date order")
	}
}

func TestCalendarSectionReadsEveryStepAgainstToday(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)

	late := testkit.CreatedMilestoneType(t, d, cookie, "late", "The one that slipped")
	due := testkit.CreatedMilestoneType(t, d, cookie, "due", "The one that comes up")
	planned := testkit.CreatedMilestoneType(t, d, cookie, "planned", "The one far out")
	done := testkit.CreatedMilestoneType(t, d, cookie, "done", "The one that happened")

	testkit.AddedMilestone(t, d, modelID, late, yesterday)
	testkit.AddedMilestone(t, d, modelID, due, nextWeek)
	testkit.AddedMilestone(t, d, modelID, planned, nextMonth)
	stamped := testkit.AddedMilestone(t, d, modelID, done, yesterday)
	testkit.EditedMilestone(t, d, stamped, milestones.MilestoneUpdateParams{
		Plan: stamped.Plan, Fact: date.MustParse(yesterday), Active: true,
	})

	body := testkit.GetAs(t, d, paths.Models+"/"+modelID, cookie).Body.String()

	testkit.Wants(t, body,
		labels.MilestonesStateLate,
		labels.MilestonesStateDue,
		labels.MilestonesStatePlanned,
		labels.MilestonesStateDone,
		labels.MilestonesOnTime,
	)
}

func TestCalendarSectionKeepsTheBaselineQuietUntilThePlanMoves(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)

	fit := testkit.CreatedMilestoneType(t, d, cookie, "fit", "Fit approved")
	one := testkit.AddedMilestone(t, d, modelID, fit, nextWeek)

	path := paths.Models + "/" + modelID
	quiet := section(t, testkit.GetAs(t, d, path, cookie).Body.String())
	if strings.Contains(quiet, labels.Delta(0)) {
		t.Error("the section reads a slip while the plan stands on the baseline")
	}
	if strings.Count(quiet, labels.Date(one.Plan)) != 1 {
		t.Error("the section reads the baseline while the plan stands on it")
	}

	moved := date.AddDays(one.Plan, 12)
	testkit.EditedMilestone(t, d, one, milestones.MilestoneUpdateParams{Plan: moved, Active: true})

	body := testkit.GetAs(t, d, path, cookie).Body.String()
	testkit.Wants(t, body,
		labels.Date(one.Baseline),
		labels.Date(moved),
		labels.Delta(12),
	)
}

func TestCalendarSectionIsEmptyForAModelWithNoCalendar(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)

	body := testkit.GetAs(t, d, paths.Models+"/"+modelID, cookie).Body.String()
	testkit.Wants(t, body, labels.MilestonesTitle, labels.MilestonesEmpty)
}

func TestCalendarSectionMovesARetiredStepOutOfTheTable(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)

	fit := testkit.CreatedMilestoneType(t, d, cookie, "fit", "Fit approved")
	one := testkit.AddedMilestone(t, d, modelID, fit, nextWeek)
	testkit.EditedMilestone(t, d, one, milestones.MilestoneUpdateParams{Plan: one.Plan, Active: false})

	body := testkit.GetAs(t, d, paths.Models+"/"+modelID, cookie).Body.String()

	// The table is gone, because the model holds no active step any more, and
	// the retired step is named under it.
	testkit.Wants(t, section(t, body), labels.MilestonesEmpty, labels.MilestonesRetired, "fit")
}

func TestModelCardShowsTheHistoryOfItsStepsUnderTheStepName(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)

	fit := testkit.CreatedMilestoneType(t, d, cookie, "fit", "Fit approved")
	one := testkit.AddedMilestone(t, d, modelID, fit, nextWeek)
	moved := date.AddDays(one.Plan, 12)
	testkit.EditedMilestone(t, d, one, milestones.MilestoneUpdateParams{
		Plan: moved, Note: "the sample was late", Active: true,
	})

	body := testkit.GetAs(t, d, paths.Models+"/"+modelID, cookie).Body.String()

	planMoved := "fit — " + labels.MilestonesAuditPlan + ": " + labels.Date(one.Plan) + " → " + labels.Date(moved)
	testkit.Wants(t, body,
		labels.AuditTitle,
		labels.ModelsAuditCreated,
		planMoved,
		"fit — "+labels.MilestonesAuditNote+": the sample was late",
	)

	// Newest first: the model was created before its step moved.
	if strings.Index(body, planMoved) > strings.Index(body, labels.ModelsAuditCreated) {
		t.Error("the trail does not show the newest change first")
	}
}
