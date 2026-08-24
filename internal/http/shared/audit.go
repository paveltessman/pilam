package shared

import (
	"net/http"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/http/shared/views"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/logging"
)

// LoadTrail reads the audit trail of one record and names the actor of every
// entry. Every card that carries a history reads it through here.
//
// It answers 500 itself and reports false when the trail cannot be read.
func LoadTrail(
	w http.ResponseWriter,
	r *http.Request,
	authSvc *auth.Service,
	log *audit.Log,
	entity string,
	entityID ids.ID,
) ([]views.AuditEvent, bool) {
	return LoadTrailOf(w, r, authSvc, log, nil, audit.Scope{Entity: entity, IDs: []ids.ID{entityID}})
}

// LoadTrailOf reads the histories of several sets of rows as one trail, newest
// first, and names the actor of every entry.
//
// A card that carries the rows under it reads them this way. The model screen
// reads the model and its milestones together, because §9 of the design shows
// the two histories as one.
//
// names is what an entry is shown under, keyed by the row it belongs to. A row
// that names nothing carries no subject, which is how the card reads its own
// entries.
//
// It answers 500 itself and reports false when the trail cannot be read.
func LoadTrailOf(
	w http.ResponseWriter,
	r *http.Request,
	authSvc *auth.Service,
	log *audit.Log,
	names map[ids.ID]string,
	scopes ...audit.Scope,
) ([]views.AuditEvent, bool) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)

	entries, err := log.Scopes(ctx, audit.DefaultLimit, scopes...)
	if err != nil {
		logger.Error("loading the audit trail failed", "err", err)
		WriteServerError(w)
		return nil, false
	}

	actorIDs := make([]ids.ID, len(entries))
	for i, entry := range entries {
		actorIDs[i] = entry.ActorID
	}

	actors, err := authSvc.Actors(ctx, actorIDs...)
	if err != nil {
		logger.Error("naming the actors of the audit trail failed", "err", err)
		WriteServerError(w)
		return nil, false
	}

	events := make([]views.AuditEvent, len(entries))
	for i, entry := range entries {
		events[i] = views.AuditEvent{
			Entry:   entry,
			Actor:   actors[entry.ActorID],
			Subject: names[entry.EntityID],
		}
	}
	return events, true
}
