package catalog

import (
	"errors"
	"slices"
	"testing"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/platform/date"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

func TestCreateDropRefusesASeasonThatIsNotThere(t *testing.T) {
	svc, _, _ := newTestService(t)

	in := DropCreateParams{
		SeasonID:   ids.MustParse("01912345-6789-7abc-def0-000000000002"),
		Name:       "Drop 1",
		TargetDate: date.MustParse("2027-02-15"),
	}
	if _, err := svc.CreateDrop(signedIn(t), in); !errors.Is(err, ErrNoSeason) {
		t.Errorf("CreateDrop error = %v, want %v", err, ErrNoSeason)
	}
}

func TestCreateDropRefusesAMissingSeasonAndDate(t *testing.T) {
	svc, _, _ := newTestService(t)

	_, err := svc.CreateDrop(t.Context(), DropCreateParams{Name: "Drop 1"})
	rejects(t, err, FieldSeason, validate.Required)
	rejects(t, err, FieldTargetDate, validate.Required)
}

func TestCreateDropRefusesANameTheSeasonHolds(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := signedIn(t)
	season, drop, _ := spine(t, svc, ctx)

	in := DropCreateParams{SeasonID: season.ID, Name: drop.Name, TargetDate: date.MustParse("2027-03-15")}
	if _, err := svc.CreateDrop(ctx, in); !errors.Is(err, ErrDropNameTaken) {
		t.Errorf("CreateDrop error = %v, want %v", err, ErrDropNameTaken)
	}
}

func TestUpdateDropRecordsTheMovedFields(t *testing.T) {
	svc, rows, trail := newTestService(t)
	ctx := signedIn(t)
	_, drop, _ := spine(t, svc, ctx)
	trail.entries = nil

	in := DropUpdateParams{Name: "Drop 1a", TargetDate: date.MustParse("2027-03-01"), Active: true}
	if err := svc.UpdateDrop(ctx, drop.ID, in); err != nil {
		t.Fatalf("UpdateDrop: %v", err)
	}

	got := rows.drops[drop.ID]
	if got.Name != in.Name || !got.TargetDate.Equal(in.TargetDate) {
		t.Errorf("The stored row = %+v, want the edit %+v", got, in)
	}
	if got.SeasonID != drop.SeasonID {
		t.Errorf("SeasonID = %s, want the season it was created in, %s", got.SeasonID, drop.SeasonID)
	}

	want := []string{audit.ActionChanged, audit.ActionChanged}
	if !slices.Equal(trail.actions(), want) {
		t.Errorf("actions = %v, want %v", trail.actions(), want)
	}
}

func TestListDropsReturnsTheDropsOfOneSeason(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := signedIn(t)
	season, drop, _ := spine(t, svc, ctx)

	other, err := svc.CreateSeason(ctx, SeasonCreateParams{Name: "AW27", StartDate: date.MustParse("2027-05-01")})
	if err != nil {
		t.Fatalf("CreateSeason: %v", err)
	}
	elsewhere := DropCreateParams{SeasonID: other.ID, Name: "Drop 1", TargetDate: date.MustParse("2027-08-15")}
	if _, err := svc.CreateDrop(ctx, elsewhere); err != nil {
		t.Fatalf("CreateDrop: %v", err)
	}

	drops, err := svc.ListDrops(ctx, season.ID)
	if err != nil {
		t.Fatalf("ListDrops: %v", err)
	}
	if len(drops) != 1 || drops[0].ID != drop.ID {
		t.Errorf("ListDrops returned %d drops, want the one of season %s", len(drops), season.ID)
	}
}
