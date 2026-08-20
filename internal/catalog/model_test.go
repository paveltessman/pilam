package catalog

import (
	"errors"
	"slices"
	"testing"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/platform/date"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

func TestCreateModelRefusesADropThatIsNotThere(t *testing.T) {
	svc, _, _ := newTestService(t)

	in := ModelCreateParams{DropID: ids.MustParse("01912345-6789-7abc-def0-000000000003"), Article: "A-100"}
	if _, err := svc.CreateModel(signedIn(t), in); !errors.Is(err, ErrNoDrop) {
		t.Errorf("CreateModel error = %v, want %v", err, ErrNoDrop)
	}
}

func TestCreateModelRefusesAnEmptyArticle(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := signedIn(t)
	_, drop, _ := spine(t, svc, ctx)

	_, err := svc.CreateModel(ctx, ModelCreateParams{DropID: drop.ID, Article: " "})
	rejects(t, err, FieldArticle, validate.Required)
}

func TestCreateModelAllowsTheSameArticleTwice(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := signedIn(t)
	_, drop, first := spine(t, svc, ctx)

	second, err := svc.CreateModel(ctx, ModelCreateParams{DropID: drop.ID, Article: first.Article})
	if err != nil {
		t.Fatalf("CreateModel: %v", err)
	}
	if second.ID == first.ID {
		t.Error("The second model reused the identifier of the first")
	}
}

func TestUpdateModelMovesTheArticleAndTheFlag(t *testing.T) {
	svc, rows, trail := newTestService(t)
	ctx := signedIn(t)
	_, drop, model := spine(t, svc, ctx)
	trail.entries = nil

	if err := svc.UpdateModel(ctx, model.ID, ModelUpdateParams{Article: "A-200", Active: false}); err != nil {
		t.Fatalf("UpdateModel: %v", err)
	}

	got := rows.models[model.ID]
	if got.Article != "A-200" || got.Active {
		t.Errorf("The stored row = %+v, want article A-200 and an inactive flag", got)
	}
	if got.DropID != drop.ID {
		t.Errorf("DropID = %s, want the drop it was created in, %s", got.DropID, drop.ID)
	}

	want := []string{audit.ActionChanged, audit.ActionDeactivated}
	if !slices.Equal(trail.actions(), want) {
		t.Errorf("actions = %v, want %v", trail.actions(), want)
	}
}

func TestListModelsFiltersBySeasonAcrossEveryDropOfIt(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := signedIn(t)
	season, first, _ := spine(t, svc, ctx)

	second, err := svc.CreateDrop(ctx, DropCreateParams{
		SeasonID:   season.ID,
		Name:       "Drop 2",
		TargetDate: date.MustParse("2027-04-15"),
	})
	if err != nil {
		t.Fatalf("CreateDrop: %v", err)
	}
	if _, err := svc.CreateModel(ctx, ModelCreateParams{DropID: second.ID, Article: "A-200"}); err != nil {
		t.Fatalf("CreateModel: %v", err)
	}

	// One model in another season, which the filter must leave out.
	other, err := svc.CreateSeason(ctx, SeasonCreateParams{Name: "AW27", StartDate: date.MustParse("2027-05-01")})
	if err != nil {
		t.Fatalf("CreateSeason: %v", err)
	}
	elsewhere, err := svc.CreateDrop(ctx, DropCreateParams{
		SeasonID:   other.ID,
		Name:       "Drop 1",
		TargetDate: date.MustParse("2027-08-15"),
	})
	if err != nil {
		t.Fatalf("CreateDrop: %v", err)
	}
	if _, err := svc.CreateModel(ctx, ModelCreateParams{DropID: elsewhere.ID, Article: "B-100"}); err != nil {
		t.Fatalf("CreateModel: %v", err)
	}

	models, err := svc.ListModels(ctx, ModelListParams{SeasonID: season.ID})
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}

	articles := make([]string, len(models))
	for i, model := range models {
		articles[i] = model.Article
	}
	if want := []string{"A-100", "A-200"}; !slices.Equal(articles, want) {
		t.Errorf("articles = %v, want %v: the season filter must reach every drop of the season", articles, want)
	}

	byDrop, err := svc.ListModels(ctx, ModelListParams{DropID: first.ID})
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(byDrop) != 1 || byDrop[0].Article != "A-100" {
		t.Errorf("The drop filter returned %d models, want the one of drop %s", len(byDrop), first.ID)
	}
}

func TestListModelsFiltersByTheActiveFlag(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := signedIn(t)
	_, drop, model := spine(t, svc, ctx)

	if _, err := svc.CreateModel(ctx, ModelCreateParams{DropID: drop.ID, Article: "A-200"}); err != nil {
		t.Fatalf("CreateModel: %v", err)
	}
	if err := svc.UpdateModel(ctx, model.ID, ModelUpdateParams{Article: model.Article, Active: false}); err != nil {
		t.Fatalf("UpdateModel: %v", err)
	}

	on, off := true, false
	for _, tc := range []struct {
		name   string
		active *bool
		want   int
	}{
		{"both", nil, 2},
		{"active", &on, 1},
		{"inactive", &off, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			models, err := svc.ListModels(ctx, ModelListParams{Active: tc.active})
			if err != nil {
				t.Fatalf("ListModels: %v", err)
			}
			if len(models) != tc.want {
				t.Errorf("ListModels returned %d models, want %d", len(models), tc.want)
			}
		})
	}
}
