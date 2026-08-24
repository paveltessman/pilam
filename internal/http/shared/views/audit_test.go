package views

import (
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/milestones"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/labels"
)

var stepID = ids.MustParse("01912345-6789-7abc-def0-1234567890b1")

// milestoneEntry is one entry about a milestone of a model.
func milestoneEntry(action, field, old, next string) audit.Entry {
	return audit.Entry{
		Entity:   audit.EntityMilestone,
		EntityID: stepID,
		Action:   action,
		FieldKey: field,
		Old:      old,
		New:      next,
	}
}

func TestWhatReadsAChangeOfOneMilestone(t *testing.T) {
	testData := map[string]struct {
		entry audit.Entry
		want  string
	}{
		"the step joined the calendar": {
			milestoneEntry(audit.ActionCreated, milestones.FieldPlanDate, "", "2026-05-20"),
			labels.MilestonesAuditCreated + ": 20.05.2026",
		},
		"the plan moved": {
			milestoneEntry(audit.ActionChanged, milestones.FieldPlanDate, "2026-05-20", "2026-06-01"),
			labels.MilestonesAuditPlan + ": 20.05.2026 → 01.06.2026",
		},
		"the fact was stamped": {
			milestoneEntry(audit.ActionFactStamped, milestones.FieldFactDate, "", "2026-05-18"),
			labels.MilestonesAuditFact + ": 18.05.2026",
		},
		"the fact was cleared": {
			milestoneEntry(audit.ActionFactCleared, milestones.FieldFactDate, "2026-05-18", ""),
			labels.MilestonesAuditFactCleared + ": 18.05.2026",
		},
		"the fact moved": {
			milestoneEntry(audit.ActionChanged, milestones.FieldFactDate, "2026-05-18", "2026-05-19"),
			labels.MilestonesAuditFactMoved + ": 18.05.2026 → 19.05.2026",
		},
		"the note changed": {
			milestoneEntry(audit.ActionChanged, milestones.FieldNote, "", "ткань приехала поздно"),
			labels.MilestonesAuditNote + ": ткань приехала поздно",
		},
		"the step left the calendar": {
			milestoneEntry(audit.ActionDeactivated, milestones.FieldActive, "true", "false"),
			labels.AuditOff,
		},
		"the step came back": {
			milestoneEntry(audit.ActionReactivated, milestones.FieldActive, "false", "true"),
			labels.AuditOn,
		},
	}

	for name, tc := range testData {
		t.Run(name, func(t *testing.T) {
			if got := (AuditEvent{Entry: tc.entry}).What(); got != tc.want {
				t.Errorf("What = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWhatNamesTheStepAnEntryBelongsTo(t *testing.T) {
	event := AuditEvent{
		Entry:   milestoneEntry(audit.ActionChanged, milestones.FieldPlanDate, "2026-05-20", "2026-06-01"),
		Subject: "Отшив",
	}

	want := "Отшив — " + labels.MilestonesAuditPlan + ": 20.05.2026 → 01.06.2026"
	if got := event.What(); got != want {
		t.Errorf("What = %q, want %q", got, want)
	}
}

func TestWhatShortensALongNote(t *testing.T) {
	note := strings.Repeat("я", noteInTrail+40)
	event := AuditEvent{Entry: milestoneEntry(audit.ActionChanged, milestones.FieldNote, "", note)}

	got := event.What()
	if !strings.HasSuffix(got, "…") {
		t.Errorf("What = %q, want a shortened note", got)
	}
	if want := labels.MilestonesAuditNote + ": " + strings.Repeat("я", noteInTrail) + "…"; got != want {
		t.Errorf("What = %q, want %q", got, want)
	}
}

func TestWhatReadsTheCalendarOfAModel(t *testing.T) {
	entry := audit.Entry{
		Entity:   audit.EntityModel,
		Action:   audit.ActionTemplateApplied,
		FieldKey: milestones.FieldTemplate,
		New:      "Импорт, ж/д (15)",
	}

	want := labels.ModelsAuditTemplate + ": Импорт, ж/д (15)"
	if got := (AuditEvent{Entry: entry}).What(); got != want {
		t.Errorf("What = %q, want %q", got, want)
	}
}
