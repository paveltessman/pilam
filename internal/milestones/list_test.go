package milestones

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/platform/date"
	"github.com/paveltessman/pilam/internal/platform/ids"
)

// The day every row of these tests is read against.
var listToday = date.MustParse("2026-08-18")

// listed is one row the list reads: the number a test names it by, the article
// of its model, and the two dates the slip comes from. It is active and it
// holds no fact date.
func listed(n int, article, baseline, plan string) ListRow {
	row := ListRow{
		Milestone: Milestone{
			ID:       ids.MustParse(fmt.Sprintf("01912345-0000-7000-8000-%012d", n)),
			Baseline: date.MustParse(baseline),
			Plan:     date.MustParse(plan),
			Active:   true,
		},
		TypeName: "Fit approved",
		Article:  article,
		DropName: "Drop 1",
	}
	return row
}

// articles is what the list shows, in the order it shows it.
func articles(rows []ListRow) []string {
	names := make([]string, len(rows))
	for i, row := range rows {
		names[i] = row.Article
	}
	return names
}

func TestListedReadsTheStateOfEveryRowAgainstToday(t *testing.T) {
	// One row per state: late, due, planned, and done.
	held := []ListRow{
		listed(1, "A-1", "2026-08-10", "2026-08-10"),
		listed(2, "A-2", "2026-08-20", "2026-08-20"),
		listed(3, "A-3", "2026-09-30", "2026-09-30"),
		listed(4, "A-4", "2026-08-01", "2026-08-01"),
	}
	held[3].Fact = date.MustParse("2026-08-01")

	rows := Listed(held, ListParams{}, listToday)

	want := map[string]State{"A-1": StateLate, "A-2": StateDue, "A-3": StatePlanned, "A-4": StateDone}
	if len(rows) != len(want) {
		t.Fatalf("the list holds %d rows, want %d", len(rows), len(want))
	}
	for _, row := range rows {
		if row.State != want[row.Article] {
			t.Errorf("%s reads as %q, want %q", row.Article, row.State, want[row.Article])
		}
	}
}

func TestListedKeepsTheStatesTheFilterNames(t *testing.T) {
	held := []ListRow{
		listed(1, "late", "2026-08-10", "2026-08-10"),
		listed(2, "due", "2026-08-20", "2026-08-20"),
		listed(3, "planned", "2026-09-30", "2026-09-30"),
		listed(4, "done", "2026-08-01", "2026-08-01"),
	}
	held[3].Fact = date.MustParse("2026-08-01")

	testData := map[string]struct {
		states []State
		want   []string
	}{
		"the list as it opens": {LateAndDue(), []string{"late", "due"}},
		"late alone":           {[]State{StateLate}, []string{"late"}},
		"done alone":           {[]State{StateDone}, []string{"done"}},
		"no state named":       {nil, []string{"late", "due", "planned", "done"}},
	}
	for name, tc := range testData {
		t.Run(name, func(t *testing.T) {
			got := articles(Listed(held, ListParams{States: tc.states}, listToday))
			slices.Sort(got)
			want := slices.Clone(tc.want)
			slices.Sort(want)

			if !slices.Equal(got, want) {
				t.Errorf("the list holds %v, want %v", got, want)
			}
		})
	}
}

func TestListedOrdersWorstSlipFirst(t *testing.T) {
	held := []ListRow{
		listed(1, "moved 3", "2026-08-10", "2026-08-13"),
		listed(2, "moved 21", "2026-08-10", "2026-08-31"),
		listed(3, "moved 10", "2026-08-10", "2026-08-20"),

		// A step that came in early. It moved the other way, so it stands last.
		listed(4, "pulled in", "2026-08-10", "2026-08-05"),
	}

	got := articles(Listed(held, ListParams{}, listToday))

	want := []string{"moved 21", "moved 10", "moved 3", "pulled in"}
	if !slices.Equal(got, want) {
		t.Errorf("the list holds %v, want %v", got, want)
	}
}

func TestListedBreaksATieByPlanDateAndThenByArticle(t *testing.T) {
	// Every row moved 10 days, so the slip says nothing about the order.
	held := []ListRow{
		listed(1, "B-2", "2026-08-20", "2026-08-30"),
		listed(2, "B-1", "2026-08-20", "2026-08-30"),
		listed(3, "A-9", "2026-08-05", "2026-08-15"),
	}

	got := articles(Listed(held, ListParams{}, listToday))

	want := []string{"A-9", "B-1", "B-2"}
	if !slices.Equal(got, want) {
		t.Errorf("the list holds %v, want %v", got, want)
	}
}

