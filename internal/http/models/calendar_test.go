package models_test

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/http/models/views"
	"github.com/paveltessman/pilam/internal/http/paths"
	"github.com/paveltessman/pilam/internal/http/testkit"
	"github.com/paveltessman/pilam/internal/milestones"
	"github.com/paveltessman/pilam/internal/platform/date"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/validate"
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

	table := section(t, testkit.GetAs(t, d, paths.Models+"/"+modelID, cookie).Body.String())

	testkit.Wants(t, table,
		labels.MilestonesTitle,
		labels.MilestonesStep,
		labels.MilestonesBaseline,
		labels.MilestonesPlan,
		labels.MilestonesFact,
		labels.MilestonesState,
		labels.MilestonesSlip,
		"fit", "ship",
		`value="`+nextWeek+`"`,
		`value="`+nextMonth+`"`,
	)
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
		Fact: date.MustParse(yesterday), Active: true,
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
	if strings.Contains(quiet, labels.Date(one.Baseline)) {
		t.Error("the section reads the baseline while the plan stands on it")
	}

	moved := date.AddDays(one.Plan, 12)
	testkit.MovedPlan(t, d, one, moved)

	testkit.Wants(t, section(t, testkit.GetAs(t, d, path, cookie).Body.String()),
		labels.Date(one.Baseline),
		`value="`+date.ISO(moved)+`"`,
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
	testkit.EditedMilestone(t, d, one, milestones.MilestoneUpdateParams{Active: false})

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
	testkit.MovedPlan(t, d, one, moved)
	testkit.EditedMilestone(t, d, one, milestones.MilestoneUpdateParams{
		Note: "the sample was late", Active: true,
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

// The target date of the drop the spine builds, and the day the fixed clock
// stands on.
const dropTarget = "2027-02-15"

// milestoneForm is what one row of the calendar posts.
func milestoneForm(fact, note, action string) url.Values {
	form := url.Values{
		views.FieldMilestoneFact:   {fact},
		views.FieldMilestoneNote:   {note},
		views.FieldMilestoneActive: {"true"},
	}
	if action != "" {
		form.Set(views.FieldRowAction, action)
	}
	return form
}

// calendarRows is the identifier of every step the section shows, in the order
// it shows them. Every row posts to a path that ends with its own identifier.
func calendarRows(t *testing.T, d testkit.Deps, cookie *http.Cookie, modelID string) []string {
	t.Helper()

	table := section(t, testkit.GetAs(t, d, paths.Models+"/"+modelID, cookie).Body.String())
	prefix := paths.Models + "/" + modelID + "/milestones/"

	var rows []string
	for rest := table; ; {
		_, remainder, found := strings.Cut(rest, `action="`+prefix)
		if !found {
			return rows
		}
		id, after, _ := strings.Cut(remainder, `"`)
		rest = after
		// The two controls of the section post under the same prefix, and
		// neither of them names a step.
		if _, err := ids.Parse(id); err != nil {
			continue
		}
		rows = append(rows, id)
	}
}

// calendarPath is where one row of the calendar posts.
func calendarPath(modelID string, one milestones.Milestone) string {
	return paths.Models + "/" + modelID + "/milestones/" + one.ID.String()
}

// planPath is where the plan date of one step posts.
func planPath(modelID, milestoneID string) string {
	return paths.Models + "/" + modelID + "/milestones/" + milestoneID + "/plan"
}

// moveForm is what the plan date box of one row posts: the date alone.
func moveForm(plan string) url.Values {
	return url.Values{views.FieldMilestonePlan: {plan}}
}

// confirmForm is what the move panel posts back. shift false is the switch the
// user turned off, which writes the one step.
func confirmForm(plan string, shift bool) url.Values {
	form := moveForm(plan)
	form.Set(views.FieldMilestoneConfirm, "true")
	if shift {
		form.Set(views.FieldMilestoneShift, "true")
	}
	return form
}

// aChain writes a model whose calendar holds one step per plan date given, and
// returns the steps in the order the dates name them.
func aChain(t *testing.T, d testkit.Deps, cookie *http.Cookie, modelID string, dates ...string) []milestones.Milestone {
	t.Helper()

	chain := make([]milestones.Milestone, len(dates))
	for i, day := range dates {
		step := testkit.CreatedMilestoneType(t, d, cookie, "step"+strconv.Itoa(i), "Step "+strconv.Itoa(i))
		chain[i] = testkit.AddedMilestone(t, d, modelID, step, day)
	}
	return chain
}

// heldPlans is the plan date of every step the section shows, in the order it
// shows them. It reads the boxes of the table, which carry the wire form.
func heldPlans(t *testing.T, d testkit.Deps, cookie *http.Cookie, modelID string) []string {
	t.Helper()

	table := section(t, testkit.GetAs(t, d, paths.Models+"/"+modelID, cookie).Body.String())

	// The date box of a row. The row also carries the plan as a hidden field,
	// so the marker names the visible box and not that one.
	const box = `type="date" name="` + views.FieldMilestonePlan + `" value="`

	var held []string
	for rest := table; ; {
		_, remainder, found := strings.Cut(rest, box)
		if !found {
			return held
		}
		day, after, _ := strings.Cut(remainder, `"`)
		rest = after
		held = append(held, day)
	}
}

// movePanel is the panel that asks about a move, and nothing else of the
// section. The table under it names the same steps.
func movePanel(t *testing.T, body string) string {
	t.Helper()

	from := strings.Index(body, labels.MilestonesMoveTitle)
	to := strings.Index(body, labels.ActionCancel)
	if from < 0 || to < from {
		t.Fatalf("the section holds no move panel: %s", body)
	}
	return body[from:to]
}

func TestMovingAPlanDateLaterAsksBeforeItWrites(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)
	chain := aChain(t, d, cookie, modelID, "2027-01-04", "2027-01-04", "2027-02-01")

	moved := date.MustParse("2027-01-14")
	rec := testkit.PostAs(t, d, planPath(modelID, chain[0].ID.String()), moveForm(date.ISO(moved)), cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body)
	}

	// The panel names the step the user moved and the step the shift carries,
	// each with the date it leaves and the date it takes.
	panel := movePanel(t, rec.Body.String())
	testkit.Wants(t, panel,
		labels.MilestonesMoveHint,
		labels.MilestonesMoveFrom,
		labels.MilestonesMoveTo,
		labels.MilestonesMoveShift,
		labels.ActionConfirmMove,
		"step0", "step2",
		labels.Date(chain[0].Plan),
		labels.Date(moved),
		labels.Date(chain[2].Plan),
		labels.Date(date.AddDays(chain[2].Plan, 10)),
	)

	// The step on the same day as the one the user moved is not later than it,
	// so the panel leaves it out.
	if strings.Contains(panel, "step1") {
		t.Error("the panel moves a step that stands on the same day")
	}

	// Nothing is written until the user answers.
	want := []string{"2027-01-04", "2027-01-04", "2027-02-01"}
	if got := heldPlans(t, d, cookie, modelID); !slices.Equal(got, want) {
		t.Errorf("plan dates = %v, want %v: the panel wrote before it asked", got, want)
	}
}

func TestConfirmingTheMoveShiftsTheLaterSteps(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)
	chain := aChain(t, d, cookie, modelID, "2027-01-04", "2027-01-20", "2027-02-01")

	// The last step is done, and work that is done does not move.
	testkit.EditedMilestone(t, d, chain[2], milestones.MilestoneUpdateParams{
		Fact: date.MustParse(yesterday), Active: true,
	})

	form := confirmForm("2027-01-14", true)
	rec := testkit.PostAs(t, d, planPath(modelID, chain[0].ID.String()), form, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	want := []string{"2027-01-14", "2027-01-30", "2027-02-01"}
	if got := heldPlans(t, d, cookie, modelID); !slices.Equal(got, want) {
		t.Errorf("plan dates = %v, want %v", got, want)
	}
}

func TestConfirmingWithTheSwitchOffMovesTheOneStep(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)
	chain := aChain(t, d, cookie, modelID, "2027-01-04", "2027-01-20")

	form := confirmForm("2027-01-14", false)
	rec := testkit.PostAs(t, d, planPath(modelID, chain[0].ID.String()), form, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	want := []string{"2027-01-14", "2027-01-20"}
	if got := heldPlans(t, d, cookie, modelID); !slices.Equal(got, want) {
		t.Errorf("plan dates = %v, want %v", got, want)
	}
}

func TestMovingAPlanDateEarlierWritesWithNoQuestion(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)
	chain := aChain(t, d, cookie, modelID, "2027-01-04", "2027-01-20")

	rec := testkit.PostAs(t, d, planPath(modelID, chain[1].ID.String()), moveForm("2027-01-10"), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	// An earlier date moves its own step. Pulling a chain forward is a plan.
	want := []string{"2027-01-04", "2027-01-10"}
	if got := heldPlans(t, d, cookie, modelID); !slices.Equal(got, want) {
		t.Errorf("plan dates = %v, want %v", got, want)
	}
}

func TestTheMovedStepsEachHoldATrailEntry(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)
	chain := aChain(t, d, cookie, modelID, "2027-01-04", "2027-01-20")

	form := confirmForm("2027-01-14", true)
	if rec := testkit.PostAs(t, d, planPath(modelID, chain[0].ID.String()), form, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	body := testkit.GetAs(t, d, paths.Models+"/"+modelID, cookie).Body.String()
	testkit.Wants(t, body,
		"step0 — "+labels.MilestonesAuditPlan+": "+labels.Date(chain[0].Plan)+" → "+labels.Date(date.MustParse("2027-01-14")),
		"step1 — "+labels.MilestonesAuditPlan+": "+labels.Date(chain[1].Plan)+" → "+labels.Date(date.MustParse("2027-01-30")),
	)
}

func TestMovingThePlanDateOfAStepThatIsNotThereIs404(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)

	path := planPath(modelID, testkit.MissingID.String())
	if rec := testkit.PostAs(t, d, path, moveForm(nextMonth), cookie); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d: %s", rec.Code, http.StatusNotFound, rec.Body)
	}
}

