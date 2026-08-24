package milestones

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/platform/date"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

// aTemplate writes a template holding one item per offset given, and returns
// the template with the types of its items, in order.
func aTemplate(t *testing.T, svc *Service, ctx context.Context, offsets ...int) (Template, []Type) {
	t.Helper()

	template := milestoneTemplate(t, svc, ctx, "Import, rail", "The long chain")
	types := make([]Type, len(offsets))
	for i, offset := range offsets {
		types[i] = milestoneType(t, svc, ctx, "step"+strconv.Itoa(i), "Step "+strconv.Itoa(i))
		templateItem(t, svc, ctx, template.ID, types[i].ID, offset)
	}
	return template, types
}

// plans is the plan date of every milestone of a list, in order.
func plans(calendar []Milestone) []string {
	out := make([]string, len(calendar))
	for i, one := range calendar {
		out[i] = date.ISO(one.Plan)
	}
	return out
}

func TestApplyTemplateDatesEveryStepFromTheTargetDateOfTheDrop(t *testing.T) {
	svc, rows, trail := newTestService(t)
	ctx := signedIn(t)
	template, types := aTemplate(t, svc, ctx, -270, -88, -14)
	modelID := aModel(rows, "2026-07-16")
	trail.entries = nil

	written, err := svc.ApplyTemplate(ctx, modelID, template.ID)
	if err != nil {
		t.Fatalf("ApplyTemplate: %v", err)
	}

	if len(written) != len(types) {
		t.Fatalf("ApplyTemplate wrote %d milestones, want %d", len(written), len(types))
	}
	want := []string{"2025-10-19", "2026-04-19", "2026-07-02"}
	if got := plans(written); !slices.Equal(got, want) {
		t.Errorf("plan dates = %v, want %v", got, want)
	}
	for i, one := range written {
		if !date.Equal(one.Baseline, one.Plan) {
			t.Errorf("step %d baseline = %s, plan = %s: a calendar starts with the two equal",
				i, date.ISO(one.Baseline), date.ISO(one.Plan))
		}
		if one.Done() {
			t.Errorf("step %d holds a fact date, want none", i)
		}
		if !one.Active {
			t.Errorf("step %d is not active", i)
		}
		if one.ModelID != modelID {
			t.Errorf("step %d model = %s, want %s", i, one.ModelID, modelID)
		}
	}

	entry := trail.only(t)
	if entry.Entity != audit.EntityModel || entry.EntityID != modelID {
		t.Errorf("trail entry = %s %s, want %s %s", entry.Entity, entry.EntityID, audit.EntityModel, modelID)
	}
	if entry.Action != audit.ActionTemplateApplied {
		t.Errorf("trail action = %q, want %q", entry.Action, audit.ActionTemplateApplied)
	}
	if want := "Import, rail (3)"; entry.New != want {
		t.Errorf("trail value = %q, want %q", entry.New, want)
	}
}

func TestApplyTemplateAgainAddsOnlyTheMissingStepsAndMovesNoDate(t *testing.T) {
	svc, rows, trail := newTestService(t)
	ctx := signedIn(t)
	template, _ := aTemplate(t, svc, ctx, -270, -88)
	modelID := aModel(rows, "2026-07-16")

	if _, err := svc.ApplyTemplate(ctx, modelID, template.ID); err != nil {
		t.Fatalf("ApplyTemplate: %v", err)
	}

	// The user moves a plan date, and a new step joins the template after that.
	held, err := svc.Calendar(ctx, modelID)
	if err != nil {
		t.Fatalf("Calendar: %v", err)
	}
	moved := date.MustParse("2025-11-30")
	err = svc.UpdateMilestone(ctx, held[0].ID, MilestoneUpdateParams{Plan: moved, Active: true})
	if err != nil {
		t.Fatalf("UpdateMilestone: %v", err)
	}
	late := milestoneType(t, svc, ctx, "sale", "Ready for sale")
	templateItem(t, svc, ctx, template.ID, late.ID, -7)
	trail.entries = nil

	written, err := svc.ApplyTemplate(ctx, modelID, template.ID)
	if err != nil {
		t.Fatalf("ApplyTemplate: %v", err)
	}

	if len(written) != 1 || written[0].TypeID != late.ID {
		t.Fatalf("ApplyTemplate wrote %+v, want the one step the model lacks", written)
	}

	calendar, err := svc.Calendar(ctx, modelID)
	if err != nil {
		t.Fatalf("Calendar: %v", err)
	}
	want := []string{"2025-11-30", "2026-04-19", "2026-07-09"}
	got := make([]string, len(calendar))
	for i, row := range calendar {
		got[i] = date.ISO(row.Plan)
	}
	if !slices.Equal(got, want) {
		t.Errorf("plan dates = %v, want %v: the write touched a date it holds already", got, want)
	}

	if entry := trail.only(t); entry.New != "Import, rail (1)" {
		t.Errorf("trail value = %q, want %q", entry.New, "Import, rail (1)")
	}
}

