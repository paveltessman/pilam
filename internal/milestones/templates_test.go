package milestones

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

func TestCreateTemplateWritesTheRowAndTheTrail(t *testing.T) {
	svc, _, trail := newTestService(t)
	ctx := signedIn(t)

	created, err := svc.CreateTemplate(ctx, TemplateCreateParams{
		Name:        "  Import, rail  ",
		Description: "  The rail chain  ",
	})
	if err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}

	// The two fields come back trimmed, and a new template is active and is not
	// the default.
	if created.Name != "Import, rail" || created.Description != "The rail chain" {
		t.Errorf("created = %+v, want the two fields trimmed", created)
	}
	if !created.Active || created.Default {
		t.Errorf("created = %+v, want an active template that is not the default", created)
	}

	entry := trail.only(t)
	if entry.Entity != audit.EntityMilestoneTemplate || entry.Action != audit.ActionCreated {
		t.Errorf("entry = %+v, want a created entry on the template", entry)
	}
	if entry.EntityID != created.ID || entry.New != created.Name {
		t.Errorf("entry = %+v, want it to name template %s", entry, created.ID)
	}
}

func TestCreateTemplateChecksTheTwoFields(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := signedIn(t)

	cases := []struct {
		name   string
		params TemplateCreateParams
		field  string
		code   validate.Code
	}{
		{"no name", TemplateCreateParams{Description: "d"}, FieldName, validate.Required},
		{"no description", TemplateCreateParams{Name: "n"}, FieldDescription, validate.Required},
		{
			"a long name",
			TemplateCreateParams{Name: strings.Repeat("a", MaxNameLen+1), Description: "d"},
			FieldName, validate.TooLong,
		},
		{
			"a long description",
			TemplateCreateParams{Name: "n", Description: strings.Repeat("a", MaxDescriptionLen+1)},
			FieldDescription, validate.TooLong,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.CreateTemplate(ctx, c.params)
			rejects(t, err, c.field, c.code)
		})
	}
}

func TestOneTemplateIsTheDefault(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := signedIn(t)

	first, err := svc.CreateTemplate(ctx, TemplateCreateParams{
		Name: "Import, rail", Description: "Rail", Default: true,
	})
	if err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}
	second, err := svc.CreateTemplate(ctx, TemplateCreateParams{
		Name: "Import, air", Description: "Air", Default: true,
	})
	if err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}

	held, err := svc.Template(ctx, first.ID)
	if err != nil {
		t.Fatalf("Template: %v", err)
	}
	if held.Default {
		t.Error("the first template is still the default: naming a second one leaves one default")
	}
	held, err = svc.Template(ctx, second.ID)
	if err != nil {
		t.Fatalf("Template: %v", err)
	}
	if !held.Default {
		t.Error("the second template is not the default")
	}
}

func TestUpdateTemplateRefusesADefaultThatIsRetired(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := signedIn(t)
	template := milestoneTemplate(t, svc, ctx, "Import, rail", "Rail")

	err := svc.UpdateTemplate(ctx, template.ID, TemplateUpdateParams{
		Name: template.Name, Description: template.Description, Default: true, Active: false,
	})
	rejects(t, err, FieldDefault, validate.NotAllowed)
}

func TestUpdateTemplateRecordsOneEntryPerFieldThatMoved(t *testing.T) {
	svc, _, trail := newTestService(t)
	ctx := signedIn(t)
	template := milestoneTemplate(t, svc, ctx, "Import, rail", "Rail")
	trail.entries = nil

	err := svc.UpdateTemplate(ctx, template.ID, TemplateUpdateParams{
		Name: "Import, air", Description: "Air", Default: true, Active: true,
	})
	if err != nil {
		t.Fatalf("UpdateTemplate: %v", err)
	}

	if got, want := len(trail.entries), 3; got != want {
		t.Fatalf("the trail holds %d entries, want %d: %+v", got, want, trail.entries)
	}
	fields := []string{FieldName, FieldDescription, FieldDefault}
	for i, entry := range trail.entries {
		if entry.FieldKey != fields[i] {
			t.Errorf("entry %d names %q, want %q", i, entry.FieldKey, fields[i])
		}
	}
}

