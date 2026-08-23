package testkit

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

// MissingID is an id no row holds, for the screens that answer 404.
var MissingID = ids.MustParse("01912345-6789-7abc-def0-0000000000ff")

// IDOfRedirect reads the id out of the location a write redirects to.
func IDOfRedirect(t *testing.T, rec *httptest.ResponseRecorder, section string) string {
	t.Helper()

	location := rec.Header().Get("Location")
	id, _, _ := strings.Cut(strings.TrimPrefix(location, section+"/"), "?")
	if _, err := ids.Parse(id); err != nil {
		t.Fatalf("the redirect to %q names no id of %s", location, section)
	}
	return id
}

// EntriesOf is what the trail holds about one entity, newest first.
func EntriesOf(t *testing.T, trail *Trail, entity, id string) []audit.Entry {
	t.Helper()

	entityID, err := ids.Parse(id)
	if err != nil {
		t.Fatalf("reading the trail of %s %q: %v", entity, id, err)
	}
	entries, err := trail.ByEntity(t.Context(), entity, entityID, audit.DefaultLimit)
	if err != nil {
		t.Fatalf("reading the trail of %s %s: %v", entity, id, err)
	}
	return entries
}

// ActionsOf is the actions of a trail, newest first.
func ActionsOf(entries []audit.Entry) []string {
	actions := make([]string, len(entries))
	for i, entry := range entries {
		actions[i] = entry.Action
	}
	return actions
}

// Message is the sentence a rejection reads as on the screen.
func Message(code validate.Code) string {
	return labels.Message(validate.FieldError{Code: code})
}

// Wants fails unless the body holds every string given.
func Wants(t *testing.T, body string, texts ...string) {
	t.Helper()

	for _, text := range texts {
		if !strings.Contains(body, text) {
			t.Errorf("the screen does not hold %q", text)
		}
	}
}
