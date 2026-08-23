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
	ctx := r.Context()
	logger := logging.FromContext(ctx)

	entries, err := log.Entity(ctx, entity, entityID, audit.DefaultLimit)
	if err != nil {
		logger.Error("loading the audit trail failed", "entity", entity, "id", entityID, "err", err)
		WriteServerError(w)
		return nil, false
	}

	actorIDs := make([]ids.ID, len(entries))
	for i, entry := range entries {
		actorIDs[i] = entry.ActorID
	}

	actors, err := authSvc.Actors(ctx, actorIDs...)
	if err != nil {
		logger.Error("naming the actors of the audit trail failed", "entity", entity, "id", entityID, "err", err)
		WriteServerError(w)
		return nil, false
	}

	events := make([]views.AuditEvent, len(entries))
	for i, entry := range entries {
		events[i] = views.AuditEvent{Entry: entry, Actor: actors[entry.ActorID]}
	}
	return events, true
}