func TestUpdateTemplateWritesNothingWhenNothingMoved(t *testing.T) {
	svc, _, trail := newTestService(t)
	ctx := signedIn(t)
	template := milestoneTemplate(t, svc, ctx, "Import, rail", "Rail")
	trail.entries = nil

	err := svc.UpdateTemplate(ctx, template.ID, TemplateUpdateParams{
		Name: template.Name, Description: template.Description, Active: true,
	})
	if err != nil {
		t.Fatalf("UpdateTemplate: %v", err)
	}
	if len(trail.entries) != 0 {
		t.Errorf("the trail holds %+v, want nothing", trail.entries)
	}
}

func TestAddTemplateItemAppendsAndRecordsUnderTheTemplate(t *testing.T) {
	svc, _, trail := newTestService(t)
	ctx := signedIn(t)
	template := milestoneTemplate(t, svc, ctx, "Import, rail", "Rail")
	first := milestoneType(t, svc, ctx, "tech1", "Tech pack for the 1st sample")
	second := milestoneType(t, svc, ctx, "sample1", "1st sample received")
	trail.entries = nil

	one := templateItem(t, svc, ctx, template.ID, first.ID, -270)
	two := templateItem(t, svc, ctx, template.ID, second.ID, -240)

	if one.Position != 0 || two.Position != 1 {
		t.Errorf("positions = %d, %d: want 0, 1", one.Position, two.Position)
	}

	items, err := svc.TemplateItems(ctx, template.ID)
	if err != nil {
		t.Fatalf("TemplateItems: %v", err)
	}
	if want := []int{-270, -240}; !slices.Equal(offsets(items), want) {
		t.Errorf("offsets = %v, want %v", offsets(items), want)
	}

	// The item has no card of its own, so the trail names the template.
	for _, entry := range trail.entries {
		if entry.Entity != audit.EntityMilestoneTemplate || entry.EntityID != template.ID {
			t.Errorf("entry = %+v, want it recorded under template %s", entry, template.ID)
		}
		if entry.Action != audit.ActionItemAdded {
			t.Errorf("entry action = %q, want %q", entry.Action, audit.ActionItemAdded)
		}
	}
	if got, want := trail.entries[0].New, "tech1 (-270)"; got != want {
		t.Errorf("the trail reads %q, want %q", got, want)
	}
}

func TestAddTemplateItemHoldsOneTypePerTemplate(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := signedIn(t)
	template := milestoneTemplate(t, svc, ctx, "Import, rail", "Rail")
	milestoneT := milestoneType(t, svc, ctx, "tech1", "Tech pack for the 1st sample")
	templateItem(t, svc, ctx, template.ID, milestoneT.ID, -270)

	_, err := svc.AddTemplateItem(ctx, template.ID, milestoneT.ID, -240)
	if !errors.Is(err, ErrTypeInTemplate) {
		t.Errorf("AddTemplateItem error = %v, want %v", err, ErrTypeInTemplate)
	}
}

func TestAddTemplateItemChecksTheOffsetAndTheType(t *testing.T) {
	svc, rows, _ := newTestService(t)
	ctx := signedIn(t)
	template := milestoneTemplate(t, svc, ctx, "Import, rail", "Rail")
	milestoneT := milestoneType(t, svc, ctx, "tech1", "Tech pack for the 1st sample")

	// An offset is zero or negative, and it lies within the bound.
	_, err := svc.AddTemplateItem(ctx, template.ID, milestoneT.ID, 1)
	rejects(t, err, FieldOffset, validate.TooBig)

	_, err = svc.AddTemplateItem(ctx, template.ID, milestoneT.ID, MinOffset-1)
	rejects(t, err, FieldOffset, validate.TooSmall)

	// A type that is not there, and a type that is retired, both leave the
	// pickers.
	_, err = svc.AddTemplateItem(ctx, template.ID, ids.MustParse("01912345-0000-7000-8000-00000000ffff"), -10)
	rejects(t, err, FieldType, validate.NotAllowed)

	retired := rows.types[milestoneT.ID]
	retired.Active = false
	rows.types[milestoneT.ID] = retired
	_, err = svc.AddTemplateItem(ctx, template.ID, milestoneT.ID, -10)
	rejects(t, err, FieldType, validate.NotAllowed)
}