func TestMovingAPlanDateRefusesAnEmptyBox(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)
	chain := aChain(t, d, cookie, modelID, "2027-01-04")

	rec := testkit.PostAs(t, d, planPath(modelID, chain[0].ID.String()), moveForm(""), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusUnprocessableEntity, rec.Body)
	}
	testkit.Wants(t, rec.Body.String(), testkit.Message(validate.Required))
}

// aTemplate writes a critical path with one step per offset, and returns the id
// of the template.
func aTemplate(t *testing.T, d testkit.Deps, cookie *http.Cookie, name string, offsets ...string) string {
	t.Helper()

	templateID := testkit.CreatedMilestoneTemplate(t, d, cookie, name, "The chain")
	for i, offset := range offsets {
		step := testkit.CreatedMilestoneType(t, d, cookie, "step"+strconv.Itoa(i), "Step "+strconv.Itoa(i))
		testkit.AddedTemplateItem(t, d, cookie, templateID, step, offset)
	}
	return templateID
}

func TestApplyingATemplateDatesEveryStepFromTheDrop(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)
	templateID := aTemplate(t, d, cookie, "Import, rail", "-270", "-14")

	path := paths.Models + "/" + modelID
	form := url.Values{views.FieldMilestoneTemplate: {templateID}}
	rec := testkit.PostAs(t, d, path+"/milestones/template", form, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	target := date.MustParse(dropTarget)
	testkit.Wants(t, section(t, testkit.GetAs(t, d, path, cookie).Body.String()),
		"step0", "step1",
		`value="`+date.ISO(date.AddDays(target, -270))+`"`,
		`value="`+date.ISO(date.AddDays(target, -14))+`"`,
	)
}

