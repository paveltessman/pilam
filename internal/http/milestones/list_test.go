package milestones_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/http/milestones/views"
	"github.com/paveltessman/pilam/internal/http/paths"
	"github.com/paveltessman/pilam/internal/http/testkit"
	"github.com/paveltessman/pilam/internal/milestones"
	"github.com/paveltessman/pilam/internal/platform/date"
	"github.com/paveltessman/pilam/internal/platform/labels"
)

// The day the fixed clock stands on, and the days around it a state is read
// against.
var (
	today     = date.Of(testkit.Now.Date())
	yesterday = date.ISO(date.AddDays(today, -1))
	nextWeek  = date.ISO(date.AddDays(today, milestones.DueWindow))
	nextMonth = date.ISO(date.AddDays(today, 30))
)

// listFixture is the season the list reads: two drops, two step types, and the
// models a test hangs steps on.
type listFixture struct {
	cookie   *http.Cookie
	seasonID string
	first    string
	second   string
	fit      string
	shipped  string
}

// newListFixture writes the season the list opens on. Only a root reaches the
// sections above a model, so it logs one in and every call uses it.
func newListFixture(t *testing.T, d testkit.Deps) listFixture {
	t.Helper()

	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)
	seasonID := testkit.CreatedSeason(t, d, cookie, "SS26", "2026-01-05")
	held := listFixture{
		cookie:   cookie,
		seasonID: seasonID,
		first:    testkit.CreatedDrop(t, d, cookie, seasonID, "Drop 1", "2026-07-16"),
		second:   testkit.CreatedDrop(t, d, cookie, seasonID, "Drop 2", "2026-09-01"),
		fit:      testkit.CreatedMilestoneType(t, d, cookie, "FIT", "Fit approved"),
		shipped:  testkit.CreatedMilestoneType(t, d, cookie, "SHIP", "Shipped"),
	}
	return held
}

// step writes one model in a drop and hangs one step on it. It returns the
// identifier of the model.
func (f listFixture) step(t *testing.T, d testkit.Deps, dropID, article, typeID, plan string) string {
	t.Helper()

	modelID := testkit.CreatedModel(t, d, f.cookie, dropID, article)
	testkit.AddedMilestone(t, d, modelID, typeID, plan)
	return modelID
}

// list reads the screen and returns the body of it.
func (f listFixture) list(t *testing.T, d testkit.Deps, query url.Values) string {
	t.Helper()

	path := paths.MilestoneList
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	rec := testkit.GetAs(t, d, path, f.cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: status = %d, want %d", path, rec.Code, http.StatusOK)
	}
	return rec.Body.String()
}

// results is the list alone, which is what htmx asks for. A test that reads a
// value the controls also carry reads this, so the assertion cannot pass on the
// controls above the table.
func (f listFixture) results(t *testing.T, d testkit.Deps, query url.Values) string {
	t.Helper()

	path := paths.MilestoneList
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	rec := testkit.GetAsHTMX(t, d, path, f.cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: status = %d, want %d", path, rec.Code, http.StatusOK)
	}
	return rec.Body.String()
}

// holds fails unless the list shows every article named, and no other.
func holds(t *testing.T, body string, shown []string, hidden ...string) {
	t.Helper()

	for _, article := range shown {
		if !strings.Contains(body, article) {
			t.Errorf("the list does not hold %q", article)
		}
	}
	for _, article := range hidden {
		if strings.Contains(body, article) {
			t.Errorf("the list holds %q, and it should not", article)
		}
	}
}

// The screen goal of the plan: a plan date that passed yesterday, with no fact
// date, reads as late this morning.
func TestMilestoneListShowsAStepWhosePlanDatePassed(t *testing.T) {
	d := testkit.NewDeps(t)
	held := newListFixture(t, d)
	held.step(t, d, held.first, "LATE-1", held.fit, yesterday)

	// The list alone, so the row carries the step and the drop rather than the
	// controls above the table.
	body := held.results(t, d, nil)

	testkit.Wants(t, body, "LATE-1", "FIT", "Drop 1", labels.MilestonesStateLate)
}