func TestSetTemplateItemOffsetMovesOneItem(t *testing.T) {
	svc, _, trail := newTestService(t)
	ctx := signedIn(t)
	template, items := chainOfThree(t, svc, ctx)
	trail.entries = nil

	if err := svc.SetTemplateItemOffset(ctx, template.ID, items[1].ID, -230); err != nil {
		t.Fatalf("SetTemplateItemOffset: %v", err)
	}

	held, err := svc.TemplateItems(ctx, template.ID)
	if err != nil {
		t.Fatalf("TemplateItems: %v", err)
	}
	if want := []int{-270, -230, -200}; !slices.Equal(offsets(held), want) {
		t.Errorf("offsets = %v, want %v", offsets(held), want)
	}

	entry := trail.only(t)
	if entry.FieldKey != FieldOffset || entry.Old != "sample1 (-240)" || entry.New != "sample1 (-230)" {
		t.Errorf("entry = %+v, want the offset of sample1 moving from -240 to -230", entry)
	}
}

func TestSetTemplateItemOffsetWritesNothingWhenTheOffsetStands(t *testing.T) {
	svc, _, trail := newTestService(t)
	ctx := signedIn(t)
	template, items := chainOfThree(t, svc, ctx)
	trail.entries = nil

	if err := svc.SetTemplateItemOffset(ctx, template.ID, items[1].ID, items[1].Offset); err != nil {
		t.Fatalf("SetTemplateItemOffset: %v", err)
	}
	if len(trail.entries) != 0 {
		t.Errorf("the trail holds %+v, want nothing", trail.entries)
	}
}

func TestSetTemplateItemGapStoresTheOffsetItWorksOutTo(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := signedIn(t)
	template, items := chainOfThree(t, svc, ctx)

	// The item above holds -270, so a gap of 10 is an offset of -260.
	if err := svc.SetTemplateItemGap(ctx, template.ID, items[1].ID, 10); err != nil {
		t.Fatalf("SetTemplateItemGap: %v", err)
	}

	held, err := svc.TemplateItems(ctx, template.ID)
	if err != nil {
		t.Fatalf("TemplateItems: %v", err)
	}
	if want := []int{-270, -260, -200}; !slices.Equal(offsets(held), want) {
		t.Errorf("offsets = %v, want %v", offsets(held), want)
	}
}

func TestSetTemplateItemGapRefusesTheFirstItem(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := signedIn(t)
	template, items := chainOfThree(t, svc, ctx)

	err := svc.SetTemplateItemGap(ctx, template.ID, items[0].ID, 10)
	rejects(t, err, FieldGap, validate.NotAllowed)
}

func TestSetTemplateItemGapKeepsTheOffsetWithinItsBound(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := signedIn(t)
	template, items := chainOfThree(t, svc, ctx)

	// The item above holds -270, so a gap of 300 runs past the target date.
	err := svc.SetTemplateItemGap(ctx, template.ID, items[1].ID, 300)
	rejects(t, err, FieldGap, validate.TooBig)
}

func TestRemoveTemplateItemClosesTheGapInTheOrder(t *testing.T) {
	svc, _, trail := newTestService(t)
	ctx := signedIn(t)
	template, items := chainOfThree(t, svc, ctx)
	trail.entries = nil

	if err := svc.RemoveTemplateItem(ctx, template.ID, items[0].ID); err != nil {
		t.Fatalf("RemoveTemplateItem: %v", err)
	}

	held, err := svc.TemplateItems(ctx, template.ID)
	if err != nil {
		t.Fatalf("TemplateItems: %v", err)
	}
	if want := []int{-240, -200}; !slices.Equal(offsets(held), want) {
		t.Errorf("offsets = %v, want %v", offsets(held), want)
	}
	for i, item := range held {
		if item.Position != i {
			t.Errorf("item %d holds position %d: the order left a hole", i, item.Position)
		}
	}

	entry := trail.only(t)
	if entry.Action != audit.ActionItemRemoved || entry.Old != "tech1 (-270)" {
		t.Errorf("entry = %+v, want tech1 removed", entry)
	}
}