func TestApplyingATemplateAgainAddsOnlyTheMissingStepAndMovesNoDate(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)
	templateID := aTemplate(t, d, cookie, "Import, rail", "-270", "-14")

	path := paths.Models + "/" + modelID
	apply := url.Values{views.FieldMilestoneTemplate: {templateID}}
	if rec := testkit.PostAs(t, d, path+"/milestones/template", apply, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	// The user moves one plan date, with the shift off so the step after it
	// stays, and a third step joins the path after that.
	rows := calendarRows(t, d, cookie, modelID)
	moved := testkit.PostAs(t, d, planPath(modelID, rows[0]), confirmForm(nextMonth, false), cookie)
	if moved.Code != http.StatusSeeOther {
		t.Fatalf("moving a plan date: status = %d, want %d: %s", moved.Code, http.StatusSeeOther, moved.Body)
	}
	late := testkit.CreatedMilestoneType(t, d, cookie, "sale", "Ready for sale")
	testkit.AddedTemplateItem(t, d, cookie, templateID, late, "-7")

	if rec := testkit.PostAs(t, d, path+"/milestones/template", apply, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	target := date.MustParse(dropTarget)
	body := testkit.GetAs(t, d, path, cookie).Body.String()
	testkit.Wants(t, section(t, body),
		// The step the user moved stands where the user put it.
		`value="`+nextMonth+`"`,
		`value="`+date.ISO(date.AddDays(target, -14))+`"`,
		`value="`+date.ISO(date.AddDays(target, -7))+`"`,
	)
	if got := strings.Count(section(t, body), `value="`+date.ISO(date.AddDays(target, -270))+`"`); got != 0 {
		t.Error("the second apply wrote the step the model already holds again")
	}

	// The trail counts what each write wrote.
	testkit.Wants(t, body,
		labels.ModelsAuditTemplate+": Import, rail (2)",
		labels.ModelsAuditTemplate+": Import, rail (1)",
	)
}

func TestApplyingATemplateRefusesWhatThePickerDoesNotOffer(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)
	path := paths.Models + "/" + modelID

	missing := url.Values{views.FieldMilestoneTemplate: {testkit.MissingID.String()}}
	rec := testkit.PostAs(t, d, path+"/milestones/template", missing, cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusUnprocessableEntity, rec.Body)
	}
	testkit.Wants(t, rec.Body.String(), testkit.Message(validate.NotAllowed))

	empty := url.Values{views.FieldMilestoneTemplate: {""}}
	if rec := testkit.PostAs(t, d, path+"/milestones/template", empty, cookie); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("an empty choice: status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
}

