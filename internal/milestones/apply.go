package milestones

import (
	"time"

	"github.com/paveltessman/pilam/internal/platform/date"
	"github.com/paveltessman/pilam/internal/platform/ids"
)

// This code provides a preview when the user creates a new critical path.

// Planned is one milestone a template produces from a target date.
type Planned struct {
	TypeID ids.ID

	// Baseline is what the calendar promises, and Plan is what the team expects
	// now. A calendar starts with the two equal.
	Baseline time.Time
	Plan     time.Time
}

// Apply is the rule a calendar is built from: one milestone per template item,
// dated the target date of the drop plus the offset of the item.
//
// The offsets are calendar days.
func Apply(items []TemplateItem, target time.Time) []Planned {
	planned := make([]Planned, len(items))
	for i, item := range items {
		day := date.AddDays(target, item.Offset)
		planned[i] = Planned{TypeID: item.TypeID, Baseline: day, Plan: day}
	}
	return planned
}

// Gaps is the days from each item to the item above it. The first item has no
// item above it, so its gap is zero.
func Gaps(items []TemplateItem) []int {
	gaps := make([]int, len(items))
	for i := 1; i < len(items); i++ {
		gaps[i] = items[i].Offset - items[i-1].Offset
	}
	return gaps
}