func TestRemoveTemplateItemReportsAnItemThatIsNotThere(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := signedIn(t)
	template, _ := chainOfThree(t, svc, ctx)

	err := svc.RemoveTemplateItem(ctx, template.ID, ids.MustParse("01912345-0000-7000-8000-00000000ffff"))
	if !errors.Is(err, ErrNoItem) {
		t.Errorf("RemoveTemplateItem error = %v, want %v", err, ErrNoItem)
	}
}

func TestReorderTemplateItemsWritesTheStatedOrder(t *testing.T) {
	svc, _, trail := newTestService(t)
	ctx := signedIn(t)
	template, items := chainOfThree(t, svc, ctx)
	trail.entries = nil

	ordered := []ids.ID{items[1].ID, items[0].ID, items[2].ID}
	if err := svc.ReorderTemplateItems(ctx, template.ID, ordered); err != nil {
		t.Fatalf("ReorderTemplateItems: %v", err)
	}

	held, err := svc.TemplateItems(ctx, template.ID)
	if err != nil {
		t.Fatalf("TemplateItems: %v", err)
	}
	if want := []int{-240, -270, -200}; !slices.Equal(offsets(held), want) {
		t.Errorf("offsets = %v, want %v", offsets(held), want)
	}

	entry := trail.only(t)
	if entry.Action != audit.ActionItemsReordered {
		t.Errorf("entry action = %q, want %q", entry.Action, audit.ActionItemsReordered)
	}
	if entry.Old != "tech1, sample1, sample2" || entry.New != "sample1, tech1, sample2" {
		t.Errorf("entry = %+v, want the two orders by name", entry)
	}
}

func TestReorderTemplateItemsRefusesAnOrderThatNamesOtherItems(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := signedIn(t)
	template, items := chainOfThree(t, svc, ctx)

	cases := map[string][]ids.ID{
		"a short order": {items[0].ID, items[1].ID},
		"a repeated id": {items[0].ID, items[0].ID, items[1].ID},
		"a foreign id":  {items[0].ID, items[1].ID, ids.MustParse("01912345-0000-7000-8000-00000000ffff")},
	}
	for name, ordered := range cases {
		t.Run(name, func(t *testing.T) {
			if err := svc.ReorderTemplateItems(ctx, template.ID, ordered); !errors.Is(err, ErrItemOrder) {
				t.Errorf("ReorderTemplateItems error = %v, want %v", err, ErrItemOrder)
			}
			held, err := svc.TemplateItems(ctx, template.ID)
			if err != nil {
				t.Fatalf("TemplateItems: %v", err)
			}
			if want := []int{-270, -240, -200}; !slices.Equal(offsets(held), want) {
				t.Errorf("offsets = %v, want %v: a refused order wrote something", offsets(held), want)
			}
		})
	}
}

// chainOfThree is a template holding three steps, at -270, -240 and -200.
func chainOfThree(t *testing.T, svc *Service, ctx context.Context) (Template, []TemplateItem) {
	t.Helper()

	template := milestoneTemplate(t, svc, ctx, "Import, rail", "Rail")
	items := []TemplateItem{
		templateItem(t, svc, ctx, template.ID, milestoneType(t, svc, ctx, "tech1", "Tech pack 1").ID, -270),
		templateItem(t, svc, ctx, template.ID, milestoneType(t, svc, ctx, "sample1", "1st sample").ID, -240),
		templateItem(t, svc, ctx, template.ID, milestoneType(t, svc, ctx, "sample2", "2nd sample").ID, -200),
	}
	return template, items
}