func TestListedOrdersTheSameWhateverOrderItReads(t *testing.T) {
	held := []ListRow{
		listed(1, "A-1", "2026-08-10", "2026-08-20"),
		listed(2, "A-2", "2026-08-10", "2026-08-20"),
		listed(3, "A-3", "2026-08-10", "2026-08-31"),
	}
	want := articles(Listed(held, ListParams{}, listToday))

	slices.Reverse(held)
	if got := articles(Listed(held, ListParams{}, listToday)); !slices.Equal(got, want) {
		t.Errorf("the reversed read holds %v, want %v", got, want)
	}
}

func TestListedKeepsTheArticlesThatAnswerEveryTerm(t *testing.T) {
	held := []ListRow{
		listed(1, "ART-100 blue", "2026-08-10", "2026-08-10"),
		listed(2, "ART-200 red", "2026-08-10", "2026-08-10"),
		listed(3, "SKI-100 blue", "2026-08-10", "2026-08-10"),
	}

	testData := map[string]struct {
		search string
		want   []string
	}{
		"one term":           {"art", []string{"ART-100 blue", "ART-200 red"}},
		"every term":         {"art blue", []string{"ART-100 blue"}},
		"any case":           {"ART-100", []string{"ART-100 blue"}},
		"nothing matches":    {"art green", nil},
		"an empty search":    {"   ", []string{"ART-100 blue", "ART-200 red", "SKI-100 blue"}},
		"terms in any order": {"blue ski", []string{"SKI-100 blue"}},
	}
	for name, tc := range testData {
		t.Run(name, func(t *testing.T) {
			got := articles(Listed(held, ListParams{Search: tc.search}, listToday))
			slices.Sort(got)
			want := slices.Clone(tc.want)
			slices.Sort(want)

			if !slices.Equal(got, want) {
				t.Errorf("the search for %q holds %v, want %v", tc.search, got, want)
			}
		})
	}
}

func TestListedOfNothing(t *testing.T) {
	if rows := Listed(nil, ListParams{States: LateAndDue()}, listToday); len(rows) != 0 {
		t.Errorf("the list holds %d rows, want 0", len(rows))
	}
}

func TestListedLeavesTheRowsItReadWhereTheyStand(t *testing.T) {
	held := []ListRow{
		listed(1, "A-1", "2026-08-10", "2026-08-20"),
		listed(2, "A-2", "2026-08-10", "2026-08-31"),
	}

	Listed(held, ListParams{}, listToday)

	if held[0].Article != "A-1" || held[0].State != "" {
		t.Errorf("the read moved the rows it was given: %+v", held[0])
	}
}

// The list opens on the work that runs late and the work that comes due, and
// nothing else names that pair.
func TestLateAndDueIsTheTwoStatesTheListOpensOn(t *testing.T) {
	want := []State{StateLate, StateDue}
	if got := LateAndDue(); !slices.Equal(got, want) {
		t.Errorf("LateAndDue = %v, want %v", got, want)
	}
}

func TestDueWindowBoundsTheDueState(t *testing.T) {
	edge := date.AddDays(listToday, DueWindow)
	held := []ListRow{
		listed(1, "on the edge", date.ISO(edge), date.ISO(edge)),
		listed(2, "past the edge", date.ISO(date.AddDays(edge, 1)), date.ISO(date.AddDays(edge, 1))),
	}

	rows := Listed(held, ListParams{States: []State{StateDue}}, listToday)

	if got := articles(rows); !slices.Equal(got, []string{"on the edge"}) {
		t.Errorf("the due rows are %v, want the row on the edge alone", got)
	}
}

