package milestones

import (
	"net/http"
	"strings"
	"time"

	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/http/middleware"
	"github.com/paveltessman/pilam/internal/http/milestones/views"
	"github.com/paveltessman/pilam/internal/http/shared"
	"github.com/paveltessman/pilam/internal/milestones"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/logging"
)

// This code holds the milestone list: the late work of a whole season on one
// screen. It is the one milestone screen a member reaches.

// ShowList renders the list: the steps of one season, worst slip first, and the
// count of the models that hold no critical path.
//
// The screen opens on the current season and on the work that runs late or
// comes due. htmx asks for the same URL as the controls.
func ShowList(catalogSvc *catalog.Service, milestoneSvc *milestones.Service) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		page, ok := listPage(w, r, catalogSvc, milestoneSvc)
		if !ok {
			return
		}

		page.Chrome = shared.Chrome(ctx)
		if middleware.IsFragment(ctx) {
			shared.Render(w, r, http.StatusOK, views.MilestoneListResults(page))
			return
		}
		shared.Render(w, r, http.StatusOK, views.MilestoneList(page))
	}
	return handler
}

// listPage reads everything the list shows. It answers 500 itself and reports
// false when the screen cannot be read.
func listPage(
	w http.ResponseWriter,
	r *http.Request,
	catalogSvc *catalog.Service,
	milestoneSvc *milestones.Service,
) (views.MilestoneListPage, bool) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)

	seasons, err := catalogSvc.ListSeasons(ctx)
	if err != nil {
		logger.Error("listing seasons failed", "err", err)
		shared.WriteServerError(w)
		return views.MilestoneListPage{}, false
	}
	// A brand with no season holds no model and no step.
	if len(seasons) == 0 {
		return views.MilestoneListPage{}, true
	}

	season := chosenSeason(r, seasons, milestoneSvc.Today())
	drops, err := catalogSvc.ListDrops(ctx, season.ID)
	if err != nil {
		logger.Error("listing the drops of a season failed", "season", season.ID, "err", err)
		shared.WriteServerError(w)
		return views.MilestoneListPage{}, false
	}

	// The list offers every step, retired ones too: a retired step keeps the
	// rows it already holds, and those rows are on the list.
	types, err := milestoneSvc.ListTypes(ctx)
	if err != nil {
		logger.Error("listing milestone types failed", "err", err)
		shared.WriteServerError(w)
		return views.MilestoneListPage{}, false
	}

	filter := submittedList(r, seasons, season, drops, types)
	rows, err := milestoneSvc.ListMilestones(ctx, milestones.ListParams{
		SeasonID: season.ID,
		DropID:   shared.ParsedID(filter.DropID),
		TypeID:   shared.ParsedID(filter.TypeID),
		States:   views.StatesOf(filter.State),
		Search:   filter.Search,
	})
	if err != nil {
		logger.Error("listing the milestones of a season failed", "season", season.ID, "err", err)
		shared.WriteServerError(w)
		return views.MilestoneListPage{}, false
	}

	// The count follows the season and the drop, which are the scope of the
	// screen.
	counts, err := milestoneSvc.ModelsWithoutCalendar(ctx, season.ID, shared.ParsedID(filter.DropID))
	if err != nil {
		logger.Error("counting the models with no calendar failed", "season", season.ID, "err", err)
		shared.WriteServerError(w)
		return views.MilestoneListPage{}, false
	}

	page := views.MilestoneListPage{
		Filter:     filter,
		Groups:     views.NewMilestoneList(rows, filter.Group),
		NoCalendar: views.NewMilestoneListCounts(counts),
		Rows:       len(rows),
	}
	return page, true
}

// chosenSeason is the season the screen stands on: the one the control names,
// and the current season while it names none.
func chosenSeason(r *http.Request, seasons []catalog.Season, today time.Time) catalog.Season {
	asked := shared.ParsedID(r.URL.Query().Get(views.FieldListSeason))
	for _, season := range seasons {
		if season.ID == asked {
			return season
		}
	}
	return catalog.CurrentSeason(seasons, today)
}

// submittedList reads the controls above the list, and the choices they offer.
//
// A value no control offers is dropped, so the control comes back on the choice
// that keeps every row. A drop of another season is such a value, because the
// season the screen stands on states which drops it offers.
func submittedList(
	r *http.Request,
	seasons []catalog.Season,
	season catalog.Season,
	drops []catalog.Drop,
	types []milestones.Type,
) views.MilestoneListFilter {
	query := r.URL.Query()
	filter := views.MilestoneListFilter{
		SeasonID: season.ID.String(),
		DropID:   query.Get(views.FieldListDrop),
		TypeID:   query.Get(views.FieldListType),
		State:    views.ChosenState(query.Get(views.FieldListState)),
		Group:    views.ChosenGroup(query.Get(views.FieldListGroup)),
		Search:   strings.TrimSpace(query.Get(views.FieldListSearch)),
		Seasons:  seasons,
		Drops:    drops,
		Types:    types,
	}

	if !holdsDrop(drops, shared.ParsedID(filter.DropID)) {
		filter.DropID = ""
	}
	if !holdsType(types, shared.ParsedID(filter.TypeID)) {
		filter.TypeID = ""
	}
	return filter
}

// holdsDrop reports whether the season the screen stands on holds the drop.
func holdsDrop(drops []catalog.Drop, dropID ids.ID) bool {
	for _, drop := range drops {
		if drop.ID == dropID {
			return true
		}
	}
	return false
}

// holdsType reports whether the step is one the brand holds.
func holdsType(types []milestones.Type, typeID ids.ID) bool {
	for _, milestoneType := range types {
		if milestoneType.ID == typeID {
			return true
		}
	}
	return false
}