func TestApplyTemplateWritesNothingWhenTheModelHoldsEveryStep(t *testing.T) {
	svc, rows, trail := newTestService(t)
	ctx := signedIn(t)
	template, _ := aTemplate(t, svc, ctx, -270, -88)
	modelID := aModel(rows, "2026-07-16")

	if _, err := svc.ApplyTemplate(ctx, modelID, template.ID); err != nil {
		t.Fatalf("ApplyTemplate: %v", err)
	}
	trail.entries = nil

	written, err := svc.ApplyTemplate(ctx, modelID, template.ID)
	if err != nil {
		t.Fatalf("ApplyTemplate: %v", err)
	}
	if len(written) != 0 {
		t.Errorf("ApplyTemplate wrote %+v, want none", written)
	}
	if len(trail.entries) != 0 {
		t.Errorf("the trail holds %+v, want nothing: no write happened", trail.entries)
	}
}

func TestApplyTemplateKeepsAStepTheUserRetired(t *testing.T) {
	svc, rows, trail := newTestService(t)
	ctx := signedIn(t)
	template, _ := aTemplate(t, svc, ctx, -270, -88)
	modelID := aModel(rows, "2026-07-16")

	if _, err := svc.ApplyTemplate(ctx, modelID, template.ID); err != nil {
		t.Fatalf("ApplyTemplate: %v", err)
	}
	held, err := svc.Calendar(ctx, modelID)
	if err != nil {
		t.Fatalf("Calendar: %v", err)
	}
	retired := held[0]
	err = svc.UpdateMilestone(ctx, retired.ID, MilestoneUpdateParams{Plan: retired.Plan, Active: false})
	if err != nil {
		t.Fatalf("UpdateMilestone: %v", err)
	}
	trail.entries = nil

	written, err := svc.ApplyTemplate(ctx, modelID, template.ID)
	if err != nil {
		t.Fatalf("ApplyTemplate: %v", err)
	}
	if len(written) != 0 {
		t.Errorf("ApplyTemplate wrote %+v: a retired step still holds its type", written)
	}
}

func TestApplyTemplateRefusesATemplateThePickerDoesNotOffer(t *testing.T) {
	svc, rows, _ := newTestService(t)
	ctx := signedIn(t)
	template, _ := aTemplate(t, svc, ctx, -270)
	modelID := aModel(rows, "2026-07-16")

	err := svc.UpdateTemplate(ctx, template.ID, TemplateUpdateParams{
		Name: "Import, rail", Description: "The long chain", Active: false,
	})
	if err != nil {
		t.Fatalf("UpdateTemplate: %v", err)
	}

	_, err = svc.ApplyTemplate(ctx, modelID, template.ID)
	rejects(t, err, FieldTemplate, validate.NotAllowed)

	_, err = svc.ApplyTemplate(ctx, modelID, ids.MustParse("01912345-0000-7000-8000-00000000ffff"))
	rejects(t, err, FieldTemplate, validate.NotAllowed)
}

func TestApplyTemplateReportsAModelThatIsNotThere(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := signedIn(t)
	template, _ := aTemplate(t, svc, ctx, -270)

	_, err := svc.ApplyTemplate(ctx, ids.MustParse("01912345-0000-7000-8000-00000000eeee"), template.ID)
	if !errors.Is(err, ErrNoModel) {
		t.Errorf("ApplyTemplate error = %v, want %v", err, ErrNoModel)
	}
}