func TestMilestoneListOpensOnTheWorkThatIsLateOrDue(t *testing.T) {
	d := testkit.NewDeps(t)
	held := newListFixture(t, d)
	held.step(t, d, held.first, "LATE-1", held.fit, yesterday)
	held.step(t, d, held.first, "DUE-1", held.shipped, nextWeek)
	held.step(t, d, held.second, "PLANNED-1", held.fit, nextMonth)

	body := held.list(t, d, nil)
	holds(t, body, []string{"LATE-1", "DUE-1"}, "PLANNED-1")

	// The state control reaches the rest of the season.
	body = held.list(t, d, url.Values{views.FieldListState: {views.StateAny}})
	holds(t, body, []string{"LATE-1", "DUE-1", "PLANNED-1"})
}

func TestMilestoneListLeavesOutAStepThatIsDone(t *testing.T) {
	d := testkit.NewDeps(t)
	held := newListFixture(t, d)
	modelID := held.step(t, d, held.first, "DONE-1", held.fit, yesterday)

	// The step reads as late until somebody stamps the fact date on it.
	holds(t, held.list(t, d, nil), []string{"DONE-1"})

	stamp(t, d, modelID, yesterday)
	holds(t, held.list(t, d, nil), nil, "DONE-1")

	body := held.results(t, d, url.Values{views.FieldListState: {views.StateDone}})
	testkit.Wants(t, body, "DONE-1", labels.MilestonesStateDone)
}

func TestMilestoneListNarrowsByTheControlsAboveIt(t *testing.T) {
	d := testkit.NewDeps(t)
	held := newListFixture(t, d)
	held.step(t, d, held.first, "ART-100", held.fit, yesterday)
	held.step(t, d, held.second, "ART-200", held.shipped, yesterday)
	held.step(t, d, held.second, "SKI-300", held.fit, yesterday)

	testData := map[string]struct {
		query  url.Values
		shown  []string
		hidden []string
	}{
		"the whole season": {nil, []string{"ART-100", "ART-200", "SKI-300"}, nil},
		"one drop": {
			url.Values{views.FieldListDrop: {held.second}},
			[]string{"ART-200", "SKI-300"}, []string{"ART-100"},
		},
		"one step": {
			url.Values{views.FieldListType: {held.shipped}},
			[]string{"ART-200"}, []string{"ART-100", "SKI-300"},
		},
		"the search": {
			url.Values{views.FieldListSearch: {"art"}},
			[]string{"ART-100", "ART-200"}, []string{"SKI-300"},
		},
		"a drop and a step": {
			url.Values{views.FieldListDrop: {held.first}, views.FieldListType: {held.shipped}},
			nil, []string{"ART-100", "ART-200", "SKI-300"},
		},
	}
	for name, tc := range testData {
		t.Run(name, func(t *testing.T) {
			holds(t, held.list(t, d, tc.query), tc.shown, tc.hidden...)
		})
	}
}

func TestMilestoneListOrdersWorstSlipFirst(t *testing.T) {
	d := testkit.NewDeps(t)
	held := newListFixture(t, d)

	// Every step starts with the plan on the baseline. Moving the plan later is
	// what makes the slip, so each of these moves by a different number of days.
	moved(t, d, held.step(t, d, held.first, "MOVED-3", held.fit, yesterday), 3)
	moved(t, d, held.step(t, d, held.first, "MOVED-21", held.shipped, yesterday), 21)
	moved(t, d, held.step(t, d, held.second, "MOVED-10", held.fit, yesterday), 10)

	body := held.list(t, d, url.Values{views.FieldListState: {views.StateAny}})

	want := []string{"MOVED-21", "MOVED-10", "MOVED-3"}
	at := make([]int, len(want))
	for i, article := range want {
		if at[i] = strings.Index(body, article); at[i] < 0 {
			t.Fatalf("the list does not hold %q", article)
		}
	}
	if at[0] > at[1] || at[1] > at[2] {
		t.Errorf("the list holds the rows out of order, want %v", want)
	}
}

