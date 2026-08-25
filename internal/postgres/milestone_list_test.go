package postgres

import (
	"slices"
	"testing"

	"github.com/paveltessman/pilam/internal/milestones"
	"github.com/paveltessman/pilam/internal/platform/date"
	"github.com/paveltessman/pilam/internal/platform/ids"
)

// listedArticles is the article of every row of a list, in order.
func listedArticles(rows []milestones.ListRow) []string {
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = row.Article
	}
	return out
}

// countedDrops is the drop and the count of every row of the no-calendar read.
func countedDrops(counts []milestones.NoCalendar) map[string]int {
	out := make(map[string]int, len(counts))
	for _, count := range counts {
		out[count.DropName] = count.Models
	}
	return out
}

func TestMilestonesListNamesTheModelTheDropAndTheType(t *testing.T) {
	t.Parallel()

	store := milestoneDB(t)
	season := store.catalog.season(t, "SS26", "2026-01-05")
	drop := store.catalog.drop(t, season, "Drop 1", "2026-07-16")
	model := store.catalog.model(t, drop, "ART-1")
	fit := store.milestoneType(t, "fit", "Fit approved")

	want := milestones.Milestone{
		ID:       gen.New(),
		ModelID:  model.ID,
		TypeID:   fit.ID,
		Baseline: date.MustParse("2026-02-11"),
		Plan:     date.MustParse("2026-02-20"),
		Fact:     date.MustParse("2026-02-18"),
		Note:     "the fabric arrived late",
		Active:   true,
	}
	if err := store.milestones.Create(t.Context(), want); err != nil {
		t.Fatalf("Create: %v", err)
	}

	rows, err := store.milestones.List(t.Context(), milestones.ListParams{SeasonID: season.ID})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("List holds %d rows, want 1", len(rows))
	}

	got := rows[0]
	if got.Milestone != want {
		t.Errorf("the row = %+v, want %+v", got.Milestone, want)
	}
	if got.TypeName != fit.Name || got.Article != model.Article {
		t.Errorf("the row names %q of %q, want %q of %q", got.TypeName, got.Article, fit.Name, model.Article)
	}
	if got.DropID != drop.ID || got.DropName != drop.Name {
		t.Errorf("the row names drop %s %q, want %s %q", got.DropID, got.DropName, drop.ID, drop.Name)
	}
}

func TestMilestonesListKeepsWhatTheFilterNames(t *testing.T) {
	t.Parallel()

	store := milestoneDB(t)
	fit := store.milestoneType(t, "fit", "Fit approved")
	shipped := store.milestoneType(t, "shipped", "Shipped")

	season := store.catalog.season(t, "SS26", "2026-01-05")
	first := store.catalog.drop(t, season, "Drop 1", "2026-07-16")
	second := store.catalog.drop(t, season, "Drop 2", "2026-09-01")

	// A whole season of its own, to prove the season narrows the read.
	other := store.catalog.season(t, "SS27", "2027-01-05")
	otherDrop := store.catalog.drop(t, other, "Drop 1", "2027-07-16")

	store.milestone(t, store.catalog.model(t, first, "A-1"), fit.ID, "2026-04-01")
	store.milestone(t, store.catalog.model(t, second, "B-1"), fit.ID, "2026-05-01")
	store.milestone(t, store.catalog.model(t, second, "B-2"), shipped.ID, "2026-06-01")
	store.milestone(t, store.catalog.model(t, otherDrop, "C-1"), fit.ID, "2027-04-01")

	testData := map[string]struct {
		filter milestones.ListParams
		want   []string
	}{
		"the whole season": {milestones.ListParams{SeasonID: season.ID}, []string{"A-1", "B-1", "B-2"}},
		"one drop":         {milestones.ListParams{SeasonID: season.ID, DropID: second.ID}, []string{"B-1", "B-2"}},
		"one type":         {milestones.ListParams{SeasonID: season.ID, TypeID: shipped.ID}, []string{"B-2"}},
		"a drop and a type": {
			milestones.ListParams{SeasonID: season.ID, DropID: first.ID, TypeID: shipped.ID},
			nil,
		},
		"the other season": {milestones.ListParams{SeasonID: other.ID}, []string{"C-1"}},
		"no season at all": {milestones.ListParams{}, nil},
	}
	for name, tc := range testData {
		t.Run(name, func(t *testing.T) {
			rows, err := store.milestones.List(t.Context(), tc.filter)
			if err != nil {
				t.Fatalf("List: %v", err)
			}

			got := listedArticles(rows)
			slices.Sort(got)
			want := slices.Clone(tc.want)
			slices.Sort(want)

			if !slices.Equal(got, want) {
				t.Errorf("List holds %v, want %v", got, want)
			}
		})
	}
}

