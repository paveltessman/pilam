package shared

import (
	"context"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/auth"
)

type BaseService struct {
	trail *audit.Trail
}

func NewBaseService(trail *audit.Trail) BaseService {
	if trail == nil {
		panic("BaseService: trail is nil")
	}
	return BaseService{trail}
}

// recordTrail hands the changes to the trail, under the user the request is
// authenticated as.
//
// A context with no identity carries no actor, and the trail refuses the entry.
func (s *BaseService) RecordTrail(ctx context.Context, changes ...audit.Change) error {
	identity, _ := auth.FromContext(ctx)
	return s.trail.Record(ctx, identity.UserID, changes...)
}
