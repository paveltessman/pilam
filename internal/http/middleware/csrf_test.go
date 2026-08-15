package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/labels"
)

// crossOrigin returns a POST that a browser reports as coming from elsewhere.
func crossOrigin(method string) *http.Request {
	r := httptest.NewRequest(method, "http://pilam.example/styles", nil)
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	r.Header.Set("Origin", "http://attacker.example")
	return r
}

func TestCSRFRejectsACrossOriginWrite(t *testing.T) {
	reached := false
	rec := serve(CSRF(), crossOrigin(http.MethodPost),
		func(http.ResponseWriter, *http.Request) { reached = true })

	if reached {
		t.Fatal("the handler ran")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if got := rec.Body.String(); !strings.Contains(got, labels.ErrorForbidden) {
		t.Errorf("body = %q, want it to contain %q", got, labels.ErrorForbidden)
	}
}

func TestCSRFAdmitsASameOriginWrite(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "http://pilam.example/styles", nil)
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	r.Header.Set("Origin", "http://pilam.example")

	reached := false
	serve(CSRF(), r, func(http.ResponseWriter, *http.Request) { reached = true })

	if !reached {
		t.Error("a same-origin write was rejected")
	}
}

func TestCSRFAdmitsSafeMethodsFromAnywhere(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		t.Run(method, func(t *testing.T) {
			reached := false
			serve(CSRF(), crossOrigin(method), func(http.ResponseWriter, *http.Request) { reached = true })

			if !reached {
				t.Errorf("%s was rejected", method)
			}
		})
	}
}

// No Sec-Fetch-Site and no Origin means it is not a browser form post — curl,
// or an old client. net/http lets those through, and the test states it so the
// assumption is visible rather than inherited.
func TestCSRFAdmitsARequestWithNoOriginSignals(t *testing.T) {
	reached := false
	serve(CSRF(), httptest.NewRequest(http.MethodPost, "http://pilam.example/styles", nil),
		func(http.ResponseWriter, *http.Request) { reached = true })

	if !reached {
		t.Error("a request carrying no origin signals was rejected")
	}
}

func TestCSRFTrustsAConfiguredOrigin(t *testing.T) {
	r := crossOrigin(http.MethodPost)
	r.Header.Set("Origin", "https://trusted.example")

	reached := false
	serve(CSRF("https://trusted.example"), r,
		func(http.ResponseWriter, *http.Request) { reached = true })

	if !reached {
		t.Error("the trusted origin was rejected")
	}
}

func TestCSRFPanicsOnAnUnparsableTrustedOrigin(t *testing.T) {
	defer func() {
		err := recover()
		if err == nil {
			t.Error("a malformed trusted origin was accepted")
		}
	}()
	CSRF("not an origin")
}

func TestCSRFRejectionQuotesTheRequestID(t *testing.T) {
	rec := httptest.NewRecorder()
	Chain(RequestID(ids.NewDeterministic(1)), CSRF())(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	).ServeHTTP(rec, crossOrigin(http.MethodPost))

	if got := rec.Body.String(); !strings.Contains(got, rec.Header().Get(RequestIDHeader)) {
		t.Errorf("body = %q, want it to quote the request id", got)
	}
}

func TestCSRFLogsTheRejection(t *testing.T) {
	logger, buf := capture()

	rec := httptest.NewRecorder()
	Chain(Logger(logger), CSRF())(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	).ServeHTTP(rec, crossOrigin(http.MethodPost))

	var found map[string]any
	for _, record := range records(t, buf) {
		if record["msg"] == "cross-origin request rejected" {
			found = record
		}
	}
	if found == nil {
		t.Fatalf("the rejection was not logged: %s", buf)
	}
	if found["origin"] != "http://attacker.example" {
		t.Errorf("origin = %v, want the attacker's", found["origin"])
	}
}
