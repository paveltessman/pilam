package catalog

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/platform/date"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

func TestCreateSeasonWritesTheRowAndOneEntry(t *testing.T) {
	svc, rows, trail := newTestService(t)
	ctx := signedIn(t)

	season, err := svc.CreateSeason(ctx, SeasonCreateParams{Name: "  S1 ", StartDate: date.MustParse("2026-11-01")})
	if err != nil {
		t.Fatalf("CreateSeason: %v", err)
	}

	if season.Name != "S1" {
		t.Errorf("Name = %q, want the trimmed %q", season.Name, "S1")
	}
	if !season.Active {
		t.Error("A new season is not active")
	}
	if got := rows.seasons[season.ID]; got != season {
		t.Errorf("The stored row = %+v, want %+v", got, season)
	}

	entry := trail.only(t)
	if entry.Entity != audit.EntitySeason || entry.Action != audit.ActionCreated {
		t.Errorf("entry = %s/%s, want %s/%s", entry.Entity, entry.Action, audit.EntitySeason, audit.ActionCreated)
	}
	if entry.ActorID != actorID {
		t.Errorf("ActorID = %s, want %s", entry.ActorID, actorID)
	}
}

func TestCreateSeasonKeepsTheDayAndDropsTheTimeOfDay(t *testing.T) {
	svc, _, _ := newTestService(t)

	noon := time.Date(2026, 11, 1, 12, 30, 0, 0, time.UTC)
	season, err := svc.CreateSeason(signedIn(t), SeasonCreateParams{Name: "S1", StartDate: noon})
	if err != nil {
		t.Fatalf("CreateSeason: %v", err)
	}

	if want := date.MustParse("2026-11-01"); !season.StartDate.Equal(want) {
		t.Errorf("StartDate = %v, want %v", season.StartDate, want)
	}
}

func TestCreateSeasonRefusesAnEmptyNameAndAMissingDate(t *testing.T) {
	svc, _, trail := newTestService(t)

	_, err := svc.CreateSeason(t.Context(), SeasonCreateParams{Name: "   "})
	rejects(t, err, FieldName, validate.Required)
	rejects(t, err, FieldStartDate, validate.Required)

	if len(trail.entries) != 0 {
		t.Errorf("The trail holds %d entries, want 0", len(trail.entries))
	}
}

func TestCreateSeasonRefusesATakenName(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := signedIn(t)

	in := SeasonCreateParams{Name: "S1", StartDate: date.MustParse("2026-11-01")}
	if _, err := svc.CreateSeason(ctx, in); err != nil {
		t.Fatalf("CreateSeason: %v", err)
	}

	in.Name = "s1"
	if _, err := svc.CreateSeason(ctx, in); !errors.Is(err, ErrSeasonNameTaken) {
		t.Errorf("CreateSeason error = %v, want %v", err, ErrSeasonNameTaken)
	}
}

func TestUpdateSeasonRecordsOneEntryPerMovedField(t *testing.T) {
	svc, rows, trail := newTestService(t)
	ctx := signedIn(t)
	season, _, _ := spine(t, svc, ctx)
	trail.entries = nil

	in := SeasonUpdateParams{Name: "S1 Main", StartDate: date.MustParse("2026-12-01"), Active: false}
	if err := svc.UpdateSeason(ctx, season.ID, in); err != nil {
		t.Fatalf("UpdateSeason: %v", err)
	}

	got := rows.seasons[season.ID]
	if got.Name != in.Name || !got.StartDate.Equal(in.StartDate) || got.Active {
		t.Errorf("The stored row = %+v, want the edit %+v", got, in)
	}

	want := []string{audit.ActionChanged, audit.ActionChanged, audit.ActionDeactivated}
	if !slices.Equal(trail.actions(), want) {
		t.Errorf("actions = %v, want %v", trail.actions(), want)
	}
}

func TestUpdateSeasonWritesNothingWhenNothingMoved(t *testing.T) {
	svc, _, trail := newTestService(t)
	ctx := signedIn(t)
	season, _, _ := spine(t, svc, ctx)
	trail.entries = nil

	in := SeasonUpdateParams{Name: season.Name, StartDate: season.StartDate, Active: season.Active}
	if err := svc.UpdateSeason(ctx, season.ID, in); err != nil {
		t.Fatalf("UpdateSeason: %v", err)
	}

	if len(trail.entries) != 0 {
		t.Errorf("The trail holds %d entries, want 0: %+v", len(trail.entries), trail.entries)
	}
}

func TestUpdateSeasonReportsAMissingRow(t *testing.T) {
	svc, _, _ := newTestService(t)

	in := SeasonUpdateParams{Name: "S1", StartDate: date.MustParse("2026-11-01"), Active: true}
	err := svc.UpdateSeason(signedIn(t), ids.MustParse("01912345-6789-7abc-def0-000000000001"), in)
	if !errors.Is(err, ErrNoSeason) {
		t.Errorf("UpdateSeason error = %v, want %v", err, ErrNoSeason)
	}
}

func TestListSeasonsOrdersByStartDate(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := signedIn(t)

	// A1 sorts before S1 alphabetically, and opens after it.
	for _, in := range []SeasonCreateParams{
		{Name: "A1", StartDate: date.MustParse("2027-05-01")},
		{Name: "S1", StartDate: date.MustParse("2026-11-01")},
	} {
		if _, err := svc.CreateSeason(ctx, in); err != nil {
			t.Fatalf("CreateSeason %q: %v", in.Name, err)
		}
	}

	seasons, err := svc.ListSeasons(ctx)
	if err != nil {
		t.Fatalf("ListSeasons: %v", err)
	}
	if len(seasons) != 2 || seasons[0].Name != "S1" || seasons[1].Name != "A1" {
		t.Errorf("ListSeasons = %v, want S1 then A1", []string{seasons[0].Name, seasons[1].Name})
	}
}
