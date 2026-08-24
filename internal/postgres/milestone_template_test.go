package postgres

import (
	"errors"
	"slices"
	"testing"

	"github.com/paveltessman/pilam/internal/milestones"
	"github.com/paveltessman/pilam/internal/platform/ids"
)

func TestMilestoneTemplatesRoundTripEveryField(t *testing.T) {
	store := milestoneDB(t)
	want := store.template(t, "Import, rail", "The rail chain")

	got, err := store.templates.ByID(t.Context(), want.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if got != want {
		t.Errorf("ByID = %+v, want %+v", got, want)
	}
}

func TestMilestoneTemplatesReportMissingRowAsErrNoTemplate(t *testing.T) {
	store := milestoneDB(t)

	if _, err := store.templates.ByID(t.Context(), gen.New()); !errors.Is(err, milestones.ErrNoTemplate) {
		t.Errorf("ByID error = %v, want %v", err, milestones.ErrNoTemplate)
	}

	missing := milestones.Template{ID: gen.New(), Name: "Import, air", Description: "Air", Active: true}
	if err := store.templates.Update(t.Context(), missing); !errors.Is(err, milestones.ErrNoTemplate) {
		t.Errorf("Update error = %v, want %v", err, milestones.ErrNoTemplate)
	}
}

func TestMilestoneTemplatesRefuseTakenName(t *testing.T) {
	store := milestoneDB(t)
	held := store.template(t, "Import, rail", "The rail chain")

	for _, name := range []string{"Import, rail", "IMPORT, RAIL"} {
		taken := milestones.Template{ID: gen.New(), Name: name, Description: "Another", Active: true}
		if err := store.templates.Create(t.Context(), taken); !errors.Is(err, milestones.ErrNameTaken) {
			t.Errorf("Create %q error = %v, want %v", name, err, milestones.ErrNameTaken)
		}
	}

	other := store.template(t, "Import, air", "The air chain")
	other.Name = "IMPORT, RAIL"
	if err := store.templates.Update(t.Context(), other); !errors.Is(err, milestones.ErrNameTaken) {
		t.Errorf("Update error = %v, want %v", err, milestones.ErrNameTaken)
	}

	// The row keeps its own name: an update of one row is not a duplicate.
	if err := store.templates.Update(t.Context(), held); err != nil {
		t.Errorf("Update: %v", err)
	}
}

func TestMilestoneTemplatesHoldOneDefault(t *testing.T) {
	store := milestoneDB(t)
	ctx := t.Context()

	first := store.template(t, "Import, rail", "The rail chain")
	first.Default = true
	if err := store.templates.Update(ctx, first); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// A second default is refused while the first one stands, and lands once
	// the store clears it. That is the order the service writes in.
	second := store.template(t, "Import, air", "The air chain")
	second.Default = true
	if err := store.templates.Update(ctx, second); err == nil {
		t.Fatal("Update wrote a second default")
	}

	if err := store.templates.ClearDefault(ctx, second.ID); err != nil {
		t.Fatalf("ClearDefault: %v", err)
	}
	if err := store.templates.Update(ctx, second); err != nil {
		t.Fatalf("Update: %v", err)
	}

	held, err := store.templates.ByID(ctx, first.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if held.Default {
		t.Error("the first template is still the default")
	}
}

func TestMilestoneTemplatesListHoldsBothStatesByName(t *testing.T) {
	store := milestoneDB(t)
	store.template(t, "Import, rail", "The rail chain")
	retired := store.template(t, "Domestic", "The domestic chain")

	retired.Active = false
	if err := store.templates.Update(t.Context(), retired); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := store.templates.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if want := []string{"Domestic", "Import, rail"}; !slices.Equal(templateNames(got), want) {
		t.Errorf("List = %v, want %v", templateNames(got), want)
	}
	if got[0].Active {
		t.Error("the retired template is missing from the list, or came back active")
	}
}

func TestMilestoneTemplateItemsRoundTripInPositionOrder(t *testing.T) {
	store := milestoneDB(t)
	ctx := t.Context()
	template := store.template(t, "Import, rail", "The rail chain")

	first := store.item(t, template.ID, store.milestoneType(t, "tech1", "Tech pack 1").ID, -270, 0)
	second := store.item(t, template.ID, store.milestoneType(t, "sample1", "1st sample").ID, -240, 1)

	got, err := store.templates.Items(ctx, template.ID)
	if err != nil {
		t.Fatalf("Items: %v", err)
	}
	if want := []milestones.TemplateItem{first, second}; !slices.Equal(got, want) {
		t.Errorf("Items = %+v, want %+v", got, want)
	}
}

func TestMilestoneTemplateItemsHoldOneTypePerTemplate(t *testing.T) {
	store := milestoneDB(t)
	template := store.template(t, "Import, rail", "The rail chain")
	milestoneType := store.milestoneType(t, "tech1", "Tech pack 1")
	store.item(t, template.ID, milestoneType.ID, -270, 0)

	twice := milestones.TemplateItem{
		ID: gen.New(), TemplateID: template.ID, TypeID: milestoneType.ID, Offset: -240, Position: 1,
	}
	if err := store.templates.AddItem(t.Context(), twice); !errors.Is(err, milestones.ErrTypeInTemplate) {
		t.Errorf("AddItem error = %v, want %v", err, milestones.ErrTypeInTemplate)
	}
}

func TestMilestoneTemplateItemsRefuseAPositiveOffset(t *testing.T) {
	store := milestoneDB(t)
	template := store.template(t, "Import, rail", "The rail chain")

	after := milestones.TemplateItem{
		ID:         gen.New(),
		TemplateID: template.ID,
		TypeID:     store.milestoneType(t, "tech1", "Tech pack 1").ID,
		Offset:     1,
	}
	if err := store.templates.AddItem(t.Context(), after); err == nil {
		t.Error("AddItem wrote an offset later than the target date")
	}
}

func TestMilestoneTemplateItemsReorderSwapsTwoRows(t *testing.T) {
	store := milestoneDB(t)
	ctx := t.Context()
	template := store.template(t, "Import, rail", "The rail chain")

	first := store.item(t, template.ID, store.milestoneType(t, "tech1", "Tech pack 1").ID, -270, 0)
	second := store.item(t, template.ID, store.milestoneType(t, "sample1", "1st sample").ID, -240, 1)

	// The order key is deferred, so the swap holds the same position on both
	// rows for a moment and still commits.
	if err := store.templates.ReorderItems(ctx, []ids.ID{second.ID, first.ID}); err != nil {
		t.Fatalf("ReorderItems: %v", err)
	}

	got, err := store.templates.Items(ctx, template.ID)
	if err != nil {
		t.Fatalf("Items: %v", err)
	}
	if want := []int{-240, -270}; !slices.Equal(itemOffsets(got), want) {
		t.Errorf("Items = %v, want %v", itemOffsets(got), want)
	}
}

func TestMilestoneTemplateItemRemoveReportsARowThatIsGone(t *testing.T) {
	store := milestoneDB(t)

	if err := store.templates.RemoveItem(t.Context(), gen.New()); !errors.Is(err, milestones.ErrNoItem) {
		t.Errorf("RemoveItem error = %v, want %v", err, milestones.ErrNoItem)
	}
}

func TestMilestoneTemplateItemUpdateWritesTheOffset(t *testing.T) {
	store := milestoneDB(t)
	ctx := t.Context()
	template := store.template(t, "Import, rail", "The rail chain")
	item := store.item(t, template.ID, store.milestoneType(t, "tech1", "Tech pack 1").ID, -270, 0)

	item.Offset = -260
	if err := store.templates.UpdateItem(ctx, item); err != nil {
		t.Fatalf("UpdateItem: %v", err)
	}

	got, err := store.templates.Items(ctx, template.ID)
	if err != nil {
		t.Fatalf("Items: %v", err)
	}
	if len(got) != 1 || got[0].Offset != -260 {
		t.Errorf("Items = %+v, want one item at -260", got)
	}

	missing := milestones.TemplateItem{ID: gen.New(), TemplateID: template.ID, TypeID: item.TypeID}
	if err := store.templates.UpdateItem(ctx, missing); !errors.Is(err, milestones.ErrNoItem) {
		t.Errorf("UpdateItem error = %v, want %v", err, milestones.ErrNoItem)
	}
}

// templateNames is the name of every template of a list, in order.
func templateNames(templates []milestones.Template) []string {
	out := make([]string, len(templates))
	for i, template := range templates {
		out[i] = template.Name
	}
	return out
}

// itemOffsets is the offset of every item of a list, in order.
func itemOffsets(items []milestones.TemplateItem) []int {
	out := make([]int, len(items))
	for i, item := range items {
		out[i] = item.Offset
	}
	return out
}