func TestAddingOneStepWritesItWithTheDateTheUserStates(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)
	shoot := testkit.CreatedMilestoneType(t, d, cookie, "shoot", "Photo shoot")

	path := paths.Models + "/" + modelID
	form := url.Values{views.FieldMilestoneType: {shoot}, views.FieldMilestonePlan: {nextMonth}}
	rec := testkit.PostAs(t, d, path+"/milestones", form, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	body := testkit.GetAs(t, d, path, cookie).Body.String()
	testkit.Wants(t, section(t, body), "shoot", `value="`+nextMonth+`"`)
	testkit.Wants(t, body, "shoot — "+labels.MilestonesAuditCreated+": "+labels.Date(date.MustParse(nextMonth)))
}

func TestAddingOneStepNeedsATypeAndADate(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)
	shoot := testkit.CreatedMilestoneType(t, d, cookie, "shoot", "Photo shoot")
	path := paths.Models + "/" + modelID + "/milestones"

	noDate := url.Values{views.FieldMilestoneType: {shoot}}
	if rec := testkit.PostAs(t, d, path, noDate, cookie); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("no date: status = %d, want %d: %s", rec.Code, http.StatusUnprocessableEntity, rec.Body)
	}

	noType := url.Values{views.FieldMilestonePlan: {nextMonth}}
	rec := testkit.PostAs(t, d, path, noType, cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("no type: status = %d, want %d: %s", rec.Code, http.StatusUnprocessableEntity, rec.Body)
	}
	testkit.Wants(t, rec.Body.String(), testkit.Message(validate.NotAllowed))
}

func TestAddingAStepTheModelAlreadyHoldsIsRefused(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)
	shoot := testkit.CreatedMilestoneType(t, d, cookie, "shoot", "Photo shoot")
	testkit.AddedMilestone(t, d, modelID, shoot, nextMonth)

	path := paths.Models + "/" + modelID + "/milestones"
	form := url.Values{views.FieldMilestoneType: {shoot}, views.FieldMilestonePlan: {nextWeek}}

	rec := testkit.PostAs(t, d, path, form, cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusUnprocessableEntity, rec.Body)
	}
	testkit.Wants(t, rec.Body.String(), testkit.Message(validate.Taken))
}

func TestSavingARowWritesTheNoteAndMovesNoDate(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)
	shoot := testkit.CreatedMilestoneType(t, d, cookie, "shoot", "Photo shoot")
	one := testkit.AddedMilestone(t, d, modelID, shoot, nextWeek)

	// The row carries the plan date the user typed as well. The write ignores
	// it, because a plan date carries the steps after it and posts on its own.
	form := milestoneForm("", "the sample was late", "")
	form.Set(views.FieldMilestonePlan, nextMonth)
	rec := testkit.PostAs(t, d, calendarPath(modelID, one), form, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	body := testkit.GetAs(t, d, paths.Models+"/"+modelID, cookie).Body.String()
	table := section(t, body)
	testkit.Wants(t, table, "the sample was late", `value="`+nextWeek+`"`)
	if strings.Contains(table, `value="`+nextMonth+`"`) {
		t.Error("the row save moved the plan date")
	}
	if strings.Contains(body, "shoot — "+labels.MilestonesAuditPlan) {
		t.Error("the row save recorded a plan date move")
	}
}

