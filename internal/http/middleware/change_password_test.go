package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/paveltessman/pilam/internal/auth"
)

const changePath = "/password"

// expired returns a request from a user whose password must change before they
// reach anything else, as the identity middleware would have left it.
func expired(path string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	identity := testIdentity
	identity.PasswdExpired = true
	return r.WithContext(auth.NewContext(r.Context(), identity))
}

func TestRequirePasswordChangeSendsUserToChangeScreen(t *testing.T) {
	reached := false
	rec := serve(RequirePasswordChange(changePath), expired("/board"),
		func(http.ResponseWriter, *http.Request) { reached = true })

	if reached {
		t.Fatal("the handler ran for a user whose password expired")
	}
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != changePath {
		t.Errorf("Location = %q, want %q", got, changePath)
	}
}

// The screen the guard sends to has to stay reachable, or the redirect loops.
func TestRequirePasswordChangeAdmitsTheChangeScreen(t *testing.T) {
	reached := false
	rec := serve(RequirePasswordChange(changePath), expired(changePath),
		func(http.ResponseWriter, *http.Request) { reached = true })

	if !reached {
		t.Fatalf("the change screen did not run; status = %d", rec.Code)
	}
}

func TestRequirePasswordChangeAdmitsCurrentPassword(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/board", nil)
	r = r.WithContext(auth.NewContext(r.Context(), testIdentity))

	reached := false
	rec := serve(RequirePasswordChange(changePath), r,
		func(http.ResponseWriter, *http.Request) { reached = true })

	if !reached {
		t.Fatalf("the handler did not run; status = %d", rec.Code)
	}
}

func TestRequirePasswordChangeIgnoresAnonymousRequest(t *testing.T) {
	reached := false
	serve(RequirePasswordChange(changePath), httptest.NewRequest(http.MethodGet, "/board", nil),
		func(http.ResponseWriter, *http.Request) { reached = true })

	if !reached {
		t.Error("the handler did not run")
	}
}

func TestRequirePasswordChangeRedirectsHTMXRequestThroughTheHeader(t *testing.T) {
	r := expired("/styles/1/milestones")
	r.Header.Set(headerHXRequest, "true")

	rec := httptest.NewRecorder()
	Chain(HTMX(), RequirePasswordChange(changePath))(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("the handler ran") }),
	).ServeHTTP(rec, r)

	if got := rec.Header().Get("HX-Redirect"); got != changePath {
		t.Errorf("HX-Redirect = %q, want %q", got, changePath)
	}
	if rec.Header().Get("Location") != "" {
		t.Error("it also sent a Location, which htmx would follow into the swap")
	}
}
