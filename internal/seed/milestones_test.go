package seed

import (
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/milestones"
	"github.com/paveltessman/pilam/internal/platform/date"
	"github.com/paveltessman/pilam/internal/platform/ids"
)

// What the critical path of the demo brand holds, from the design.
const (
	wantSteps       = 15
	wantFirstOffset = -270
	wantLastOffset  = -7
)

func TestCriticalPathIsWellFormed(t *testing.T) {
	names := make(map[string]bool, len(criticalPath))
	descriptions := make(map[string]bool, len(criticalPath))

	for i, one := range criticalPath {
		switch {
		case one.name == "":
			t.Errorf("Step %d carries no short name", i)
		case names[one.name]:
			t.Errorf("Two steps hold the short name %q", one.name)
		case utf8.RuneCountInString(one.name) > milestones.MaxNameLen:
			t.Errorf("The short name %q is longer than %d characters", one.name, milestones.MaxNameLen)
		}
		names[one.name] = true

		switch {
		case one.description == "":
			t.Errorf("Step %q carries no description", one.name)
		case descriptions[one.description]:
			t.Errorf("Two steps hold the description %q", one.description)
		case utf8.RuneCountInString(one.description) > milestones.MaxDescriptionLen:
			t.Errorf("The description of %q is longer than %d characters",
				one.name, milestones.MaxDescriptionLen)
		}
		descriptions[one.description] = true

		if one.offset > milestones.MaxOffset || one.offset < milestones.MinOffset {
			t.Errorf("Step %q runs at %d days, outside the bound the service holds", one.name, one.offset)
		}
		// The path runs forwards: a step never starts before the step above it.
		if i > 0 && one.offset < criticalPath[i-1].offset {
			t.Errorf("Step %q runs at %d days, before the %d days of the step above it",
				one.name, one.offset, criticalPath[i-1].offset)
		}
	}

	switch {
	case len(criticalPath) != wantSteps:
		t.Errorf("The path holds %d steps, want %d", len(criticalPath), wantSteps)
	case criticalPath[0].offset != wantFirstOffset:
		t.Errorf("The path opens at %d days, want %d", criticalPath[0].offset, wantFirstOffset)
	case criticalPath[len(criticalPath)-1].offset != wantLastOffset:
		t.Errorf("The path closes at %d days, want %d",
			criticalPath[len(criticalPath)-1].offset, wantLastOffset)
	}
}

func TestRunWritesTheCriticalPath(t *testing.T) {
	deps, _, _, _ := newTestDeps(t)

	report := run(t, deps, Options{Users: short, Models: 1})

	got := report.Milestones
	switch {
	case got.Types.Created != wantSteps || got.Types.Skipped != 0:
		t.Errorf("Incorrect type count: %+v", got.Types)
	case got.Templates.Created != 1 || got.Templates.Skipped != 0:
		t.Errorf("Incorrect template count: %+v", got.Templates)
	case got.Steps.Created != wantSteps || got.Steps.Skipped != 0:
		t.Errorf("Incorrect step count: %+v", got.Steps)
	}

	names := typeNames(t, deps)
	if len(names) != wantSteps {
		t.Fatalf("The store holds %d types, want %d", len(names), wantSteps)
	}

	// The items carry the path in path order, with the offsets of the design.
	items := templateItems(t, deps)
	if len(items) != wantSteps {
		t.Fatalf("The template holds %d items, want %d", len(items), wantSteps)
	}
	for i, item := range items {
		want := criticalPath[i]
		if names[item.TypeID] != want.name {
			t.Errorf("Item %d runs %q, want %q", i, names[item.TypeID], want.name)
		}
		if item.Offset != want.offset {
			t.Errorf("Item %q runs at %d days, want %d", want.name, item.Offset, want.offset)
		}
		if item.Position != i {
			t.Errorf("Item %q sits at position %d, want %d", want.name, item.Position, i)
		}
	}
}

func TestSeededTypesAndTemplateAreActive(t *testing.T) {
	deps, _, _, _ := newTestDeps(t)

	run(t, deps, Options{Users: short, Models: 1})

	types, err := deps.Milestones.ListTypes(t.Context())
	if err != nil {
		t.Fatalf("ListTypes: %v", err)
	}
	for _, one := range types {
		if !one.Active {
			t.Errorf("Type %q is seeded retired", one.Name)
		}
	}

	template := seededTemplate(t, deps)
	switch {
	case !template.Active:
		t.Error("The seeded template is retired")
	case !template.Default:
		t.Error("The seeded template is not the one a new model starts with")
	case template.Description == "":
		t.Error("The seeded template carries no description")
	}
}

