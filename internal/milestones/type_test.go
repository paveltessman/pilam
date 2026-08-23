package milestones

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

func TestCreateTypeWritesTheRowAndOneEntry(t *testing.T) {
	svc, rows, trail := newTestService(t)
	ctx := signedIn(t)

	created, err := svc.CreateType(ctx, TypeCreateParams{
		Name:        "  PPS  ",
		Description: "  PPS received  ",
	})
	if err != nil {
		t.Fatalf("CreateType: %v", err)
	}

	if created.Name != "PPS" || created.Description != "PPS received" {
		t.Errorf("the row = %+v, want both fields trimmed", created)
	}
	if !created.Active {
		t.Error("A new milestone type is not active")
	}
	if got := rows.types[created.ID]; got != created {
		t.Errorf("The stored row = %+v, want %+v", got, created)
	}

	entry := trail.only(t)
	if entry.Entity != audit.EntityMilestoneType || entry.Action != audit.ActionCreated {
		t.Errorf("entry = %s/%s, want %s/%s",
			entry.Entity, entry.Action, audit.EntityMilestoneType, audit.ActionCreated)
	}
	if entry.ActorID != actorID {
		t.Errorf("ActorID = %s, want %s", entry.ActorID, actorID)
	}
	if entry.New != "PPS" {
		t.Errorf("entry.New = %q, want the short name %q", entry.New, "PPS")
	}
}

func TestCreateTypeRefusesTwoEmptyFields(t *testing.T) {
	svc, _, trail := newTestService(t)

	_, err := svc.CreateType(t.Context(), TypeCreateParams{Name: "   ", Description: " "})
	rejects(t, err, FieldName, validate.Required)
	rejects(t, err, FieldDescription, validate.Required)

	if len(trail.entries) != 0 {
		t.Errorf("The trail holds %d entries, want 0", len(trail.entries))
	}
}

func TestCreateTypeRefusesTextAboveTheCap(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := signedIn(t)

	_, err := svc.CreateType(ctx, TypeCreateParams{
		Name:        strings.Repeat("s", MaxNameLen+1),
		Description: strings.Repeat("d", MaxDescriptionLen+1),
	})
	rejects(t, err, FieldName, validate.TooLong)
	rejects(t, err, FieldDescription, validate.TooLong)
}

func TestCreateTypeCountsCharactersNotBytes(t *testing.T) {
	svc, _, _ := newTestService(t)

	name := strings.Repeat("э", MaxNameLen)
	_, err := svc.CreateType(signedIn(t), TypeCreateParams{Name: name, Description: "Этап"})
	if err != nil {
		t.Fatalf("CreateType: %v", err)
	}
}

func TestCreateTypeRefusesTakenName(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := signedIn(t)
	milestoneType(t, svc, ctx, "PPS", "PPS received")

	in := TypeCreateParams{Name: "pps", Description: "The pre-production sample"}
	if _, err := svc.CreateType(ctx, in); !errors.Is(err, ErrNameTaken) {
		t.Errorf("CreateType error = %v, want %v", err, ErrNameTaken)
	}
}

func TestUpdateTypeRecordsOneEntryPerMovedField(t *testing.T) {
	svc, rows, trail := newTestService(t)
	ctx := signedIn(t)
	created := milestoneType(t, svc, ctx, "PPS", "PPS received")
	trail.entries = nil

	in := TypeUpdateParams{Name: "PP-S", Description: "The pre-production sample", Active: false}
	if err := svc.UpdateType(ctx, created.ID, in); err != nil {
		t.Fatalf("UpdateType: %v", err)
	}

	got := rows.types[created.ID]
	if got.Name != in.Name || got.Description != in.Description || got.Active {
		t.Errorf("The stored row = %+v, want the edit %+v", got, in)
	}

	want := []string{audit.ActionChanged, audit.ActionChanged, audit.ActionDeactivated}
	if !slices.Equal(trail.actions(), want) {
		t.Errorf("actions = %v, want %v", trail.actions(), want)
	}
}

func TestUpdateTypeKeepsTheRowOfRetiredType(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := signedIn(t)
	created := milestoneType(t, svc, ctx, "PPS", "PPS received")

	in := TypeUpdateParams{Name: created.Name, Description: created.Description, Active: false}
	if err := svc.UpdateType(ctx, created.ID, in); err != nil {
		t.Fatalf("UpdateType: %v", err)
	}

	got, err := svc.Type(ctx, created.ID)
	if err != nil {
		t.Fatalf("Type: %v", err)
	}
	if got.Active {
		t.Error("The type is still active")
	}
	if got.Name != created.Name || got.Description != created.Description {
		t.Errorf("the row = %+v, want the fields of %+v", got, created)
	}
}

func TestUpdateTypeWritesNothingWhenNothingMoved(t *testing.T) {
	svc, _, trail := newTestService(t)
	ctx := signedIn(t)
	created := milestoneType(t, svc, ctx, "PPS", "PPS received")
	trail.entries = nil

	in := TypeUpdateParams{
		Name:        created.Name,
		Description: created.Description,
		Active:      created.Active,
	}
	if err := svc.UpdateType(ctx, created.ID, in); err != nil {
		t.Fatalf("UpdateType: %v", err)
	}

	if len(trail.entries) != 0 {
		t.Errorf("The trail holds %d entries, want 0: %+v", len(trail.entries), trail.entries)
	}
}

func TestUpdateTypeRefusesTakenName(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := signedIn(t)
	milestoneType(t, svc, ctx, "PPS", "PPS received")
	second := milestoneType(t, svc, ctx, "FIT", "Fit approved")

	in := TypeUpdateParams{Name: "pps", Description: second.Description, Active: true}
	if err := svc.UpdateType(ctx, second.ID, in); !errors.Is(err, ErrNameTaken) {
		t.Errorf("UpdateType error = %v, want %v", err, ErrNameTaken)
	}
}

func TestUpdateTypeReportsMissingRow(t *testing.T) {
	svc, _, _ := newTestService(t)

	in := TypeUpdateParams{Name: "PPS", Description: "PPS received", Active: true}
	err := svc.UpdateType(signedIn(t), ids.MustParse("01912345-6789-7abc-def0-000000000001"), in)
	if !errors.Is(err, ErrNoType) {
		t.Errorf("UpdateType error = %v, want %v", err, ErrNoType)
	}
}

func TestTypeReportsMissingRow(t *testing.T) {
	svc, _, _ := newTestService(t)

	_, err := svc.Type(signedIn(t), ids.MustParse("01912345-6789-7abc-def0-000000000001"))
	if !errors.Is(err, ErrNoType) {
		t.Errorf("Type error = %v, want %v", err, ErrNoType)
	}
}

func TestListTypesOrdersByNameAndHoldsRetiredOnes(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := signedIn(t)
	milestoneType(t, svc, ctx, "PPS", "PPS received")
	retired := milestoneType(t, svc, ctx, "FIT", "Fit approved")

	in := TypeUpdateParams{Name: retired.Name, Description: retired.Description, Active: false}
	if err := svc.UpdateType(ctx, retired.ID, in); err != nil {
		t.Fatalf("UpdateType: %v", err)
	}

	types, err := svc.ListTypes(ctx)
	if err != nil {
		t.Fatalf("ListTypes: %v", err)
	}
	if len(types) != 2 || types[0].Name != "FIT" || types[1].Name != "PPS" {
		t.Fatalf("ListTypes = %+v, want FIT then PPS", types)
	}
	if types[0].Active {
		t.Error("The list dropped the retired type")
	}
}
