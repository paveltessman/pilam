package models_test

import (
	"net/http"
	"net/url"
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
	if strings.Contains(quiet, labels.Date(one.Baseline)) {
		t.Error("the section reads the baseline while the plan stands on it")
	}

	moved := date.AddDays(one.Plan, 12)
	testkit.EditedMilestone(t, d, one, milestones.MilestoneUpdateParams{Plan: moved, Active: true})

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

// The target date of the drop the spine builds, and the day the fixed clock
// stands on.
const dropTarget = "2027-02-15"

// milestoneForm is what one row of the calendar posts. action is the button the
// user pressed, and it is empty for a plain save.
func milestoneForm(plan, fact, note, action string) url.Values {
	form := url.Values{
		views.FieldMilestonePlan:   {plan},
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

	// The user moves one plan date, and a third step joins the path after that.
	rows := calendarRows(t, d, cookie, modelID)
	moved := milestoneForm(nextMonth, "", "", "")
	if rec := testkit.PostAs(t, d, path+"/milestones/"+rows[0], moved, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("moving a plan date: status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
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

func TestSavingARowMovesThePlanDateAndWritesTheNote(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)
	shoot := testkit.CreatedMilestoneType(t, d, cookie, "shoot", "Photo shoot")
	one := testkit.AddedMilestone(t, d, modelID, shoot, nextWeek)

	form := milestoneForm(nextMonth, "", "the sample was late", "")
	rec := testkit.PostAs(t, d, calendarPath(modelID, one), form, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	body := testkit.GetAs(t, d, paths.Models+"/"+modelID, cookie).Body.String()
	testkit.Wants(t, section(t, body),
		`value="`+nextMonth+`"`,
		"the sample was late",
		labels.Date(one.Baseline),
	)
	testkit.Wants(t, body, "shoot — "+labels.MilestonesAuditPlan)
}

func TestStampingTheFactAsTodayAndClearingItAgain(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)
	shoot := testkit.CreatedMilestoneType(t, d, cookie, "shoot", "Photo shoot")
	one := testkit.AddedMilestone(t, d, modelID, shoot, nextWeek)
	path := calendarPath(modelID, one)

	stamp := milestoneForm(nextWeek, "", "", views.RowFactToday)
	if rec := testkit.PostAs(t, d, path, stamp, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("stamping: status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	table := section(t, testkit.GetAs(t, d, paths.Models+"/"+modelID, cookie).Body.String())
	testkit.Wants(t, table, `value="`+date.ISO(today)+`"`, labels.MilestonesStateDone)

	clear := milestoneForm(nextWeek, date.ISO(today), "", views.RowFactClear)
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

	form := milestoneForm(nextWeek, date.ISO(date.AddDays(today, 1)), "", "")
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

	form := milestoneForm(nextWeek, "", strings.Repeat("я", milestones.MaxNoteLen+1), "")
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

	retire := milestoneForm(nextWeek, "", "", views.RowRetire)
	if rec := testkit.PostAs(t, d, path, retire, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("retiring: status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}
	testkit.Wants(t, section(t, testkit.GetAs(t, d, paths.Models+"/"+modelID, cookie).Body.String()),
		labels.MilestonesRetired, labels.MilestonesEmpty)

	restore := milestoneForm(nextWeek, "", "", views.RowRestore)
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
	form := milestoneForm(nextMonth, "", "", "")
	if rec := testkit.PostAs(t, d, calendarPath(modelID, one), form, member); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body)
	}

	testkit.Wants(t, section(t, testkit.GetAs(t, d, paths.Models+"/"+modelID, member).Body.String()),
		`value="`+nextMonth+`"`)
}

func TestSavingARowThatIsNotThereIs404(t *testing.T) {
	d := testkit.NewDeps(t)
	modelID, cookie := calendarModel(t, d)

	path := paths.Models + "/" + modelID + "/milestones/" + testkit.MissingID.String()
	form := milestoneForm(nextWeek, "", "", "")
	if rec := testkit.PostAs(t, d, path, form, cookie); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d: %s", rec.Code, http.StatusNotFound, rec.Body)
	}
}