func TestAddMilestoneWritesOneStepTheModelLacks(t *testing.T) {
	svc, rows, trail := newTestService(t)
	ctx := signedIn(t)
	modelID := aModel(rows, "2026-07-16")
	shoot := milestoneType(t, svc, ctx, "shoot", "Photo shoot")
	trail.entries = nil

	plan := date.MustParse("2026-05-20")
	written, err := svc.AddMilestone(ctx, modelID, shoot.ID, plan)
	if err != nil {
		t.Fatalf("AddMilestone: %v", err)
	}

	if !date.Equal(written.Baseline, plan) || !date.Equal(written.Plan, plan) {
		t.Errorf("baseline = %s, plan = %s, want both %s",
			date.ISO(written.Baseline), date.ISO(written.Plan), date.ISO(plan))
	}
	if !written.Active {
		t.Error("the step is not active")
	}

	entry := trail.only(t)
	if entry.Entity != audit.EntityMilestone || entry.EntityID != written.ID {
		t.Errorf("trail entry = %s %s, want %s %s",
			entry.Entity, entry.EntityID, audit.EntityMilestone, written.ID)
	}
	if entry.Action != audit.ActionCreated || entry.New != "2026-05-20" {
		t.Errorf("trail entry = %s %q, want %s %q",
			entry.Action, entry.New, audit.ActionCreated, "2026-05-20")
	}
}

func TestAddMilestoneRefusesAStepTheModelAlreadyHolds(t *testing.T) {
	svc, rows, _ := newTestService(t)
	ctx := signedIn(t)
	modelID := aModel(rows, "2026-07-16")
	shoot := milestoneType(t, svc, ctx, "shoot", "Photo shoot")

	plan := date.MustParse("2026-05-20")
	if _, err := svc.AddMilestone(ctx, modelID, shoot.ID, plan); err != nil {
		t.Fatalf("AddMilestone: %v", err)
	}

	_, err := svc.AddMilestone(ctx, modelID, shoot.ID, plan)
	if !errors.Is(err, ErrTypeOnModel) {
		t.Errorf("AddMilestone error = %v, want %v", err, ErrTypeOnModel)
	}
}

func TestAddMilestoneRefusesWhatThePickerDoesNotOffer(t *testing.T) {
	svc, rows, _ := newTestService(t)
	ctx := signedIn(t)
	modelID := aModel(rows, "2026-07-16")
	shoot := milestoneType(t, svc, ctx, "shoot", "Photo shoot")
	plan := date.MustParse("2026-05-20")

	err := svc.UpdateType(ctx, shoot.ID, TypeUpdateParams{
		Name: "shoot", Description: "Photo shoot", Active: false,
	})
	if err != nil {
		t.Fatalf("UpdateType: %v", err)
	}

	_, err = svc.AddMilestone(ctx, modelID, shoot.ID, plan)
	rejects(t, err, FieldType, validate.NotAllowed)

	_, err = svc.AddMilestone(ctx, modelID, ids.MustParse("01912345-0000-7000-8000-00000000dddd"), plan)
	rejects(t, err, FieldType, validate.NotAllowed)
}

func TestAddMilestoneNeedsAPlanDate(t *testing.T) {
	svc, rows, _ := newTestService(t)
	ctx := signedIn(t)
	modelID := aModel(rows, "2026-07-16")
	shoot := milestoneType(t, svc, ctx, "shoot", "Photo shoot")

	_, err := svc.AddMilestone(ctx, modelID, shoot.ID, time.Time{})
	rejects(t, err, FieldPlanDate, validate.Required)
}

func TestUpdateMilestoneRecordsOneEntryPerFieldThatMoved(t *testing.T) {
	svc, rows, trail := newTestService(t)
	ctx := signedIn(t)
	one := aMilestone(t, svc, rows, ctx, "2026-05-20")
	trail.entries = nil

	err := svc.UpdateMilestone(ctx, one.ID, MilestoneUpdateParams{
		Plan:   date.MustParse("2026-06-01"),
		Fact:   date.MustParse("2026-06-02"),
		Note:   "the sample was late",
		Active: true,
	})
	if err != nil {
		t.Fatalf("UpdateMilestone: %v", err)
	}

	want := []string{audit.ActionChanged, audit.ActionFactStamped, audit.ActionChanged}
	if got := trail.actions(); !slices.Equal(got, want) {
		t.Fatalf("trail actions = %v, want %v", got, want)
	}
	fields := []string{FieldPlanDate, FieldFactDate, FieldNote}
	for i, entry := range trail.entries {
		if entry.Entity != audit.EntityMilestone || entry.EntityID != one.ID {
			t.Errorf("entry %d = %s %s, want %s %s",
				i, entry.Entity, entry.EntityID, audit.EntityMilestone, one.ID)
		}
		if entry.FieldKey != fields[i] {
			t.Errorf("entry %d field = %q, want %q", i, entry.FieldKey, fields[i])
		}
	}
	if got := trail.entries[0]; got.Old != "2026-05-20" || got.New != "2026-06-01" {
		t.Errorf("the plan moved %q → %q, want %q → %q", got.Old, got.New, "2026-05-20", "2026-06-01")
	}
}

