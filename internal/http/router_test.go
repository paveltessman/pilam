package http_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/http/paths"
	"github.com/paveltessman/pilam/internal/http/testkit"
	"github.com/paveltessman/pilam/internal/platform/labels"
)

func TestHealthReportsOKWhenTheDatabaseAnswers(t *testing.T) {
	rec := testkit.Get(t, "/healthz")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got, want := strings.TrimSpace(rec.Body.String()), `{"status":"ok"}`; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestHealthReportsUnavailableWhenTheDatabaseDoesNot(t *testing.T) {
	broken := testkit.UnhealthyDeps(t, errors.New("connection refused"))
	rec := testkit.GetWith(t, broken, "/healthz")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if body := rec.Body.String(); strings.Contains(body, "connection refused") {
		t.Errorf("body leaks the underlying error: %q", body)
	}
}

func TestPagesAreStyledAndRenderComponents(t *testing.T) {
	rec := testkit.Get(t, paths.Login)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`<title>` + labels.LoginTitle + `</title>`,
		`/static/css/app.css`,
		`type="submit"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
}

// The stylesheet is embedded at compile time, so a build that skipped
// `make css` cannot reach this test — but a mis-mounted route can.
func TestStylesheetIsServed(t *testing.T) {
	rec := testkit.Get(t, "/static/css/app.css")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.Len() == 0 {
		t.Error("stylesheet is empty")
	}
}

func TestUnknownPathIs404(t *testing.T) {
	if rec := testkit.Get(t, "/nope"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
