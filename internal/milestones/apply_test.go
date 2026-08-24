package milestones

import (
	"slices"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/platform/date"
	"github.com/paveltessman/pilam/internal/platform/ids"
)

var chain = []TemplateItem{
	{TypeID: ids.MustParse("01912345-0000-7000-8000-000000000001"), Offset: -270},
	{TypeID: ids.MustParse("01912345-0000-7000-8000-000000000002"), Offset: -240},
	{TypeID: ids.MustParse("01912345-0000-7000-8000-000000000003"), Offset: -88},
	{TypeID: ids.MustParse("01912345-0000-7000-8000-000000000004"), Offset: -88},
	{TypeID: ids.MustParse("01912345-0000-7000-8000-000000000005"), Offset: -14},
}

func TestApplyDatesEveryItemFromTheTargetDate(t *testing.T) {
	target := date.Of(2026, time.July, 16)

	got := Apply(chain, target)

	if len(got) != len(chain) {
		t.Fatalf("Apply returned %d milestones, want %d", len(got), len(chain))
	}
	want := []string{"2025-10-19", "2025-11-18", "2026-04-19", "2026-04-19", "2026-07-02"}
	for i, planned := range got {
		if date.ISO(planned.Plan) != want[i] {
			t.Errorf("item %d plan = %s, want %s", i, date.ISO(planned.Plan), want[i])
		}
		if !date.Equal(planned.Baseline, planned.Plan) {
			t.Errorf("item %d baseline = %s, plan = %s: a calendar starts with the two equal",
				i, date.ISO(planned.Baseline), date.ISO(planned.Plan))
		}
		if planned.TypeID != chain[i].TypeID {
			t.Errorf("item %d type = %s, want %s", i, planned.TypeID, chain[i].TypeID)
		}
	}
}

func TestApplyHoldsNoMilestoneForAnEmptyTemplate(t *testing.T) {
	if got := Apply(nil, date.Of(2026, time.July, 16)); len(got) != 0 {
		t.Errorf("Apply = %+v, want none", got)
	}
}

func TestGapsAreTheDaysToTheItemAbove(t *testing.T) {
	got := Gaps(chain)

	// The first item has no item above it, and two items on the same day are a
	// gap of zero.
	want := []int{0, 30, 152, 0, 74}
	if !slices.Equal(got, want) {
		t.Errorf("Gaps = %v, want %v", got, want)
	}
}
