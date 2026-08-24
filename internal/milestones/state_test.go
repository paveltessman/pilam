package milestones

import (
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/platform/date"
)

var today = date.Of(2026, time.March, 10)

// stepOn is a milestone with the plan date given and no fact date.
func stepOn(plan time.Time) Milestone {
	return Milestone{Baseline: plan, Plan: plan, Active: true}
}

func TestStateReadsTheMilestoneAgainstToday(t *testing.T) {
	testData := map[string]struct {
		milestone Milestone
		want      State
	}{
		"a fact date is done":                    {done(today, date.AddDays(today, -30)), StateDone},
		"a fact date is done however late it is": {done(date.AddDays(today, -60), date.AddDays(today, -1)), StateDone},
		"yesterday is late":                      {stepOn(date.AddDays(today, -1)), StateLate},
		"today is due":                           {stepOn(today), StateDue},
		"the last day of the window is due":      {stepOn(date.AddDays(today, DueWindow)), StateDue},
		"the day after the window is planned":    {stepOn(date.AddDays(today, DueWindow+1)), StatePlanned},
	}

	for name, tc := range testData {
		t.Run(name, func(t *testing.T) {
			if got := tc.milestone.State(today); got != tc.want {
				t.Errorf("State = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStateReadsATimestampByTheDayItFallsOn(t *testing.T) {
	// A store hands a date back with a zone on it. The state answers about the
	// day, not about the instant.
	plan := time.Date(2026, time.March, 10, 23, 0, 0, 0, time.FixedZone("UTC+3", 3*60*60))

	if got := (Milestone{Plan: plan}).State(today); got != StateDue {
		t.Errorf("State = %q, want %q", got, StateDue)
	}
}

func TestSlipIsThePlanDateAgainstTheBaseline(t *testing.T) {
	baseline := date.Of(2026, time.March, 1)

	testData := map[string]struct {
		plan time.Time
		want int
	}{
		"a plan that moved later":   {date.AddDays(baseline, 12), 12},
		"a plan that moved earlier": {date.AddDays(baseline, -3), -3},
		"a plan that never moved":   {baseline, 0},
	}

	for name, tc := range testData {
		t.Run(name, func(t *testing.T) {
			milestone := Milestone{Baseline: baseline, Plan: tc.plan}
			if got := milestone.Slip(); got != tc.want {
				t.Errorf("Slip = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestFactSlipIsTheSlipThatReallyHappened(t *testing.T) {
	baseline := date.Of(2026, time.March, 1)

	milestone := done(baseline, date.AddDays(baseline, 5))
	milestone.Plan = date.AddDays(baseline, 20)
	if got := milestone.FactSlip(); got != 5 {
		t.Errorf("FactSlip = %d, want 5: the fact is what happened, the plan is not", got)
	}

	if got := stepOn(baseline).FactSlip(); got != 0 {
		t.Errorf("FactSlip = %d, want 0 for a step that holds no fact date", got)
	}
}

func TestOnTimeIsAFactDateNoLaterThanTheBaseline(t *testing.T) {
	baseline := date.Of(2026, time.March, 1)

	testData := map[string]struct {
		milestone Milestone
		want      bool
	}{
		"a fact before the baseline": {done(baseline, date.AddDays(baseline, -1)), true},
		"a fact on the baseline":     {done(baseline, baseline), true},
		"a fact after the baseline":  {done(baseline, date.AddDays(baseline, 1)), false},
		"no fact at all":             {stepOn(baseline), false},
	}

	for name, tc := range testData {
		t.Run(name, func(t *testing.T) {
			if got := tc.milestone.OnTime(); got != tc.want {
				t.Errorf("OnTime = %t, want %t", got, tc.want)
			}
		})
	}
}

// done is a milestone that holds a fact date.
func done(baseline, fact time.Time) Milestone {
	return Milestone{Baseline: baseline, Plan: baseline, Fact: fact, Active: true}
}
