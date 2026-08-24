package postgres

import (
	"errors"
	"slices"
	"testing"

	"github.com/paveltessman/pilam/internal/milestones"
)

func TestMilestoneTypesRoundTripEveryField(t *testing.T) {
	t.Parallel()

	store := milestoneDB(t)
	want := store.milestoneType(t, "fit", "Fitting")

	got, err := store.types.ByID(t.Context(), want.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}

	if got != want {
		t.Errorf("ByID = %+v, want %+v", got, want)
	}
}

func TestMilestoneTypesReportMissingRowAsErrNoType(t *testing.T) {
	t.Parallel()

	store := milestoneDB(t)

	if _, err := store.types.ByID(t.Context(), gen.New()); !errors.Is(err, milestones.ErrNoType) {
		t.Errorf("ByID error = %v, want %v", err, milestones.ErrNoType)
	}

	missing := milestones.Type{ID: gen.New(), Name: "fit", Description: "Fitting", Active: true}
	if err := store.types.Update(t.Context(), missing); !errors.Is(err, milestones.ErrNoType) {
		t.Errorf("Update error = %v, want %v", err, milestones.ErrNoType)
	}
}

func TestMilestoneTypesRefusesTakenName(t *testing.T) {
	t.Parallel()

	store := milestoneDB(t)
	held := store.milestoneType(t, "fit", "Fitting")

	for _, name := range []string{"fit", "FIT"} {
		taken := milestones.Type{ID: gen.New(), Name: name, Description: "Second fitting", Active: true}
		if err := store.types.Create(t.Context(), taken); !errors.Is(err, milestones.ErrNameTaken) {
			t.Errorf("Create %q error = %v, want %v", name, err, milestones.ErrNameTaken)
		}
	}

	other := store.milestoneType(t, "shoot", "Photo shoot")
	other.Name = "FIT"
	if err := store.types.Update(t.Context(), other); !errors.Is(err, milestones.ErrNameTaken) {
		t.Errorf("Update error = %v, want %v", err, milestones.ErrNameTaken)
	}

	// The row keeps its own name: an update of one row is not a duplicate.
	if err := store.types.Update(t.Context(), held); err != nil {
		t.Errorf("Update: %v", err)
	}
}

func TestMilestoneTypesRefusesTakenID(t *testing.T) {
	t.Parallel()

	store := milestoneDB(t)
	held := store.milestoneType(t, "fit", "Fitting")

	taken := milestones.Type{ID: held.ID, Name: "shoot", Description: "Photo shoot", Active: true}
	if err := store.types.Create(t.Context(), taken); !errors.Is(err, milestones.ErrIDTaken) {
		t.Errorf("Create error = %v, want %v", err, milestones.ErrIDTaken)
	}
}

func TestMilestoneTypesUpdateWritesEveryField(t *testing.T) {
	t.Parallel()

	store := milestoneDB(t)
	want := store.milestoneType(t, "fit", "Fitting")

	want.Name = "fit1"
	want.Description = "First fitting"
	want.Active = false
	if err := store.types.Update(t.Context(), want); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := store.types.ByID(t.Context(), want.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}

	if got != want {
		t.Errorf("ByID = %+v, want %+v", got, want)
	}
}

func TestMilestoneTypesListHoldsBothStatesByShortName(t *testing.T) {
	t.Parallel()

	store := milestoneDB(t)
	store.milestoneType(t, "shoot", "Photo shoot")
	retired := store.milestoneType(t, "fit", "Fitting")

	retired.Active = false
	if err := store.types.Update(t.Context(), retired); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := store.types.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if want := []string{"fit", "shoot"}; !slices.Equal(names(got), want) {
		t.Errorf("List = %v, want %v", names(got), want)
	}
	if got[0].Active {
		t.Error("The retired type is missing from the list, or came back active")
	}
}

func TestMilestoneTypesListIsEmptyWithoutRows(t *testing.T) {
	t.Parallel()

	store := milestoneDB(t)

	got, err := store.types.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("List = %+v, want none", got)
	}
}
