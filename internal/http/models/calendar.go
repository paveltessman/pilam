package models

import (
	"net/http"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/http/models/views"
	"github.com/paveltessman/pilam/internal/http/shared"
	"github.com/paveltessman/pilam/internal/milestones"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/logging"
)

// This code holds the calendar section of the model screen: the steps of one
// model, with the three dates and the state of each one.

// calendar is everything the section shows, and what the history of the model
// needs to name the step an entry belongs to.
type calendar struct {
	section views.ModelCalendar

	// steps is the short name of every milestone of the model, keyed by the
	// milestone. The trail of the card reads an entry under it.
	steps map[ids.ID]string

	// ids is every milestone of the model, which is the scope the trail reads
	// beside the model itself.
	ids []ids.ID
}

// scopes is what the history of the model covers: the model, and the steps
// under it.
func (c calendar) scopes(modelID ids.ID) []audit.Scope {
	return []audit.Scope{
		{Entity: audit.EntityModel, IDs: []ids.ID{modelID}},
		{Entity: audit.EntityMilestone, IDs: c.ids},
	}
}

// loadCalendar reads the calendar section of one model.
//
// It answers 500 itself and reports false when the section cannot be read.
func loadCalendar(
	w http.ResponseWriter,
	r *http.Request,
	milestoneSvc *milestones.Service,
	modelID ids.ID,
) (calendar, bool) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)

	rows, err := milestoneSvc.Calendar(ctx, modelID)
	if err != nil {
		logger.Error("listing the calendar of a model failed", "model", modelID, "err", err)
		shared.WriteServerError(w)
		return calendar{}, false
	}

	types, err := milestoneSvc.ListTypes(ctx)
	if err != nil {
		logger.Error("listing milestone types failed", "err", err)
		shared.WriteServerError(w)
		return calendar{}, false
	}

	names := make(map[ids.ID]string, len(types))
	for _, milestoneType := range types {
		names[milestoneType.ID] = milestoneType.Name
	}

	held := calendar{
		section: views.NewModelCalendar(rows, names),
		steps:   make(map[ids.ID]string, len(rows)),
		ids:     make([]ids.ID, len(rows)),
	}
	for i, row := range rows {
		held.steps[row.ID] = names[row.TypeID]
		held.ids[i] = row.ID
	}
	return held, true
}
