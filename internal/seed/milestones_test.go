package seed

import (
	"testing"
	"unicode/utf8"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/milestones"
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