// The state of a row is read against the day the caller states, and not against
// the day the machine stands on.
func TestListedReadsAgainstTheDayItIsGiven(t *testing.T) {
	held := []ListRow{listed(1, "A-1", "2026-08-20", "2026-08-20")}

	testData := map[string]struct {
		today time.Time
		want  State
	}{
		"the day before": {date.MustParse("2026-08-19"), StateDue},
		"the day after":  {date.MustParse("2026-08-21"), StateLate},
	}
	for name, tc := range testData {
		t.Run(name, func(t *testing.T) {
			rows := Listed(held, ListParams{}, tc.today)
			if len(rows) != 1 || rows[0].State != tc.want {
				t.Errorf("the row reads as %q, want %q", rows[0].State, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------- the service

// errShaky is a store that cannot answer.
var errShaky = errors.New("the store is down")

func TestListMilestonesHandsTheFilterToTheStore(t *testing.T) {
	svc, rows, _ := newTestService(t)

	filter := ListParams{
		SeasonID: ids.MustParse("01912345-0000-7000-8000-000000000101"),
		DropID:   ids.MustParse("01912345-0000-7000-8000-000000000102"),
		TypeID:   ids.MustParse("01912345-0000-7000-8000-000000000103"),
		States:   LateAndDue(),
		Search:   "art",
	}
	if _, err := svc.ListMilestones(signedIn(t), filter); err != nil {
		t.Fatalf("ListMilestones: %v", err)
	}

	// The season, the drop and the type are what the store narrows on. The
	// state and the search run in this package, and the store sees them too
	// because the filter travels whole.
	if rows.asked.SeasonID != filter.SeasonID || rows.asked.DropID != filter.DropID {
		t.Errorf("the store was asked for season %s and drop %s, want %s and %s",
			rows.asked.SeasonID, rows.asked.DropID, filter.SeasonID, filter.DropID)
	}
	if rows.asked.TypeID != filter.TypeID {
		t.Errorf("the store was asked for type %s, want %s", rows.asked.TypeID, filter.TypeID)
	}
}

// The service reads the state against the current business day, which is the
// one place the clock is read.
func TestListMilestonesReadsTheStateAgainstTheBusinessDay(t *testing.T) {
	svc, rows, _ := newTestService(t)

	// testNow stands on 2026-08-19.
	rows.list = []ListRow{
		listed(1, "late", "2026-08-18", "2026-08-18"),
		listed(2, "due", "2026-08-21", "2026-08-21"),
		listed(3, "planned", "2026-10-01", "2026-10-01"),
	}

	held, err := svc.ListMilestones(signedIn(t), ListParams{})
	if err != nil {
		t.Fatalf("ListMilestones: %v", err)
	}

	want := map[string]State{"late": StateLate, "due": StateDue, "planned": StatePlanned}
	if len(held) != len(want) {
		t.Fatalf("the list holds %d rows, want %d", len(held), len(want))
	}
	for _, row := range held {
		if row.State != want[row.Article] {
			t.Errorf("%s reads as %q, want %q", row.Article, row.State, want[row.Article])
		}
	}
}

func TestListMilestonesKeepsAndOrdersWhatTheStoreAnswers(t *testing.T) {
	svc, rows, _ := newTestService(t)

	rows.list = []ListRow{
		listed(1, "ART-1 moved 3", "2026-08-10", "2026-08-13"),
		listed(2, "ART-2 moved 21", "2026-08-10", "2026-08-31"),
		listed(3, "SKI-1 moved 10", "2026-08-10", "2026-08-20"),
	}

	held, err := svc.ListMilestones(signedIn(t), ListParams{Search: "art"})
	if err != nil {
		t.Fatalf("ListMilestones: %v", err)
	}

	want := []string{"ART-2 moved 21", "ART-1 moved 3"}
	if got := articles(held); !slices.Equal(got, want) {
		t.Errorf("the list holds %v, want %v", got, want)
	}
}

func TestListMilestonesReportsAFailedRead(t *testing.T) {
	svc, rows, _ := newTestService(t)
	rows.failWith = errShaky

	if _, err := svc.ListMilestones(signedIn(t), ListParams{}); !errors.Is(err, errShaky) {
		t.Errorf("ListMilestones error = %v, want %v", err, errShaky)
	}
}

func TestModelsWithoutCalendarCountsWhatTheStoreAnswers(t *testing.T) {
	svc, rows, _ := newTestService(t)

	seasonID := ids.MustParse("01912345-0000-7000-8000-000000000101")
	dropID := ids.MustParse("01912345-0000-7000-8000-000000000102")
	rows.counts = []NoCalendar{{DropID: dropID, DropName: "Drop 1", Models: 12}}

	counts, err := svc.ModelsWithoutCalendar(signedIn(t), seasonID, dropID)
	if err != nil {
		t.Fatalf("ModelsWithoutCalendar: %v", err)
	}

	if !slices.Equal(counts, rows.counts) {
		t.Errorf("the count holds %v, want %v", counts, rows.counts)
	}
	if rows.asked.SeasonID != seasonID || rows.asked.DropID != dropID {
		t.Errorf("the store was asked for season %s and drop %s, want %s and %s",
			rows.asked.SeasonID, rows.asked.DropID, seasonID, dropID)
	}
}

func TestModelsWithoutCalendarReportsAFailedRead(t *testing.T) {
	svc, rows, _ := newTestService(t)
	rows.failWith = errShaky

	_, err := svc.ModelsWithoutCalendar(signedIn(t), ids.Nil, ids.Nil)
	if !errors.Is(err, errShaky) {
		t.Errorf("ModelsWithoutCalendar error = %v, want %v", err, errShaky)
	}
}