func TestUpdateMilestoneNeverMovesTheBaseline(t *testing.T) {
	svc, rows, _ := newTestService(t)
	ctx := signedIn(t)
	one := aMilestone(t, svc, rows, ctx, "2026-05-20")

	err := svc.UpdateMilestone(ctx, one.ID, MilestoneUpdateParams{
		Plan: date.MustParse("2026-06-01"), Active: true,
	})
	if err != nil {
		t.Fatalf("UpdateMilestone: %v", err)
	}

	calendar, err := svc.Calendar(ctx, one.ModelID)
	if err != nil {
		t.Fatalf("Calendar: %v", err)
	}
	if got := calendar[0]; !date.Equal(got.Baseline, one.Baseline) {
		t.Errorf("baseline = %s, want %s", date.ISO(got.Baseline), date.ISO(one.Baseline))
	}
	if got := calendar[0].Slip(); got != 12 {
		t.Errorf("slip = %d, want 12", got)
	}
}

func TestUpdateMilestoneStampsAndClearsAFactDate(t *testing.T) {
	svc, rows, trail := newTestService(t)
	ctx := signedIn(t)
	one := aMilestone(t, svc, rows, ctx, "2026-05-20")
	trail.entries = nil

	stamped := MilestoneUpdateParams{Plan: one.Plan, Fact: date.MustParse("2026-05-18"), Active: true}
	if err := svc.UpdateMilestone(ctx, one.ID, stamped); err != nil {
		t.Fatalf("UpdateMilestone: %v", err)
	}
	if got := trail.only(t); got.Action != audit.ActionFactStamped || got.New != "2026-05-18" {
		t.Errorf("trail entry = %s %q, want %s %q",
			got.Action, got.New, audit.ActionFactStamped, "2026-05-18")
	}

	trail.entries = nil
	cleared := MilestoneUpdateParams{Plan: one.Plan, Active: true}
	if err := svc.UpdateMilestone(ctx, one.ID, cleared); err != nil {
		t.Fatalf("UpdateMilestone: %v", err)
	}
	if got := trail.only(t); got.Action != audit.ActionFactCleared || got.Old != "2026-05-18" || got.New != "" {
		t.Errorf("trail entry = %s %q → %q, want %s %q → %q",
			got.Action, got.Old, got.New, audit.ActionFactCleared, "2026-05-18", "")
	}
}

func TestUpdateMilestoneRefusesAFactDateInTheFuture(t *testing.T) {
	svc, rows, _ := newTestService(t)
	ctx := signedIn(t)
	one := aMilestone(t, svc, rows, ctx, "2026-05-20")

	today := date.Of(testNow.Date())
	err := svc.UpdateMilestone(ctx, one.ID, MilestoneUpdateParams{
		Plan: one.Plan, Fact: date.AddDays(today, 1), Active: true,
	})
	rejects(t, err, FieldFactDate, validate.NotAllowed)

	// Today itself is the day the app offers.
	err = svc.UpdateMilestone(ctx, one.ID, MilestoneUpdateParams{
		Plan: one.Plan, Fact: today, Active: true,
	})
	if err != nil {
		t.Errorf("UpdateMilestone: %v", err)
	}
}

func TestUpdateMilestoneRefusesANoteThatIsTooLong(t *testing.T) {
	svc, rows, _ := newTestService(t)
	ctx := signedIn(t)
	one := aMilestone(t, svc, rows, ctx, "2026-05-20")

	err := svc.UpdateMilestone(ctx, one.ID, MilestoneUpdateParams{
		Plan: one.Plan, Note: strings.Repeat("я", MaxNoteLen+1), Active: true,
	})
	rejects(t, err, FieldNote, validate.TooLong)
}