func TestMovingThePlanDateKeepsTheBaseline(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)
	shoot := testkit.CreatedMilestoneType(t, d, cookie, "shoot", "Photo shoot")
	one := testkit.AddedMilestone(t, d, modelID, shoot, nextWeek)

	rec := testkit.PostAs(t, d, planPath(modelID, one.ID.String()), moveForm(nextMonth), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	body := testkit.GetAs(t, d, paths.Models+"/"+modelID, cookie).Body.String()
	testkit.Wants(t, section(t, body), `value="`+nextMonth+`"`, labels.Date(one.Baseline))
	testkit.Wants(t, body, "shoot — "+labels.MilestonesAuditPlan)
}

func TestStampingTheFactAsTodayAndClearingItAgain(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)
	shoot := testkit.CreatedMilestoneType(t, d, cookie, "shoot", "Photo shoot")
	one := testkit.AddedMilestone(t, d, modelID, shoot, nextWeek)
	path := calendarPath(modelID, one)

	stamp := milestoneForm("", "", views.RowFactToday)
	if rec := testkit.PostAs(t, d, path, stamp, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("stamping: status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	table := section(t, testkit.GetAs(t, d, paths.Models+"/"+modelID, cookie).Body.String())
	testkit.Wants(t, table, `value="`+date.ISO(today)+`"`, labels.MilestonesStateDone)

	clear := milestoneForm(date.ISO(today), "", views.RowFactClear)
	if rec := testkit.PostAs(t, d, path, clear, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("clearing: status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	body := testkit.GetAs(t, d, paths.Models+"/"+modelID, cookie).Body.String()
	if strings.Contains(section(t, body), labels.MilestonesStateDone) {
		t.Error("the step still reads as done after the fact was cleared")
	}
	testkit.Wants(t, body,
		"shoot — "+labels.MilestonesAuditFact,
		"shoot — "+labels.MilestonesAuditFactCleared,
	)
}

func TestSavingARowRefusesAFactDateInTheFuture(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)
	shoot := testkit.CreatedMilestoneType(t, d, cookie, "shoot", "Photo shoot")
	one := testkit.AddedMilestone(t, d, modelID, shoot, nextWeek)

	form := milestoneForm(date.ISO(date.AddDays(today, 1)), "", "")
	rec := testkit.PostAs(t, d, calendarPath(modelID, one), form, cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusUnprocessableEntity, rec.Body)
	}
	testkit.Wants(t, rec.Body.String(), testkit.Message(validate.NotAllowed))
}

func TestSavingARowRefusesANoteThatIsTooLong(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)
	shoot := testkit.CreatedMilestoneType(t, d, cookie, "shoot", "Photo shoot")
	one := testkit.AddedMilestone(t, d, modelID, shoot, nextWeek)

	form := milestoneForm("", strings.Repeat("я", milestones.MaxNoteLen+1), "")
	rec := testkit.PostAs(t, d, calendarPath(modelID, one), form, cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusUnprocessableEntity, rec.Body)
	}
	tooLong := labels.Message(validate.FieldError{
		Field: milestones.FieldNote,
		Code:  validate.TooLong,
		Arg:   strconv.Itoa(milestones.MaxNoteLen),
	})
	testkit.Wants(t, rec.Body.String(), tooLong)
}

