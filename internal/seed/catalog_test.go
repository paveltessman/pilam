package seed

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/platform/date"
)

const (
	wantDrops   = 3
	wantDesigns = 60
	wantModels  = 140
)

func TestSeasonPlanIsWellFormed(t *testing.T) {
	articles := make(map[string]bool, plannedDesigns())
	names := make(map[string]bool, len(plan))

	for _, drop := range plan {
		if names[drop.name] {
			t.Errorf("Two drops hold the name %q", drop.name)
		}
		names[drop.name] = true

		if len(drop.designs) == 0 {
			t.Errorf("Drop %q puts nothing on sale", drop.name)
		}

		for _, d := range drop.designs {
			switch {
			case d.article == "":
				t.Errorf("A design of %q carries no article", drop.name)
			case articles[d.article]:
				t.Errorf("Two designs hold the article %q", d.article)
			case d.colors < 1:
				t.Errorf("%s is made in %d colors", d.article, d.colors)
			}
			articles[d.article] = true
		}
	}

	switch {
	case len(plan) != wantDrops:
		t.Errorf("The season holds %d drops, want %d", len(plan), wantDrops)
	case plannedDesigns() != wantDesigns:
		t.Errorf("The season holds %d designs, want %d", plannedDesigns(), wantDesigns)
	case plannedModels() != wantModels:
		t.Errorf("The season makes %d models, want %d", plannedModels(), wantModels)
	}
}

func TestRunWritesTheWholeSeason(t *testing.T) {
	deps, _, rows, _ := newTestDeps(t)

	report := run(t, deps, Options{Users: short})

	got := report.Catalog
	switch {
	case got.Seasons.Created != 1 || got.Seasons.Skipped != 0:
		t.Errorf("Incorrect season count: %+v", got.Seasons)
	case got.Drops.Created != wantDrops || got.Drops.Skipped != 0:
		t.Errorf("Incorrect drop count: %+v", got.Drops)
	case got.Models.Created != wantModels || got.Models.Skipped != 0:
		t.Errorf("Incorrect model count: %+v", got.Models)
	case got.Photos == 0:
		t.Error("The run attached no photo")
	}

	switch {
	case len(rows.seasons) != 1:
		t.Errorf("The store holds %d seasons, want 1", len(rows.seasons))
	case len(rows.drops) != wantDrops:
		t.Errorf("The store holds %d drops, want %d", len(rows.drops), wantDrops)
	case len(rows.models) != wantModels:
		t.Errorf("The store holds %d models, want %d", len(rows.models), wantModels)
	case len(rows.photos) != got.Photos:
		t.Errorf("The store holds %d photos, the report names %d", len(rows.photos), got.Photos)
	}

	season := onlySeason(t, rows)
	if season.Name != seasonName {
		t.Errorf("Incorrect season name: want=%q, got=%q", seasonName, season.Name)
	}
	if !season.Active {
		t.Error("The seeded season is deactivated")
	}
}

func TestSeededModelsHangOffTheOneSeason(t *testing.T) {
	deps, _, rows, _ := newTestDeps(t)

	run(t, deps, Options{Users: short})
	season := onlySeason(t, rows)

	held, err := deps.Catalog.ListModels(t.Context(), catalog.ModelListParams{SeasonID: season.ID})
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(held) != wantModels {
		t.Errorf("The season holds %d models, want %d", len(held), wantModels)
	}

	for _, drop := range rows.drops {
		if drop.SeasonID != season.ID {
			t.Errorf("Drop %q belongs to season %s, want %s", drop.Name, drop.SeasonID, season.ID)
		}
	}

	// The designs of a drop land in that drop.
	for i, drop := range plan {
		row := dropNamed(t, rows, drop.name)
		inDrop, err := deps.Catalog.ListModels(t.Context(), catalog.ModelListParams{DropID: row.ID})
		if err != nil {
			t.Fatalf("ListModels: %v", err)
		}

		want := 0
		for _, d := range drop.designs {
			want += d.colors
		}
		if len(inDrop) != want {
			t.Errorf("Drop %d holds %d models, want %d", i+1, len(inDrop), want)
		}
	}
}

func TestOneDesignBecomesOneModelPerColor(t *testing.T) {
	deps, _, rows, _ := newTestDeps(t)

	run(t, deps, Options{Users: short})

	counts := make(map[string]int, wantDesigns)
	for _, model := range rows.models {
		counts[model.Article]++
		if !model.Active {
			t.Errorf("Model %s is seeded deactivated", model.Article)
		}
	}

	for _, drop := range plan {
		for _, d := range drop.designs {
			if counts[d.article] != d.colors {
				t.Errorf("%s holds %d models, want %d", d.article, counts[d.article], d.colors)
			}
		}
	}
}

