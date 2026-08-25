package catalog

import (
	"errors"
	"fmt"
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

// season is one row the current-season rule reads.
func season(n int, name, start string, active bool) Season {
	return Season{
		ID:        ids.MustParse(fmt.Sprintf("01912345-0000-7000-8000-%012d", n)),
		Name:      name,
		StartDate: date.MustParse(start),
		Active:    active,
	}
}

func TestCurrentSeasonIsTheActiveSeasonThatStartedLast(t *testing.T) {
	seasons := []Season{
		season(1, "SS25", "2025-11-01", true),
		season(2, "SS26", "2026-05-01", true),

		// It started, and nobody works in it any more.
		season(3, "AW26", "2026-06-01", false),

		// It is active, and it has not started yet.
		season(4, "SS27", "2027-01-01", true),
	}

	got := CurrentSeason(seasons, date.MustParse("2026-08-18"))

	if got.Name != "SS26" {
		t.Errorf("CurrentSeason = %q, want the active season that started last", got.Name)
	}
}

func TestCurrentSeasonCountsASeasonThatStartsToday(t *testing.T) {
	seasons := []Season{
		season(1, "SS26", "2026-01-01", true),
		season(2, "AW26", "2026-08-18", true),
	}

	if got := CurrentSeason(seasons, date.MustParse("2026-08-18")); got.Name != "AW26" {
		t.Errorf("CurrentSeason = %q, want the season that starts today", got.Name)
	}
}

func TestCurrentSeasonTakesTheEarliestWhenEveryActiveSeasonStartsLater(t *testing.T) {
	seasons := []Season{
		season(1, "AW27", "2027-05-01", true),
		season(2, "SS27", "2027-01-01", true),
		season(3, "SS26", "2026-01-01", false),
	}

	got := CurrentSeason(seasons, date.MustParse("2026-08-18"))

	if got.Name != "SS27" {
		t.Errorf("CurrentSeason = %q, want the active season that starts first", got.Name)
	}
}

func TestCurrentSeasonTakesTheNewestWhenNoSeasonIsActive(t *testing.T) {
	seasons := []Season{
		season(1, "SS25", "2025-01-01", false),
		season(2, "SS26", "2026-01-01", false),
	}

	got := CurrentSeason(seasons, date.MustParse("2026-08-18"))

	if got.Name != "SS26" {
		t.Errorf("CurrentSeason = %q, want the newest season of all", got.Name)
	}
}

func TestCurrentSeasonOfNothing(t *testing.T) {
	if got := CurrentSeason(nil, date.MustParse("2026-08-18")); got.ID != ids.Nil {
		t.Errorf("CurrentSeason = %+v, want the zero season", got)
	}
}

// Two seasons that start on the same day break the tie by name, so the answer
// does not depend on the order the rows arrive in.
func TestCurrentSeasonAnswersTheSameWhateverOrderItReads(t *testing.T) {
	today := date.MustParse("2026-08-18")
	seasons := []Season{
		season(1, "SS26 main", "2026-05-01", true),
		season(2, "SS26 carry", "2026-05-01", true),
		season(3, "SS25", "2026-01-01", true),
	}
	want := CurrentSeason(seasons, today)

	slices.Reverse(seasons)
	if got := CurrentSeason(seasons, today); got != want {
		t.Errorf("the reversed read answers %q, want %q", got.Name, want.Name)
	}
}