func TestTakingAStepOffTheCalendarAndPuttingItBack(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)
	shoot := testkit.CreatedMilestoneType(t, d, cookie, "shoot", "Photo shoot")
	one := testkit.AddedMilestone(t, d, modelID, shoot, nextWeek)
	path := calendarPath(modelID, one)

	retire := milestoneForm("", "", views.RowRetire)
	if rec := testkit.PostAs(t, d, path, retire, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("retiring: status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}
	testkit.Wants(t, section(t, testkit.GetAs(t, d, paths.Models+"/"+modelID, cookie).Body.String()),
		labels.MilestonesRetired, labels.MilestonesEmpty)

	restore := milestoneForm("", "", views.RowRestore)
	if rec := testkit.PostAs(t, d, path, restore, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("restoring: status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	table := section(t, testkit.GetAs(t, d, paths.Models+"/"+modelID, cookie).Body.String())
	if strings.Contains(table, labels.MilestonesRetired) {
		t.Error("the step is still off the calendar")
	}
	testkit.Wants(t, table, `value="`+nextWeek+`"`)
}

func TestAMemberEditsTheCalendarOfAModel(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, root := calendarModel(t, d)
	shoot := testkit.CreatedMilestoneType(t, d, root, "shoot", "Photo shoot")
	one := testkit.AddedMilestone(t, d, modelID, shoot, nextWeek)

	member := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)
	form := milestoneForm("", "the sample was late", "")
	if rec := testkit.PostAs(t, d, calendarPath(modelID, one), form, member); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	// A member moves a plan date as well, and the move is its own write.
	move := testkit.PostAs(t, d, planPath(modelID, one.ID.String()), moveForm(nextMonth), member)
	if move.Code != http.StatusSeeOther {
		t.Fatalf("moving a plan date: status = %d, want %d: %s", move.Code, http.StatusSeeOther, move.Body)
	}

	testkit.Wants(t, section(t, testkit.GetAs(t, d, paths.Models+"/"+modelID, member).Body.String()),
		"the sample was late",
		`value="`+nextMonth+`"`)
}

func TestSavingARowThatIsNotThereIs404(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)

	path := paths.Models + "/" + modelID + "/milestones/" + testkit.MissingID.String()
	form := milestoneForm("", "", "")
	if rec := testkit.PostAs(t, d, path, form, cookie); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d: %s", rec.Code, http.StatusNotFound, rec.Body)
	}
}

// A calendar from the create screen: the model takes a critical path the moment
// it is written, and the card holds the dated steps.

// aDefaultTemplate writes the critical path a new model starts on, with one
// step per offset.
func aDefaultTemplate(t *testing.T, d testkit.Deps, cookie *http.Cookie, name string, offsets ...string) string {
	t.Helper()

	templateID := testkit.CreatedDefaultMilestoneTemplate(t, d, cookie, name, "The chain")
	for i, offset := range offsets {
		step := testkit.CreatedMilestoneType(t, d, cookie, "step"+strconv.Itoa(i), "Step "+strconv.Itoa(i))
		testkit.AddedTemplateItem(t, d, cookie, templateID, step, offset)
	}
	return templateID
}

// createForm is what the create screen posts, with the critical path the user
// picked. An empty templateID is the choice that builds no calendar.
func createForm(dropID, article, templateID string) url.Values {
	form := modelForm(dropID, article, true)
	form.Set(views.FieldMilestoneTemplate, templateID)
	return form
}

func TestModelCreateOffersTheCriticalPathsAndPreselectsTheDefault(t *testing.T) {
	d := testkit.NewDeps(t)
	spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	air := testkit.CreatedMilestoneTemplate(t, d, cookie, "Import, air", "The fast chain")
	rail := aDefaultTemplate(t, d, cookie, "Import, rail", "-270")

	body := testkit.GetAs(t, d, paths.Models+"/new", cookie).Body.String()
	testkit.Wants(t, body,
		labels.MilestonesTemplate,
		labels.ModelsTemplateHint,
		labels.ModelsNoCalendar,
		"Import, air",
		"Import, rail",
		// The default path stands in the control before the user reads it.
		`value="`+rail+`" selected>`,
	)
	if strings.Contains(body, `value="`+air+`" selected>`) {
		t.Error("the create screen preselects a path that is not the default one")
	}
}

func TestModelCreateLeavesTheCriticalPathOutWhileThereIsNone(t *testing.T) {
	d := testkit.NewDeps(t)
	spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	body := testkit.GetAs(t, d, paths.Models+"/new", cookie).Body.String()
	if strings.Contains(body, labels.ModelsNoCalendar) {
		t.Error("the create screen offers a critical path control while there is no path")
	}
}