func TestSeededDatesAreRelativeToTheClock(t *testing.T) {
	deps, _, rows, _ := newTestDeps(t)

	run(t, deps, Options{Users: short, Models: 1})

	season := onlySeason(t, rows)
	wantStart := date.AddDays(today, seasonStartsIn)
	if !date.Equal(season.StartDate, wantStart) {
		t.Errorf("The season starts on %s, want %s", date.ISO(season.StartDate), date.ISO(wantStart))
	}

	for i, drop := range plan {
		row := dropNamed(t, rows, drop.name)
		want := date.AddDays(today, firstDropIn+i*dropsApart)
		if !date.Equal(row.TargetDate, want) {
			t.Errorf("Drop %q targets %s, want %s", drop.name, date.ISO(row.TargetDate), date.ISO(want))
		}
		if !date.Before(season.StartDate, row.TargetDate) {
			t.Errorf("Drop %q goes on sale before its season opens", drop.name)
		}
	}
}

func TestRunWritesOnlyTheModelsAskedFor(t *testing.T) {
	deps, _, rows, _ := newTestDeps(t)

	report := run(t, deps, Options{Users: short, Models: 5})

	if report.Catalog.Models.Created != 5 || len(rows.models) != 5 {
		t.Errorf("Incorrect count: want=5, created=%d, stored=%d",
			report.Catalog.Models.Created, len(rows.models))
	}
	if len(rows.drops) != wantDrops {
		t.Errorf("A short run wrote %d drops, want %d", len(rows.drops), wantDrops)
	}
}

func TestSecondRunLeavesTheCatalogAsItStands(t *testing.T) {
	deps, _, rows, _ := newTestDeps(t)

	run(t, deps, Options{Users: short, Models: 12})
	first := slices.Collect(maps.Keys(rows.models))
	photos := len(rows.photos)

	report := run(t, deps, Options{Users: short, Models: 12})

	got := report.Catalog
	switch {
	case got.Seasons.Created != 0 || got.Seasons.Skipped != 1:
		t.Errorf("The second run wrote the season again: %+v", got.Seasons)
	case got.Drops.Created != 0 || got.Drops.Skipped != wantDrops:
		t.Errorf("The second run wrote the drops again: %+v", got.Drops)
	case got.Models.Created != 0 || got.Models.Skipped != 12:
		t.Errorf("The second run wrote the models again: %+v", got.Models)
	case got.Photos != 0:
		t.Errorf("The second run attached %d photos again", got.Photos)
	}

	switch {
	case len(rows.models) != 12:
		t.Errorf("The store holds %d models, want 12", len(rows.models))
	case len(rows.photos) != photos:
		t.Errorf("The store holds %d photos, want %d", len(rows.photos), photos)
	case !sameIDs(first, slices.Collect(maps.Keys(rows.models))):
		t.Error("The second run moved the rows of the first")
	}
}

func TestLaterRunWritesTheRestOfTheModels(t *testing.T) {
	deps, _, rows, _ := newTestDeps(t)

	run(t, deps, Options{Users: short, Models: 12})
	first := slices.Collect(maps.Keys(rows.models))

	report := run(t, deps, Options{Users: short, Models: 20})

	switch {
	case report.Catalog.Models.Created != 8:
		t.Errorf("The second run wrote %d models, want 8", report.Catalog.Models.Created)
	case report.Catalog.Models.Skipped != 12:
		t.Errorf("The second run skipped %d models, want 12", report.Catalog.Models.Skipped)
	case len(rows.models) != 20:
		t.Errorf("The store holds %d models, want 20", len(rows.models))
	}

	held := slices.Collect(maps.Keys(rows.models))
	for _, id := range first {
		if !slices.Contains(held, id) {
			t.Errorf("The second run dropped model %s", id)
		}
	}
}

func TestTwoRunsFromEmptyWriteTheSameCatalog(t *testing.T) {
	first, second := catalogRows(t), catalogRows(t)

	if !slices.Equal(first, second) {
		t.Errorf("Two runs from empty wrote different rows:\n%v\n%v", first, second)
	}
}

// catalogRows runs the seed against an empty store and returns what it wrote,
// as one sorted line per row.
func catalogRows(t *testing.T) []string {
	t.Helper()

	deps, _, rows, _ := newTestDeps(t)
	run(t, deps, Options{Users: short, Models: 12})

	var out []string
	for _, season := range rows.seasons {
		out = append(out, "season "+season.ID.String()+" "+season.Name+" "+date.ISO(season.StartDate))
	}
	for _, drop := range rows.drops {
		out = append(out, "drop "+drop.ID.String()+" "+drop.Name+" "+date.ISO(drop.TargetDate))
	}
	for _, model := range rows.models {
		out = append(out, "model "+model.ID.String()+" "+model.Article+" "+model.DropID.String())
	}
	for _, photo := range rows.photos {
		out = append(out, "photo "+photo.ID.String()+" "+string(photo.MediaKey)+" "+photo.ModelID.String())
	}
	slices.Sort(out)
	return out
}