func TestSecondRunLeavesTheCriticalPathAsItStands(t *testing.T) {
	deps, _, _, _ := newTestDeps(t)

	run(t, deps, Options{Users: short, Models: 1})
	first := seededTemplate(t, deps)
	firstItems := templateItems(t, deps)

	report := run(t, deps, Options{Users: short, Models: 1})

	got := report.Milestones
	switch {
	case got.Types.Created != 0 || got.Types.Skipped != wantSteps:
		t.Errorf("The second run wrote the types again: %+v", got.Types)
	case got.Templates.Created != 0 || got.Templates.Skipped != 1:
		t.Errorf("The second run wrote the template again: %+v", got.Templates)
	case got.Steps.Created != 0 || got.Steps.Skipped != wantSteps:
		t.Errorf("The second run wrote the steps again: %+v", got.Steps)
	}

	if second := seededTemplate(t, deps); second.ID != first.ID {
		t.Errorf("The second run moved the template: want=%s, got=%s", first.ID, second.ID)
	}
	secondItems := templateItems(t, deps)
	if len(secondItems) != len(firstItems) {
		t.Fatalf("The second run left %d items, want %d", len(secondItems), len(firstItems))
	}
	for i, item := range secondItems {
		if item.ID != firstItems[i].ID || item.Offset != firstItems[i].Offset {
			t.Errorf("The second run moved item %d: want=%s (%d), got=%s (%d)",
				i, firstItems[i].ID, firstItems[i].Offset, item.ID, item.Offset)
		}
	}
}

func TestEverySeededMilestoneWriteLeavesTrailEntry(t *testing.T) {
	deps, users, _, recorder := newTestDeps(t)

	run(t, deps, Options{Users: short, Models: 1})
	root := storedUser(t, users, roster[rootAt].email(DefaultDomain))
	template := seededTemplate(t, deps)

	counts := make(map[string]int)
	for _, entry := range recorder.entries {
		switch entry.Entity {
		case audit.EntityMilestoneType:
			counts[entry.Entity+" "+entry.Action]++
		case audit.EntityMilestoneTemplate:
			if entry.EntityID != template.ID {
				t.Errorf("A %s entry names %s, want the seeded template %s",
					entry.Action, entry.EntityID, template.ID)
			}
			counts[entry.Entity+" "+entry.Action]++
		default:
			continue
		}
		if entry.ActorID != root.ID {
			t.Errorf("A %s entry names the actor %s, want the root %s",
				entry.Entity, entry.ActorID, root.ID)
		}
	}

	want := map[string]int{
		audit.EntityMilestoneType + " " + audit.ActionCreated:       wantSteps,
		audit.EntityMilestoneTemplate + " " + audit.ActionCreated:   1,
		audit.EntityMilestoneTemplate + " " + audit.ActionItemAdded: wantSteps,
	}
	for key, n := range want {
		if counts[key] != n {
			t.Errorf("The trail holds %d %q entries, want %d", counts[key], key, n)
		}
	}
}

// seededTemplate returns the one template the run wrote.
func seededTemplate(t *testing.T, deps Deps) milestones.Template {
	t.Helper()

	held, err := deps.Milestones.ListTemplates(t.Context())
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if len(held) != 1 {
		t.Fatalf("The store holds %d templates, want 1", len(held))
	}
	if held[0].Name != templateName {
		t.Fatalf("The seeded template is named %q, want %q", held[0].Name, templateName)
	}
	return held[0]
}

// templateItems returns the items of the seeded template, in the order the
// editor shows them.
func templateItems(t *testing.T, deps Deps) []milestones.TemplateItem {
	t.Helper()

	items, err := deps.Milestones.TemplateItems(t.Context(), seededTemplate(t, deps).ID)
	if err != nil {
		t.Fatalf("TemplateItems: %v", err)
	}
	return items
}

// typeNames is the short name of every seeded type, keyed by the type.
func typeNames(t *testing.T, deps Deps) map[ids.ID]string {
	t.Helper()

	types, err := deps.Milestones.ListTypes(t.Context())
	if err != nil {
		t.Fatalf("ListTypes: %v", err)
	}
	names := make(map[ids.ID]string, len(types))
	for _, one := range types {
		names[one.ID] = one.Name
	}
	return names
}

// ---------------------------------------------------------------- calendars

// How many models the season leaves late, from the design. The drops go on sale
// 180, 240, and 300 days out, and the path opens 270 days before a drop, so only
// the first two drops hold a step whose plan date has passed.
const (
	minLateModels = 8
	maxLateModels = 12
)