func TestModelCreatedWithADefaultPathHoldsEveryStepDatedFromTheDrop(t *testing.T) {
	d := testkit.NewDeps(t)
	dropID := spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)
	templateID := aDefaultTemplate(t, d, cookie, "Import, rail", "-270", "-14")

	rec := testkit.PostAs(t, d, paths.Models, createForm(dropID, "A-200", templateID), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}
	modelID := testkit.IDOfRedirect(t, rec, paths.Models)

	target := date.MustParse(dropTarget)
	testkit.Wants(t, section(t, testkit.GetAs(t, d, paths.Models+"/"+modelID, cookie).Body.String()),
		"step0", "step1",
		`value="`+date.ISO(date.AddDays(target, -270))+`"`,
		`value="`+date.ISO(date.AddDays(target, -14))+`"`,
	)
}

func TestModelCreatedWithNoCalendarHoldsAnEmptySection(t *testing.T) {
	d := testkit.NewDeps(t)
	dropID := spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)
	aDefaultTemplate(t, d, cookie, "Import, rail", "-270")

	rec := testkit.PostAs(t, d, paths.Models, createForm(dropID, "A-200", ""), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}
	modelID := testkit.IDOfRedirect(t, rec, paths.Models)

	// The add control below the table names every step the model lacks, so the
	// empty calendar is read off the dates and not off the step names.
	table := section(t, testkit.GetAs(t, d, paths.Models+"/"+modelID, cookie).Body.String())
	testkit.Wants(t, table, labels.MilestonesEmpty)
	dated := date.ISO(date.AddDays(date.MustParse(dropTarget), -270))
	if strings.Contains(table, `value="`+dated+`"`) {
		t.Error("the model the user asked for with no calendar holds a step")
	}
}

func TestModelCreateRefusesAPathThePickerDoesNotOfferAndWritesNoModel(t *testing.T) {
	d := testkit.NewDeps(t)
	dropID := spine(t, d)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)
	aDefaultTemplate(t, d, cookie, "Import, rail", "-270")

	form := createForm(dropID, "A-200", testkit.MissingID.String())
	rec := testkit.PostAs(t, d, paths.Models, form, cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusUnprocessableEntity, rec.Body)
	}
	testkit.Wants(t, rec.Body.String(), testkit.Message(validate.NotAllowed))

	if list := testkit.GetAs(t, d, paths.Models, cookie).Body.String(); strings.Contains(list, "A-200") {
		t.Error("the refused create wrote a model")
	}
}

// The card above the table names the step the model comes to next: the active
// step that holds no fact date and stands earliest.
func TestModelCardNamesTheStepThatComesNext(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)

	done := testkit.CreatedMilestoneType(t, d, cookie, "stamped", "The one that happened")
	soon := testkit.CreatedMilestoneType(t, d, cookie, "nearest", "The one that comes up")
	far := testkit.CreatedMilestoneType(t, d, cookie, "farthest", "The one far out")

	stamped := testkit.AddedMilestone(t, d, modelID, done, yesterday)
	testkit.EditedMilestone(t, d, stamped, milestones.MilestoneUpdateParams{
		Fact: date.MustParse(yesterday), Active: true,
	})
	testkit.AddedMilestone(t, d, modelID, soon, nextWeek)
	testkit.AddedMilestone(t, d, modelID, far, nextMonth)

	body := testkit.GetAs(t, d, paths.Models+"/"+modelID, cookie).Body.String()

	testkit.Wants(t, body,
		labels.ModelsNext,
		"nearest",
		labels.Date(date.MustParse(nextWeek))+labels.Separator+
			fmt.Sprintf(labels.ModelsNextIn, labels.Days(milestones.DueWindow)),
	)
	if strings.Contains(body[:strings.Index(body, labels.MilestonesTitle)], "farthest") {
		t.Error("the card names a step later than the nearest one")
	}
}

// A model with nothing left to do says so, and a model with no calendar has
// nothing to say at all.
func TestModelCardReadsACalendarWithNothingLeftToDo(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)
	path := paths.Models + "/" + modelID

	if body := testkit.GetAs(t, d, path, cookie).Body.String(); strings.Contains(body, labels.ModelsNext) {
		t.Error("a model with no calendar carries the card that names the next step")
	}

	fit := testkit.CreatedMilestoneType(t, d, cookie, "fit", "Fit approved")
	one := testkit.AddedMilestone(t, d, modelID, fit, yesterday)
	testkit.EditedMilestone(t, d, one, milestones.MilestoneUpdateParams{
		Fact: date.MustParse(yesterday), Active: true,
	})

	testkit.Wants(t, testkit.GetAs(t, d, path, cookie).Body.String(),
		labels.ModelsNext,
		labels.ModelsNextAllClosed,
	)
}
