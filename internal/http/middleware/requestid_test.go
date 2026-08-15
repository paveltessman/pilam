package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/paveltessman/pilam/internal/platform/ids"
)

// serve runs one request through mw and returns what came back, along with
// whatever the handler recorded from the context.
func serve(mw Middleware, r *http.Request, handler http.HandlerFunc) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	mw(handler).ServeHTTP(rec, r)
	return rec
}

func TestRequestIDIsGeneratedAndEchoed(t *testing.T) {
	gen := ids.NewDeterministic(1)
	want := ids.NewDeterministic(1).New().String()

	var seen string
	rec := serve(RequestID(gen), httptest.NewRequest(http.MethodGet, "/", nil),
		func(_ http.ResponseWriter, r *http.Request) {
			seen = RequestIDFromContext(r.Context())
		})

	if seen != want {
		t.Errorf("context id = %q, want %q", seen, want)
	}
	if got := rec.Header().Get(RequestIDHeader); got != want {
		t.Errorf("%s = %q, want %q", RequestIDHeader, got, want)
	}
}

func TestRequestIDIgnoresTheInboundHeader(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(RequestIDHeader, "chosen-by-the-client")

	var seen string
	serve(RequestID(ids.NewGenerator()), r, func(_ http.ResponseWriter, r *http.Request) {
		seen = RequestIDFromContext(r.Context())
	})

	if seen == "chosen-by-the-client" {
		t.Error("the inbound header was honoured")
	}
	if seen == "" {
		t.Error("no id was generated")
	}
}

func TestRequestIDsDifferBetweenRequests(t *testing.T) {
	mw := RequestID(ids.NewGenerator())

	var first, second string
	serve(mw, httptest.NewRequest(http.MethodGet, "/", nil), func(_ http.ResponseWriter, r *http.Request) {
		first = RequestIDFromContext(r.Context())
	})
	serve(mw, httptest.NewRequest(http.MethodGet, "/", nil), func(_ http.ResponseWriter, r *http.Request) {
		second = RequestIDFromContext(r.Context())
	})

	if first == second {
		t.Errorf("both requests got id %q", first)
	}
}

func TestRequestIDOutsideRequestIsEmpty(t *testing.T) {
	if got := RequestIDFromContext(t.Context()); got != "" {
		t.Errorf("id = %q, want empty", got)
	}
}