func TestSeededCalendarsRunFromTheTargetDateOfTheDrop(t *testing.T) {
	deps, _, _, _ := newTestDeps(t)

	report := run(t, deps, Options{Users: short, Models: 3})

	if got := report.Milestones.Calendars; got.Created != 3 || got.Skipped != 0 {
		t.Errorf("Incorrect calendar count: %+v", got)
	}

	offsets := offsetsByType(t, deps)
	eachCalendar(t, deps, func(drop catalog.Drop, model catalog.Model, calendar []milestones.CalendarRow) {
		if len(calendar) != wantSteps {
			t.Fatalf("Model %s holds %d milestones, want %d", model.Article, len(calendar), wantSteps)
		}
		for _, row := range calendar {
			// The baseline is what the calendar promised, and nothing the seed
			// writes afterwards moves it.
			want := date.AddDays(drop.TargetDate, offsets[row.TypeID])
			if !date.Equal(row.Baseline, want) {
				t.Errorf("Model %s promises %s for step %s, want %s",
					model.Article, date.ISO(row.Baseline), row.TypeID, date.ISO(want))
			}
			if !row.Active {
				t.Errorf("Model %s holds a retired step %s", model.Article, row.TypeID)
			}
		}
	})
}

func TestSeededSeasonLeavesModelsLate(t *testing.T) {
	deps, _, _, _ := newTestDeps(t)

	run(t, deps, Options{Users: short})

	perDrop := make(map[string]int)
	eachCalendar(t, deps, func(drop catalog.Drop, _ catalog.Model, calendar []milestones.CalendarRow) {
		for _, row := range calendar {
			if row.State == milestones.StateLate {
				perDrop[drop.Name]++
				return
			}
		}
	})

	total := 0
	for _, n := range perDrop {
		total += n
	}
	if total < minLateModels || total > maxLateModels {
		t.Errorf("The season leaves %d models late, want between %d and %d",
			total, minLateModels, maxLateModels)
	}

	// The spread is uneven: the drop that goes on sale first has run through
	// more of its path, and the last drop has not started.
	first, second, last := plan[0].name, plan[1].name, plan[len(plan)-1].name
	switch {
	case perDrop[first] <= perDrop[second]:
		t.Errorf("%s leaves %d models late and %s leaves %d, want the spread uneven",
			first, perDrop[first], second, perDrop[second])
	case perDrop[last] != 0:
		t.Errorf("%s leaves %d models late, and nothing of it has come due yet",
			last, perDrop[last])
	}
}

func TestSeededFactsHoldWhatTheSeasonAlreadyRan(t *testing.T) {
	deps, _, _, _ := newTestDeps(t)

	report := run(t, deps, Options{Users: short})

	stamped := 0
	eachCalendar(t, deps, func(drop catalog.Drop, model catalog.Model, calendar []milestones.CalendarRow) {
		for _, row := range calendar {
			if !row.Done() {
				// A step nobody stamped carries no note and no move.
				if !date.Equal(row.Plan, row.Baseline) {
					t.Errorf("Model %s moved a step it never ran: %s", model.Article, date.ISO(row.Plan))
				}
				continue
			}
			stamped++

			switch {
			case date.After(row.Fact, today):
				t.Errorf("Model %s stamped %s, which is later than today", model.Article, date.ISO(row.Fact))
			case !date.Before(row.Baseline, today):
				t.Errorf("Model %s stamped the step it promised for %s, which has not come due",
					model.Article, date.ISO(row.Baseline))
			}
		}
	})

	if stamped != report.Milestones.Facts {
		t.Errorf("The season holds %d stamped steps, the report names %d", stamped, report.Milestones.Facts)
	}
	if stamped == 0 {
		t.Error("The season holds no step that already ran")
	}
}

func TestAMovedPlanDateCarriesTheFactAndTheReason(t *testing.T) {
	deps, _, _, _ := newTestDeps(t)

	run(t, deps, Options{Users: short, Models: 24})

	moved := 0
	eachCalendar(t, deps, func(_ catalog.Drop, model catalog.Model, calendar []milestones.CalendarRow) {
		for _, row := range calendar {
			if date.Equal(row.Plan, row.Baseline) {
				if row.Note != "" {
					t.Errorf("Model %s explains a step that never moved: %q", model.Article, row.Note)
				}
				continue
			}
			moved++

			switch {
			case !date.Equal(row.Plan, row.Fact):
				t.Errorf("Model %s expects %s and stamped %s, want the plan on the day it ran",
					model.Article, date.ISO(row.Plan), date.ISO(row.Fact))
			case row.Slip() <= 0:
				t.Errorf("Model %s moved a step %d days, want a move later than the promise",
					model.Article, row.Slip())
			case row.Note == "":
				t.Errorf("Model %s moved a step and wrote no reason", model.Article)
			}
		}
	})

	if moved == 0 {
		t.Error("No seeded step ran later than the day the calendar promised")
	}
}

