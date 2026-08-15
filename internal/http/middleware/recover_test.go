package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/labels"
)

func TestRecoverTurnsPanicInto500(t *testing.T) {
	rec := serve(Recover(), httptest.NewRequest(http.MethodGet, "/", nil),
		func(http.ResponseWriter, *http.Request) { panic("boom") })

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got := rec.Body.String(); !strings.Contains(got, labels.ErrorUnexpected) {
		t.Errorf("body = %q, want it to contain %q", got, labels.ErrorUnexpected)
	}
}

func TestRecoverDoesNotLeakPanicToClient(t *testing.T) {
	rec := serve(Recover(), httptest.NewRequest(http.MethodGet, "/", nil),
		func(http.ResponseWriter, *http.Request) { panic("secret internal detail") })

	if got := rec.Body.String(); strings.Contains(got, "secret internal detail") {
		t.Errorf("body leaks the panic value: %q", got)
	}
}

func TestRecoverRespondsInPlainText(t *testing.T) {
	rec := serve(Recover(), httptest.NewRequest(http.MethodGet, "/", nil),
		func(http.ResponseWriter, *http.Request) { panic("boom") })

	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", got)
	}
}

func TestRecoverQuotesTheRequestID(t *testing.T) {
	want := ids.NewDeterministic(3).New().String()

	rec := httptest.NewRecorder()
	Chain(RequestID(ids.NewDeterministic(3)), Recover())(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }),
	).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := rec.Body.String(); !strings.Contains(got, want) {
		t.Errorf("body = %q, want it to quote request id %s", got, want)
	}
}

func TestRecoverLogsPanicWithStack(t *testing.T) {
	logger, buf := capture()

	rec := httptest.NewRecorder()
	Chain(Logger(logger), Recover())(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }),
	).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	var found map[string]any
	for _, record := range records(t, buf) {
		if record["msg"] == "panic recovered" {
			found = record
		}
	}
	if found == nil {
		t.Fatalf("nothing logged the panic: %s", buf)
	}
	if got, ok := found["stack"].(string); !ok || !strings.Contains(got, "recover_test.go") {
		t.Errorf("stack = %q, want it to name the panicking file", found["stack"])
	}
	if found["level"] != slog.LevelError.String() {
		t.Errorf("level = %v, want ERROR", found["level"])
	}
}

func TestRecoverLeavesPartlyWrittenResponseAlone(t *testing.T) {
	rec := serve(Recover(), httptest.NewRequest(http.MethodGet, "/", nil),
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html>half a page"))
			panic("boom")
		})

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want the %d already sent", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != "<html>half a page" {
		t.Errorf("body = %q, want it untouched", got)
	}
}

func TestRecoverRepanicsOnErrAbortHandler(t *testing.T) {
	defer func() {
		if v := recover(); v != http.ErrAbortHandler {
			t.Errorf("recovered %v, want it to propagate %v", v, http.ErrAbortHandler)
		}
	}()

	serve(Recover(), httptest.NewRequest(http.MethodGet, "/", nil),
		func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) })
}

func TestRecoverIsTransparentWhenNothingPanics(t *testing.T) {
	rec := serve(Recover(), httptest.NewRequest(http.MethodGet, "/", nil),
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTeapot)
			_, _ = w.Write([]byte("fine"))
		})

	if rec.Code != http.StatusTeapot || rec.Body.String() != "fine" {
		t.Errorf("status/body = %d/%q, want %d/%q", rec.Code, rec.Body, http.StatusTeapot, "fine")
	}
}

// Recover reaches for the context logger, which the logger middleware installs.
// Used on its own it must still work, falling back to slog's default.
func TestRecoverWorksWithoutTheLoggerMiddleware(t *testing.T) {
	rec := serve(Recover(), httptest.NewRequest(http.MethodGet, "/", nil),
		func(http.ResponseWriter, *http.Request) { panic("boom") })

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}