func TestMilestoneListGroupsByDropAndByStep(t *testing.T) {
	d := testkit.NewDeps(t)
	held := newListFixture(t, d)
	held.step(t, d, held.first, "ART-100", held.fit, yesterday)
	held.step(t, d, held.second, "ART-200", held.shipped, yesterday)

	// A grouped list holds one table per group, so it holds one heading per
	// group over the rows of it.
	body := held.list(t, d, url.Values{views.FieldListGroup: {views.GroupDrop}})
	holds(t, body, []string{"ART-100", "ART-200"})
	if headings := strings.Count(body, "<h2"); headings < 2 {
		t.Errorf("the list grouped by drop holds %d headings, want one per drop", headings)
	}

	body = held.list(t, d, url.Values{views.FieldListGroup: {views.GroupType}})
	holds(t, body, []string{"ART-100", "ART-200"})
	if headings := strings.Count(body, "<h2"); headings < 2 {
		t.Errorf("the list grouped by step holds %d headings, want one per step", headings)
	}

	// A list that is not grouped holds no heading over the table at all.
	if headings := strings.Count(held.list(t, d, nil), "<h2"); headings != 0 {
		t.Errorf("the plain list holds %d headings, want none", headings)
	}
}

func TestMilestoneListRowLinksToTheCardOfItsModel(t *testing.T) {
	d := testkit.NewDeps(t)
	held := newListFixture(t, d)
	modelID := held.step(t, d, held.first, "ART-100", held.fit, yesterday)

	testkit.Wants(t, held.list(t, d, nil), `href="`+paths.Models+"/"+modelID+`"`)
}

func TestMilestoneListCountsTheModelsThatHoldNoCalendar(t *testing.T) {
	d := testkit.NewDeps(t)
	held := newListFixture(t, d)
	held.step(t, d, held.first, "ART-100", held.fit, yesterday)

	// Two models of the second drop, and neither of them holds a step.
	testkit.CreatedModel(t, d, held.cookie, held.second, "NO-CAL-1")
	testkit.CreatedModel(t, d, held.cookie, held.second, "NO-CAL-2")

	body := held.results(t, d, nil)
	testkit.Wants(t, body, labels.MilestoneListNoCalendar, "Drop 2", labels.Styles(2))

	// The count follows the drop the screen stands on.
	body = held.list(t, d, url.Values{views.FieldListDrop: {held.first}})
	if strings.Contains(body, labels.MilestoneListNoCalendar) {
		t.Error("the count names a drop the screen does not stand on")
	}
}

func TestMilestoneListOpensOnTheCurrentSeason(t *testing.T) {
	d := testkit.NewDeps(t)
	held := newListFixture(t, d)
	held.step(t, d, held.first, "NOW-1", held.fit, yesterday)

	// A season that has not started yet. The list opens on the season that has.
	later := testkit.CreatedSeason(t, d, held.cookie, "SS27", "2027-01-05")
	laterDrop := testkit.CreatedDrop(t, d, held.cookie, later, "Drop 1", "2027-07-16")
	held.step(t, d, laterDrop, "LATER-1", held.fit, yesterday)

	holds(t, held.list(t, d, nil), []string{"NOW-1"}, "LATER-1")

	body := held.list(t, d, url.Values{views.FieldListSeason: {later}})
	holds(t, body, []string{"LATER-1"}, "NOW-1")
}

func TestMilestoneListSaysWhenTheFilterKeepsNothing(t *testing.T) {
	d := testkit.NewDeps(t)
	held := newListFixture(t, d)
	held.step(t, d, held.first, "ART-100", held.fit, yesterday)

	body := held.list(t, d, url.Values{views.FieldListSearch: {"nothing matches this"}})

	testkit.Wants(t, body, labels.MilestoneListNoMatch)
	if strings.Contains(body, labels.MilestoneListEmpty) {
		t.Error("a filter that keeps nothing claims the season holds no step")
	}
}

func TestMilestoneListSaysWhenTheSeasonHoldsNoStep(t *testing.T) {
	d := testkit.NewDeps(t)
	held := newListFixture(t, d)

	testkit.Wants(t, held.list(t, d, nil), labels.MilestoneListEmpty)
}

