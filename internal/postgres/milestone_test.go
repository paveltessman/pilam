package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/milestones"
	"github.com/paveltessman/pilam/internal/platform/date"
	"github.com/paveltessman/pilam/internal/platform/ids"
)

// milestoneRepos is the port of the milestone package over one database. It
// carries the catalog stores as well, because a calendar needs the model and
// the drop that hold it.
type milestoneRepos struct {
	db         *DB
	types      *MilestoneTypes
	templates  *MilestoneTemplates
	milestones *Milestones
	catalog    repos
}

func milestoneDB(t *testing.T) milestoneRepos {
	t.Helper()
	db := openTestDB(t)
	return milestoneRepos{
		db:         db,
		types:      NewMilestoneTypes(db),
		templates:  NewMilestoneTemplates(db),
		milestones: NewMilestones(db),
		catalog: repos{
			db:      db,
			seasons: NewSeasons(db),
			drops:   NewDrops(db),
			models:  NewModels(db),
			photos:  NewPhotos(db),
		},
	}
}

// model writes the whole spine above a calendar and returns the model at the
// end of it. target is the target date of the drop.
func (r milestoneRepos) model(t *testing.T, article, target string) catalog.Model {
	t.Helper()

	season := r.catalog.season(t, "SS26 "+article, "2026-01-05")
	drop := r.catalog.drop(t, season, "Drop 1", target)
	return r.catalog.model(t, drop, article)
}

// milestone writes one milestone of a model and returns it.
func (r milestoneRepos) milestone(t *testing.T, model catalog.Model, typeID ids.ID, plan string) milestones.Milestone {
	t.Helper()

	created := milestones.Milestone{
		ID:       gen.New(),
		ModelID:  model.ID,
		TypeID:   typeID,
		Baseline: date.MustParse(plan),
		Plan:     date.MustParse(plan),
		Active:   true,
	}
	if err := r.milestones.Create(t.Context(), created); err != nil {
		t.Fatalf("Milestones.Create: %v", err)
	}
	return created
}

// milestoneType writes one type and returns it.
func (r milestoneRepos) milestoneType(t *testing.T, name, description string) milestones.Type {
	t.Helper()

	created := milestones.Type{ID: gen.New(), Name: name, Description: description, Active: true}
	if err := r.types.Create(t.Context(), created); err != nil {
		t.Fatalf("MilestoneTypes.Create: %v", err)
	}
	return created
}

// template writes one template and returns it.
func (r milestoneRepos) template(t *testing.T, name, description string) milestones.Template {
	t.Helper()

	created := milestones.Template{ID: gen.New(), Name: name, Description: description, Active: true}
	if err := r.templates.Create(t.Context(), created); err != nil {
		t.Fatalf("MilestoneTemplates.Create: %v", err)
	}
	return created
}

// item appends one step to a template and returns it.
func (r milestoneRepos) item(t *testing.T, templateID, typeID ids.ID, offset, position int) milestones.TemplateItem {
	t.Helper()

	created := milestones.TemplateItem{
		ID:         gen.New(),
		TemplateID: templateID,
		TypeID:     typeID,
		Offset:     offset,
		Position:   position,
	}
	if err := r.templates.AddItem(t.Context(), created); err != nil {
		t.Fatalf("MilestoneTemplates.AddItem: %v", err)
	}
	return created
}

// names is the short name of every type of a list, in order.
func names(types []milestones.Type) []string {
	out := make([]string, len(types))
	for i, milestoneType := range types {
		out[i] = milestoneType.Name
	}
	return out
}

func TestMilestoneWriteRollsBackWithTransaction(t *testing.T) {
	t.Parallel()

	store := milestoneDB(t)

	sentinel := errors.New("the work after the write failed")
	created := milestones.Type{ID: gen.New(), Name: "fit", Description: "Fitting", Active: true}

	err := store.db.InTx(t.Context(), func(ctx context.Context) error {
		if err := store.types.Create(ctx, created); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("InTx error = %v, want %v", err, sentinel)
	}

	if _, err := store.types.ByID(t.Context(), created.ID); !errors.Is(err, milestones.ErrNoType) {
		t.Errorf("ByID error = %v, want %v: the rollback left the row behind", err, milestones.ErrNoType)
	}
}