func TestSeededModelsCarryAPhotoStrip(t *testing.T) {
	deps, _, rows, _ := newTestDeps(t)

	run(t, deps, Options{Users: short})

	longest, bare := 0, 0
	for _, model := range rows.models {
		strip, err := deps.Catalog.ListPhotos(t.Context(), model.ID)
		if err != nil {
			t.Fatalf("ListPhotos: %v", err)
		}
		if len(strip) == 0 {
			bare++
			continue
		}
		longest = max(longest, len(strip))

		for position, photo := range strip {
			if photo.Position != position {
				t.Errorf("Model %s holds a photo at position %d, want %d",
					model.Article, photo.Position, position)
			}
			if !photo.MediaKey.Valid() {
				t.Errorf("Model %s holds the malformed key %q", model.Article, photo.MediaKey)
			}
		}
	}

	if longest < 3 {
		t.Errorf("The longest strip holds %d photos, want at least 3", longest)
	}
	if bare == 0 {
		t.Error("Every seeded model carries a photo, so the list never shows the empty thumbnail")
	}
}

func TestSeededPhotosShareTheirBlobs(t *testing.T) {
	deps, _, rows, _ := newTestDeps(t)

	run(t, deps, Options{Users: short})

	blobs := make(map[string]bool, len(rows.photos))
	for _, photo := range rows.photos {
		blobs[string(photo.MediaKey)] = true
	}

	if len(blobs) == 0 {
		t.Fatal("The run stored no blob")
	}
	if len(blobs) >= len(rows.photos) {
		t.Errorf("%d photos stand on %d blobs, want fewer blobs than photos",
			len(rows.photos), len(blobs))
	}
}

func TestEverySeededCatalogWriteLeavesTrailEntry(t *testing.T) {
	deps, users, rows, recorder := newTestDeps(t)

	run(t, deps, Options{Users: short, Models: 12})
	root := storedUser(t, users, roster[rootAt].email(DefaultDomain))

	counts := make(map[string]int)
	for _, entry := range recorder.entries {
		if entry.Entity == audit.EntityUser {
			continue
		}
		if entry.ActorID != root.ID {
			t.Errorf("A %s entry names the actor %s, want the root %s",
				entry.Entity, entry.ActorID, root.ID)
		}
		counts[entry.Entity+" "+entry.Action]++
	}

	want := map[string]int{
		audit.EntitySeason + " " + audit.ActionCreated:   1,
		audit.EntityDrop + " " + audit.ActionCreated:     wantDrops,
		audit.EntityModel + " " + audit.ActionCreated:    12,
		audit.EntityModel + " " + audit.ActionPhotoAdded: len(rows.photos),
	}
	for key, n := range want {
		if counts[key] != n {
			t.Errorf("The trail holds %d %q entries, want %d", counts[key], key, n)
		}
	}
}

func TestRunRefusesAModelCountItCantWorkFrom(t *testing.T) {
	cases := map[string]Options{
		"more than the season holds": {Models: plannedModels() + 1},
		"a negative count":           {Models: -1},
	}

	for name, opts := range cases {
		t.Run(name, func(t *testing.T) {
			deps, _, rows, _ := newTestDeps(t)

			if _, err := Run(t.Context(), deps, opts); err == nil {
				t.Error("Run accepted the options")
			}
			if len(rows.models) != 0 {
				t.Errorf("A refused run still wrote %d models", len(rows.models))
			}
		})
	}
}

func TestRunRefusesDependenciesItCantWorkWithout(t *testing.T) {
	whole, _, _, _ := newTestDeps(t)

	cases := map[string]Deps{
		"no auth service":    {Catalog: whole.Catalog, Media: whole.Media, Clock: whole.Clock},
		"no catalog service": {Auth: whole.Auth, Media: whole.Media, Clock: whole.Clock},
		"no media store":     {Auth: whole.Auth, Catalog: whole.Catalog, Clock: whole.Clock},
		"no clock":           {Auth: whole.Auth, Catalog: whole.Catalog, Media: whole.Media},
	}

	for name, deps := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Run(t.Context(), deps, Options{Users: 1, Models: 1}); err == nil {
				t.Error("Run accepted the dependencies")
			}
		})
	}
}

// A placeholder shows its band on a light panel and on a dark one.
func TestPlaceholderBandShowsOnEveryPanel(t *testing.T) {
	for _, base := range palette {
		band := shade(base)
		if abs(luminance(band)-luminance(base)) < 0x20 {
			t.Errorf("The band on %v is too close to the panel: %v", base, band)
		}
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// onlySeason returns the one season the store holds.
func onlySeason(t *testing.T, rows *catalogStore) catalog.Season {
	t.Helper()

	if len(rows.seasons) != 1 {
		t.Fatalf("The store holds %d seasons, want 1", len(rows.seasons))
	}
	for _, season := range rows.seasons {
		return season
	}
	return catalog.Season{}
}

// dropNamed returns the drop the store holds under that name.
func dropNamed(t *testing.T, rows *catalogStore, name string) catalog.Drop {
	t.Helper()

	for _, drop := range rows.drops {
		if strings.EqualFold(drop.Name, name) {
			return drop
		}
	}
	t.Fatalf("The store holds no drop named %q", name)
	return catalog.Drop{}
}