func TestMilestoneListSaysWhenThereIsNoSeasonAtAll(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	rec := testkit.GetAs(t, d, paths.MilestoneList, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	testkit.Wants(t, rec.Body.String(), labels.MilestoneListNoSeason)
}

// The list is the one milestone screen a member reaches, and the nav offers it
// to every user.
func TestMilestoneListReachesAMember(t *testing.T) {
	d := testkit.NewDeps(t)
	held := newListFixture(t, d)
	held.step(t, d, held.first, "ART-100", held.fit, yesterday)

	member := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)
	rec := testkit.GetAs(t, d, paths.MilestoneList, member)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	testkit.Wants(t, rec.Body.String(), "ART-100")

	// The nav offers the list, and it still keeps the admin section back.
	models := testkit.GetAs(t, d, paths.Models, member).Body.String()
	testkit.Wants(t, models, `href="`+paths.MilestoneList+`"`)
	if strings.Contains(models, `href="`+paths.Milestones+`"`) {
		t.Errorf("a member is offered %s", paths.Milestones)
	}
}

func TestMilestoneListNeedsLogin(t *testing.T) {
	d := testkit.NewDeps(t)

	rec := testkit.GetWith(t, d, paths.MilestoneList)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != paths.Login {
		t.Errorf("Location = %q, want %q", got, paths.Login)
	}
}

// htmx asks for the same URL as the controls, and it gets the list alone: the
// rest of the screen already stands in the browser.
func TestMilestoneListAnswersHTMXWithTheListAlone(t *testing.T) {
	d := testkit.NewDeps(t)
	held := newListFixture(t, d)
	held.step(t, d, held.first, "ART-100", held.fit, yesterday)

	rec := testkit.GetAsHTMX(t, d, paths.MilestoneList, held.cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	testkit.Wants(t, body, "ART-100")
	if strings.Contains(body, "<form") || strings.Contains(body, labels.MilestoneListHint) {
		t.Error("the fragment carries the whole screen, want the list alone")
	}
}

// A value no control offers comes back as the choice the list opens on, so a
// hand-written query string cannot put the screen into a state it does not
// offer.
func TestMilestoneListDropsAValueNoControlOffers(t *testing.T) {
	d := testkit.NewDeps(t)
	held := newListFixture(t, d)
	held.step(t, d, held.first, "LATE-1", held.fit, yesterday)
	held.step(t, d, held.first, "PLANNED-1", held.shipped, nextMonth)

	query := url.Values{
		views.FieldListState: {"not a state"},
		views.FieldListGroup: {"not a grouping"},
		views.FieldListDrop:  {testkit.MissingID.String()},
		views.FieldListType:  {testkit.MissingID.String()},
	}
	body := held.list(t, d, query)

	holds(t, body, []string{"LATE-1"}, "PLANNED-1")
	if strings.Count(body, "<h2") != 0 {
		t.Error("a grouping no control offers grouped the list")
	}
}

// stamp writes today's fact date on the one step of a model.
func stamp(t *testing.T, d testkit.Deps, modelID, fact string) {
	t.Helper()

	held, err := d.MilestoneSvc.Calendar(testkit.ActorContext(t, testkit.RootUserID), testkit.ParsedID(t, modelID))
	if err != nil {
		t.Fatalf("reading the calendar of model %s: %v", modelID, err)
	}
	if len(held) != 1 {
		t.Fatalf("model %s holds %d steps, want 1", modelID, len(held))
	}
	testkit.EditedMilestone(t, d, held[0].Milestone, milestones.MilestoneUpdateParams{
		Fact:   date.MustParse(fact),
		Active: true,
	})
}

// moved pushes the plan date of the one step of a model by days, which is what
// makes the slip the list orders on.
func moved(t *testing.T, d testkit.Deps, modelID string, days int) {
	t.Helper()

	held, err := d.MilestoneSvc.Calendar(testkit.ActorContext(t, testkit.RootUserID), testkit.ParsedID(t, modelID))
	if err != nil {
		t.Fatalf("reading the calendar of model %s: %v", modelID, err)
	}
	if len(held) != 1 {
		t.Fatalf("model %s holds %d steps, want 1", modelID, len(held))
	}
	testkit.MovedPlan(t, d, held[0].Milestone, date.AddDays(held[0].Plan, days))
}
