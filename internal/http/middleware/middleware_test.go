package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/platform/logging"
)

// Links used without the logger middleware fall back to slog's default, and
// Recover's fallback is a full stack trace. Discard it: the tests that care
// about what was logged install a logger of their own and read it back.
func TestMain(m *testing.M) {
	slog.SetDefault(logging.New(logging.Options{Format: logging.FormatText, Output: io.Discard}))
	os.Exit(m.Run())
}

// tag returns a middleware that appends name to order on the way in and
// name+"-out" on the way out, so a test can assert both directions at once.
func tag(order *[]string, name string) Middleware {
	middleware := func(next http.Handler) http.Handler {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*order = append(*order, name)
			next.ServeHTTP(w, r)
			*order = append(*order, name+"-out")
		})
		return handler
	}
	return middleware
}

func TestChainRunsOutermostFirstAndUnwindsInReverse(t *testing.T) {
	var order []string

	middleware := Chain(
		tag(&order, "a"),
		tag(&order, "b"),
		tag(&order, "c"),
	)
	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		order = append(order, "handler")
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	got := strings.Join(order, " ")
	want := "a b c handler c-out b-out a-out"
	if got != want {
		t.Errorf("order = %q, want %q", got, want)
	}
}

func TestChainOfNothingIsTransparent(t *testing.T) {
	called := false
	handler := Chain()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if !called {
		t.Error("the wrapped handler did not run")
	}
}
