package catalog

import (
	"errors"
	"testing"

	"github.com/paveltessman/pilam/internal/platform/date"
)

// A catalog write happens on behalf of somebody. A context with no identity
// carries no actor, the trail refuses the entry, and the transaction the write
// runs in rolls back. The rollback itself is proven against Postgres, in
// internal/postgres.
func TestAWriteWithNoActorIsRefused(t *testing.T) {
	svc, _, trail := newTestService(t)

	_, err := svc.CreateSeason(t.Context(), SeasonCreateParams{Name: "S1", StartDate: date.MustParse("2026-11-01")})
	if err == nil {
		t.Fatal("CreateSeason wrote a season with no actor")
	}
	if len(trail.entries) != 0 {
		t.Errorf("The trail holds %d entries, want 0", len(trail.entries))
	}
}

func TestFailingWriteRecordsNothing(t *testing.T) {
	svc, rows, trail := newTestService(t)
	ctx := signedIn(t)

	rows.failWith = errors.New("the store is down")
	if _, err := svc.CreateSeason(ctx, SeasonCreateParams{Name: "S1", StartDate: date.MustParse("2026-11-01")}); err == nil {
		t.Fatal("CreateSeason returned no error over a failing store")
	}
	if len(trail.entries) != 0 {
		t.Errorf("The trail holds %d entries, want 0", len(trail.entries))
	}
}
