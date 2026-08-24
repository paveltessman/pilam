package postgres

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/milestones"
	"github.com/paveltessman/pilam/internal/platform/date"
)

func TestMilestonesRoundTripEveryField(t *testing.T) {
	t.Parallel()

	store := milestoneDB(t)
	model := store.model(t, "ART-1", "2026-07-16")
	fit := store.milestoneType(t, "fit", "Fit approved")

	want := milestones.Milestone{
		ID:       gen.New(),
		ModelID:  model.ID,
		TypeID:   fit.ID,
		Baseline: date.MustParse("2026-02-11"),
		Plan:     date.MustParse("2026-02-20"),
		Fact:     date.MustParse("2026-02-18"),
		Note:     "the fabric arrived late",
		Active:   true,
	}
	if err := store.milestones.Create(t.Context(), want); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := store.milestones.ByID(t.Context(), want.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}

	if got != want {
		t.Errorf("ByID = %+v, want %+v", got, want)
	}
}

func TestMilestonesHoldNoFactDateUntilOneIsStamped(t *testing.T) {
	t.Parallel()

	store := milestoneDB(t)
	model := store.model(t, "ART-1", "2026-07-16")
	fit := store.milestoneType(t, "fit", "Fit approved")

	created := store.milestone(t, model, fit.ID, "2026-02-11")

	got, err := store.milestones.ByID(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if got.Done() {
		t.Errorf("fact = %s, want none", date.ISO(got.Fact))
	}

	got.Fact = date.MustParse("2026-02-18")
	if err := store.milestones.Update(t.Context(), got); err != nil {
		t.Fatalf("Update: %v", err)
	}

	stamped, err := store.milestones.ByID(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if !date.Equal(stamped.Fact, got.Fact) {
		t.Errorf("fact = %s, want %s", date.ISO(stamped.Fact), date.ISO(got.Fact))
	}

	// A fact stamped by mistake can be cleared again.
	stamped.Fact = time.Time{}
	if err := store.milestones.Update(t.Context(), stamped); err != nil {
		t.Fatalf("Update: %v", err)
	}

	cleared, err := store.milestones.ByID(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if cleared.Done() {
		t.Errorf("fact = %s, want none", date.ISO(cleared.Fact))
	}
}

func TestMilestonesReportMissingRowAsErrNoMilestone(t *testing.T) {
	t.Parallel()

	store := milestoneDB(t)
	model := store.model(t, "ART-1", "2026-07-16")
	fit := store.milestoneType(t, "fit", "Fit approved")

	if _, err := store.milestones.ByID(t.Context(), gen.New()); !errors.Is(err, milestones.ErrNoMilestone) {
		t.Errorf("ByID error = %v, want %v", err, milestones.ErrNoMilestone)
	}

	missing := milestones.Milestone{
		ID:       gen.New(),
		ModelID:  model.ID,
		TypeID:   fit.ID,
		Baseline: date.MustParse("2026-02-11"),
		Plan:     date.MustParse("2026-02-11"),
		Active:   true,
	}
	if err := store.milestones.Update(t.Context(), missing); !errors.Is(err, milestones.ErrNoMilestone) {
		t.Errorf("Update error = %v, want %v", err, milestones.ErrNoMilestone)
	}
}

func TestMilestonesRefuseATypeTheModelAlreadyHolds(t *testing.T) {
	t.Parallel()

	store := milestoneDB(t)
	model := store.model(t, "ART-1", "2026-07-16")
	fit := store.milestoneType(t, "fit", "Fit approved")
	store.milestone(t, model, fit.ID, "2026-02-11")

	again := milestones.Milestone{
		ID:       gen.New(),
		ModelID:  model.ID,
		TypeID:   fit.ID,
		Baseline: date.MustParse("2026-03-01"),
		Plan:     date.MustParse("2026-03-01"),
		Active:   true,
	}
	if err := store.milestones.Create(t.Context(), again); !errors.Is(err, milestones.ErrTypeOnModel) {
		t.Errorf("Create error = %v, want %v", err, milestones.ErrTypeOnModel)
	}

	// Another model runs the same step.
	other := store.model(t, "ART-2", "2026-07-16")
	store.milestone(t, other, fit.ID, "2026-02-11")
}

func TestMilestonesListOneModelInPlanDateOrder(t *testing.T) {
	t.Parallel()

	store := milestoneDB(t)
	model := store.model(t, "ART-1", "2026-07-16")
	other := store.model(t, "ART-2", "2026-07-16")

	shipped := store.milestoneType(t, "ship", "Shipped")
	fit := store.milestoneType(t, "fit", "Fit approved")
	bulk := store.milestoneType(t, "bulk", "Bulk production started")

	store.milestone(t, model, shipped.ID, "2026-05-03")
	store.milestone(t, model, fit.ID, "2026-02-11")
	store.milestone(t, model, bulk.ID, "2026-02-26")
	store.milestone(t, other, fit.ID, "2026-01-01")

	calendar, err := store.milestones.ByModel(t.Context(), model.ID)
	if err != nil {
		t.Fatalf("ByModel: %v", err)
	}

	got := make([]string, len(calendar))
	for i, one := range calendar {
		got[i] = date.ISO(one.Plan)
	}
	want := []string{"2026-02-11", "2026-02-26", "2026-05-03"}
	if !slices.Equal(got, want) {
		t.Errorf("ByModel = %v, want %v", got, want)
	}
}

func TestMilestonesListAModelThatHoldsNoCalendar(t *testing.T) {
	t.Parallel()

	store := milestoneDB(t)
	model := store.model(t, "ART-1", "2026-07-16")

	calendar, err := store.milestones.ByModel(t.Context(), model.ID)
	if err != nil {
		t.Fatalf("ByModel: %v", err)
	}
	if len(calendar) != 0 {
		t.Errorf("ByModel = %+v, want none", calendar)
	}
}

func TestMilestonesReadTheTargetDateOfTheDropOfAModel(t *testing.T) {
	t.Parallel()

	store := milestoneDB(t)
	model := store.model(t, "ART-1", "2026-07-16")

	target, err := store.milestones.Target(t.Context(), model.ID)
	if err != nil {
		t.Fatalf("Target: %v", err)
	}
	if want := date.MustParse("2026-07-16"); !date.Equal(target, want) {
		t.Errorf("Target = %s, want %s", date.ISO(target), date.ISO(want))
	}

	if _, err := store.milestones.Target(t.Context(), gen.New()); !errors.Is(err, milestones.ErrNoModel) {
		t.Errorf("Target error = %v, want %v", err, milestones.ErrNoModel)
	}
}
