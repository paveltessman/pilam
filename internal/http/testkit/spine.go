package testkit

import (
	"net/http"
	"net/url"
	"testing"

	dropviews "github.com/paveltessman/pilam/internal/http/drops/views"
	"github.com/paveltessman/pilam/internal/http/paths"
	seasonviews "github.com/paveltessman/pilam/internal/http/seasons/views"
)

// The catalog spine: a model needs a drop, and a drop needs a season. A test of
// any screen below the season builds the rows above it through these calls.

// SeasonForm is what the season create and edit screen post.
func SeasonForm(name, start string, active bool) url.Values {
	form := url.Values{
		seasonviews.FieldSeasonName:  {name},
		seasonviews.FieldSeasonStart: {start},
	}
	if active {
		form.Set(seasonviews.FieldSeasonActive, "true")
	}
	return form
}

// CreatedSeason posts the create form and returns the id of the season it wrote.
func CreatedSeason(t *testing.T, d Deps, cookie *http.Cookie, name, start string) string {
	t.Helper()

	rec := PostAs(t, d, paths.Seasons, SeasonForm(name, start, true), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("creating season %q: status = %d, want %d: %s", name, rec.Code, http.StatusSeeOther, rec.Body)
	}
	return IDOfRedirect(t, rec, paths.Seasons)
}

// DropForm is what the drop create and edit screen post.
func DropForm(seasonID, name, target string, active bool) url.Values {
	form := url.Values{
		dropviews.FieldDropSeason: {seasonID},
		dropviews.FieldDropName:   {name},
		dropviews.FieldDropTarget: {target},
	}
	if active {
		form.Set(dropviews.FieldDropActive, "true")
	}
	return form
}

// CreatedDrop posts the create form and returns the id of the drop it wrote.
func CreatedDrop(t *testing.T, d Deps, cookie *http.Cookie, seasonID, name, target string) string {
	t.Helper()

	rec := PostAs(t, d, paths.Drops, DropForm(seasonID, name, target, true), cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("creating drop %q: status = %d, want %d: %s", name, rec.Code, http.StatusSeeOther, rec.Body)
	}
	return IDOfRedirect(t, rec, paths.Drops)
}