func TestSecondRunLeavesTheCalendarsAsItStands(t *testing.T) {
	deps, _, _, _ := newTestDeps(t)

	run(t, deps, Options{Users: short, Models: 12})
	first := calendarRows(t, deps)

	report := run(t, deps, Options{Users: short, Models: 12})

	got := report.Milestones
	switch {
	case got.Calendars.Created != 0 || got.Calendars.Skipped != 12:
		t.Errorf("The second run built the calendars again: %+v", got.Calendars)
	case got.Facts != 0:
		t.Errorf("The second run stamped %d steps again", got.Facts)
	}

	if second := calendarRows(t, deps); !slices.Equal(first, second) {
		t.Errorf("The second run moved the calendars of the first:\n%v\n%v", first, second)
	}
}

func TestEverySeededCalendarWriteLeavesTrailEntry(t *testing.T) {
	deps, users, _, recorder := newTestDeps(t)

	report := run(t, deps, Options{Users: short, Models: 12})
	root := storedUser(t, users, roster[rootAt].email(DefaultDomain))

	counts := make(map[string]int)
	for _, entry := range recorder.entries {
		if entry.Entity != audit.EntityMilestone && entry.Action != audit.ActionTemplateApplied {
			continue
		}
		if entry.ActorID != root.ID {
			t.Errorf("A %s entry names the actor %s, want the root %s",
				entry.Action, entry.ActorID, root.ID)
		}
		counts[entry.Action]++
	}

	want := map[string]int{
		audit.ActionTemplateApplied: report.Milestones.Calendars.Created,
		audit.ActionFactStamped:     report.Milestones.Facts,
	}
	for action, n := range want {
		if counts[action] != n {
			t.Errorf("The trail holds %d %q entries, want %d", counts[action], action, n)
		}
	}
	if counts[audit.ActionChanged] == 0 {
		t.Error("The trail holds no moved plan date")
	}
}

// eachCalendar walks the calendar of every seeded model, drop by drop, in the
// order the run wrote them.
func eachCalendar(
	t *testing.T,
	deps Deps,
	fn func(drop catalog.Drop, model catalog.Model, calendar []milestones.CalendarRow),
) {
	t.Helper()

	seasons, err := deps.Catalog.ListSeasons(t.Context())
	if err != nil {
		t.Fatalf("ListSeasons: %v", err)
	}
	for _, season := range seasons {
		drops, err := deps.Catalog.ListDrops(t.Context(), season.ID)
		if err != nil {
			t.Fatalf("ListDrops: %v", err)
		}
		for _, drop := range drops {
			models, err := deps.Catalog.ListModels(t.Context(), catalog.ModelListParams{DropID: drop.ID})
			if err != nil {
				t.Fatalf("ListModels: %v", err)
			}
			for _, model := range models {
				calendar, err := deps.Milestones.Calendar(t.Context(), model.ID)
				if err != nil {
					t.Fatalf("Calendar: %v", err)
				}
				fn(drop, model, calendar)
			}
		}
	}
}

// calendarRows is every seeded milestone, as one sorted line per row.
func calendarRows(t *testing.T, deps Deps) []string {
	t.Helper()

	var out []string
	eachCalendar(t, deps, func(_ catalog.Drop, model catalog.Model, calendar []milestones.CalendarRow) {
		for _, row := range calendar {
			out = append(out, strings.Join([]string{
				model.Article, row.ID.String(), row.TypeID.String(),
				date.ISO(row.Baseline), date.ISO(row.Plan), isoOrNone(row.Fact), row.Note,
			}, " "))
		}
	})
	slices.Sort(out)
	return out
}

// isoOrNone renders a date, and a dash for a date that is not there.
func isoOrNone(d time.Time) string {
	if d.IsZero() {
		return "-"
	}
	return date.ISO(d)
}

// offsetsByType is the offset every seeded type runs at, keyed by the type.
func offsetsByType(t *testing.T, deps Deps) map[ids.ID]int {
	t.Helper()

	offsets := make(map[ids.ID]int, len(criticalPath))
	for id, name := range typeNames(t, deps) {
		at := indexOfStep(name)
		if at < 0 {
			t.Fatalf("The store holds the type %q, which the path does not name", name)
		}
		offsets[id] = criticalPath[at].offset
	}
	return offsets
}

// indexOfStep is where the short name sits in the path, or -1 when the path
// does not hold it.
func indexOfStep(name string) int {
	for i, one := range criticalPath {
		if one.name == name {
			return i
		}
	}
	return -1
}