// A retired step left the calendar, and a cancelled model is not late. Neither
// of them belongs on the list.
func TestMilestonesListLeavesOutARetiredStepAndACancelledModel(t *testing.T) {
	t.Parallel()

	store := milestoneDB(t)
	fit := store.milestoneType(t, "fit", "Fit approved")
	season := store.catalog.season(t, "SS26", "2026-01-05")
	drop := store.catalog.drop(t, season, "Drop 1", "2026-07-16")

	store.milestone(t, store.catalog.model(t, drop, "on the list"), fit.ID, "2026-04-01")

	retired := store.milestone(t, store.catalog.model(t, drop, "retired step"), fit.ID, "2026-04-01")
	retired.Active = false
	if err := store.milestones.Update(t.Context(), retired); err != nil {
		t.Fatalf("Update: %v", err)
	}

	cancelled := store.catalog.model(t, drop, "cancelled model")
	store.milestone(t, cancelled, fit.ID, "2026-04-01")
	cancelled.Active = false
	if err := store.catalog.models.Update(t.Context(), cancelled); err != nil {
		t.Fatalf("Models.Update: %v", err)
	}

	rows, err := store.milestones.List(t.Context(), milestones.ListParams{SeasonID: season.ID})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if got := listedArticles(rows); !slices.Equal(got, []string{"on the list"}) {
		t.Errorf("List holds %v, want the one active row", got)
	}
}

func TestModelsWithoutCalendarCountsThemPerDrop(t *testing.T) {
	t.Parallel()

	store := milestoneDB(t)
	fit := store.milestoneType(t, "fit", "Fit approved")
	season := store.catalog.season(t, "SS26", "2026-01-05")
	first := store.catalog.drop(t, season, "Drop 1", "2026-07-16")
	second := store.catalog.drop(t, season, "Drop 2", "2026-09-01")

	// Drop 1: one model with a calendar, and one without.
	store.milestone(t, store.catalog.model(t, first, "A-1"), fit.ID, "2026-04-01")
	store.catalog.model(t, first, "A-2")

	// Drop 2: one model whose only step is retired, which is a model with no
	// calendar, and one cancelled model, which is nobody's problem.
	retired := store.milestone(t, store.catalog.model(t, second, "B-1"), fit.ID, "2026-05-01")
	retired.Active = false
	if err := store.milestones.Update(t.Context(), retired); err != nil {
		t.Fatalf("Update: %v", err)
	}
	cancelled := store.catalog.model(t, second, "B-2")
	cancelled.Active = false
	if err := store.catalog.models.Update(t.Context(), cancelled); err != nil {
		t.Fatalf("Models.Update: %v", err)
	}

	counts, err := store.milestones.WithoutCalendar(t.Context(), season.ID, ids.Nil)
	if err != nil {
		t.Fatalf("WithoutCalendar: %v", err)
	}

	want := map[string]int{first.Name: 1, second.Name: 1}
	got := countedDrops(counts)
	if len(got) != len(want) {
		t.Fatalf("WithoutCalendar holds %v, want %v", got, want)
	}
	for drop, models := range want {
		if got[drop] != models {
			t.Errorf("%s holds %d models with no calendar, want %d", drop, got[drop], models)
		}
	}

	// The count follows the drop the screen stands on.
	counts, err = store.milestones.WithoutCalendar(t.Context(), season.ID, first.ID)
	if err != nil {
		t.Fatalf("WithoutCalendar: %v", err)
	}
	if got := countedDrops(counts); len(got) != 1 || got[first.Name] != 1 {
		t.Errorf("the drop filter holds %v, want one row of %q", got, first.Name)
	}
}

func TestModelsWithoutCalendarOfASeasonThatHoldsNothing(t *testing.T) {
	t.Parallel()

	store := milestoneDB(t)
	season := store.catalog.season(t, "SS26", "2026-01-05")

	counts, err := store.milestones.WithoutCalendar(t.Context(), season.ID, ids.Nil)
	if err != nil {
		t.Fatalf("WithoutCalendar: %v", err)
	}
	if len(counts) != 0 {
		t.Errorf("WithoutCalendar holds %v, want nothing", counts)
	}
}

// The list orders its rows the same way twice, so a screen that reads it twice
// reads the same thing. The milestone package orders what the screen shows.
func TestMilestonesListReadsInAStableOrder(t *testing.T) {
	t.Parallel()

	store := milestoneDB(t)
	fit := store.milestoneType(t, "fit", "Fit approved")
	season := store.catalog.season(t, "SS26", "2026-01-05")
	drop := store.catalog.drop(t, season, "Drop 1", "2026-07-16")

	for _, article := range []string{"A-1", "A-2", "A-3"} {
		store.milestone(t, store.catalog.model(t, drop, article), fit.ID, "2026-04-01")
	}

	filter := milestones.ListParams{SeasonID: season.ID}
	first, err := store.milestones.List(t.Context(), filter)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	second, err := store.milestones.List(t.Context(), filter)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if !slices.Equal(listedArticles(first), listedArticles(second)) {
		t.Errorf("two reads hold %v and %v", listedArticles(first), listedArticles(second))
	}
}