func TestUpdateMilestoneNeedsAPlanDate(t *testing.T) {
	svc, rows, _ := newTestService(t)
	ctx := signedIn(t)
	one := aMilestone(t, svc, rows, ctx, "2026-05-20")

	err := svc.UpdateMilestone(ctx, one.ID, MilestoneUpdateParams{Active: true})
	rejects(t, err, FieldPlanDate, validate.Required)
}

func TestUpdateMilestoneWritesNothingWhenNothingMoved(t *testing.T) {
	svc, rows, trail := newTestService(t)
	ctx := signedIn(t)
	one := aMilestone(t, svc, rows, ctx, "2026-05-20")
	trail.entries = nil

	err := svc.UpdateMilestone(ctx, one.ID, MilestoneUpdateParams{Plan: one.Plan, Active: true})
	if err != nil {
		t.Fatalf("UpdateMilestone: %v", err)
	}
	if len(trail.entries) != 0 {
		t.Errorf("the trail holds %+v, want nothing", trail.entries)
	}
}

func TestUpdateMilestoneTakesAStepOffTheCalendarAndBack(t *testing.T) {
	svc, rows, trail := newTestService(t)
	ctx := signedIn(t)
	one := aMilestone(t, svc, rows, ctx, "2026-05-20")
	trail.entries = nil

	off := MilestoneUpdateParams{Plan: one.Plan, Active: false}
	if err := svc.UpdateMilestone(ctx, one.ID, off); err != nil {
		t.Fatalf("UpdateMilestone: %v", err)
	}
	if got := trail.only(t); got.Action != audit.ActionDeactivated {
		t.Errorf("trail action = %q, want %q", got.Action, audit.ActionDeactivated)
	}

	trail.entries = nil
	on := MilestoneUpdateParams{Plan: one.Plan, Active: true}
	if err := svc.UpdateMilestone(ctx, one.ID, on); err != nil {
		t.Fatalf("UpdateMilestone: %v", err)
	}
	if got := trail.only(t); got.Action != audit.ActionReactivated {
		t.Errorf("trail action = %q, want %q", got.Action, audit.ActionReactivated)
	}
}

func TestUpdateMilestoneReportsAMilestoneThatIsNotThere(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := signedIn(t)

	err := svc.UpdateMilestone(ctx, ids.MustParse("01912345-0000-7000-8000-00000000cccc"),
		MilestoneUpdateParams{Plan: date.MustParse("2026-05-20"), Active: true})
	if !errors.Is(err, ErrNoMilestone) {
		t.Errorf("UpdateMilestone error = %v, want %v", err, ErrNoMilestone)
	}
}

func TestCalendarReadsEveryRowAgainstToday(t *testing.T) {
	svc, rows, _ := newTestService(t)
	ctx := signedIn(t)
	modelID := aModel(rows, "2026-07-16")
	today := date.Of(testNow.Date())

	testData := []struct {
		name string
		plan time.Time
		want State
	}{
		{"late", date.AddDays(today, -1), StateLate},
		{"due", date.AddDays(today, 3), StateDue},
		{"planned", date.AddDays(today, 30), StatePlanned},
	}
	for i, tc := range testData {
		step := milestoneType(t, svc, ctx, tc.name, "Step "+strconv.Itoa(i))
		if _, err := svc.AddMilestone(ctx, modelID, step.ID, tc.plan); err != nil {
			t.Fatalf("AddMilestone %q: %v", tc.name, err)
		}
	}

	calendar, err := svc.Calendar(ctx, modelID)
	if err != nil {
		t.Fatalf("Calendar: %v", err)
	}
	if len(calendar) != len(testData) {
		t.Fatalf("Calendar holds %d rows, want %d", len(calendar), len(testData))
	}
	for i, row := range calendar {
		if row.State != testData[i].want {
			t.Errorf("row %d state = %q, want %q", i, row.State, testData[i].want)
		}
	}
}

// aMilestone writes one model with one step on it, and returns the step.
func aMilestone(t *testing.T, svc *Service, rows *store, ctx context.Context, plan string) Milestone {
	t.Helper()

	modelID := aModel(rows, "2026-07-16")
	step := milestoneType(t, svc, ctx, "shoot", "Photo shoot")
	written, err := svc.AddMilestone(ctx, modelID, step.ID, date.MustParse(plan))
	if err != nil {
		t.Fatalf("AddMilestone: %v", err)
	}
	return written
}
