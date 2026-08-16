package http

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/platform/clock"
	"github.com/paveltessman/pilam/internal/platform/config"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/logging"
	"github.com/paveltessman/pilam/internal/platform/media"
	"github.com/paveltessman/pilam/internal/platform/session"
)

type stubPinger struct{ err error }

func (p stubPinger) Ping(context.Context) error { return p.err }

// deps is what the composition root would have built, with the database
// swapped for a stub, media in a throwaway directory, and the log discarded.
func deps(t *testing.T) Deps {
	t.Helper()
	d := Deps{
		DB:              stubPinger{},
		Logger:          logging.New(logging.Options{Format: logging.FormatText, Output: io.Discard}),
		IDs:             ids.NewGenerator(),
		Media:           mediaStore(t),
		SessionMgr:      session.New([]byte("test signing key"), time.Hour, clock.New(time.UTC)),
		ResolveIdentity: auth.Resolve,
	}
	return d
}

func mediaStore(t *testing.T) media.Store {
	t.Helper()
	store, err := media.New(config.Media{Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("building the media store: %v", err)
	}
	return store
}

func get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	return getWith(t, deps(t), path)
}

func getWith(t *testing.T, deps Deps, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	NewRouter(deps).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestHealthReportsOKWhenTheDatabaseAnswers(t *testing.T) {
	rec := get(t, "/healthz")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got, want := strings.TrimSpace(rec.Body.String()), `{"status":"ok"}`; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestHealthReportsUnavailableWhenTheDatabaseDoesNot(t *testing.T) {
	broken := deps(t)
	broken.DB = stubPinger{err: errors.New("connection refused")}
	rec := getWith(t, broken, "/healthz")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if body := rec.Body.String(); strings.Contains(body, "connection refused") {
		t.Errorf("body leaks the underlying error: %q", body)
	}
}

func TestHomeRendersStyledPage(t *testing.T) {
	rec := get(t, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`<title>pilam</title>`,
		`/static/css/app.css`,
		// From the vendored templUI button: proves the component pipeline.
		`href="/healthz"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
}

// The stylesheet is embedded at compile time, so a build that skipped
// `make css` cannot reach this test — but a mis-mounted route can.
func TestStylesheetIsServed(t *testing.T) {
	rec := get(t, "/static/css/app.css")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.Len() == 0 {
		t.Error("stylesheet is empty")
	}
}

func TestUnknownPathIs404(t *testing.T) {
	if rec := get(t, "/nope"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
