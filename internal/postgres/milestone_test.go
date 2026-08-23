package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/paveltessman/pilam/internal/milestone"
)

// milestoneRepos is the port of the milestone package over one database.
type milestoneRepos struct {
	db    *DB
	types *MilestoneTypes
}

func milestoneDB(t *testing.T) milestoneRepos {
	t.Helper()
	db := openTestDB(t)
	return milestoneRepos{db: db, types: NewMilestoneTypes(db)}
}

// milestoneType writes one type and returns it.
func (r milestoneRepos) milestoneType(t *testing.T, name, description string) milestone.Type {
	t.Helper()

	created := milestone.Type{ID: gen.New(), Name: name, Description: description, Active: true}
	if err := r.types.Create(t.Context(), created); err != nil {
		t.Fatalf("MilestoneTypes.Create: %v", err)
	}
	return created
}

// names is the short name of every type of a list, in order.
func names(types []milestone.Type) []string {
	out := make([]string, len(types))
	for i, milestoneType := range types {
		out[i] = milestoneType.Name
	}
	return out
}

func TestMilestoneWriteRollsBackWithTransaction(t *testing.T) {
	store := milestoneDB(t)

	sentinel := errors.New("the work after the write failed")
	created := milestone.Type{ID: gen.New(), Name: "fit", Description: "Fitting", Active: true}

	err := store.db.InTx(t.Context(), func(ctx context.Context) error {
		if err := store.types.Create(ctx, created); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("InTx error = %v, want %v", err, sentinel)
	}

	if _, err := store.types.ByID(t.Context(), created.ID); !errors.Is(err, milestone.ErrNoType) {
		t.Errorf("ByID error = %v, want %v: the rollback left the row behind", err, milestone.ErrNoType)
	}
}
