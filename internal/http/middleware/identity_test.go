package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/platform/logging"
)

const loginPath = "/login"

// authenticated returns a request whose session has already been resolved to
// subject, as the session middleware would have left it.
func authenticated(subject string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/board", nil)
	return r.WithContext(newSubjectContext(r.Context(), subject))
}

// capturesIdentity records what the identity middleware resolved, if anything.
func capturesIdentity(identity *auth.Identity, ok *bool) http.HandlerFunc {
	f := func(_ http.ResponseWriter, r *http.Request) {
		*identity, *ok = auth.FromContext(r.Context())
	}
	return f
}

func TestIdentityResolvesTheSessionSubject(t *testing.T) {
	var identity auth.Identity
	var ok bool
	serve(Identity(auth.Resolve), authenticated(auth.Subject), capturesIdentity(&identity, &ok))

	if !ok {
		t.Fatal("the request came out anonymous")
	}
	if identity.Subject != auth.Subject || identity.Role != auth.RoleManager {
		t.Errorf("identity = %+v, want the manager", identity)
	}
}

func TestIdentityLeavesAnonymousRequestAnonymous(t *testing.T) {
	var identity auth.Identity
	var ok bool
	serve(Identity(auth.Resolve), httptest.NewRequest(http.MethodGet, "/", nil),
		capturesIdentity(&identity, &ok))

	if ok {
		t.Errorf("identity = %+v, want none", identity)
	}
}

// A validly signed session naming a subject that no longer resolves: the key
// outlived the account.
func TestIdentityDiscardsSessionItCannotResolve(t *testing.T) {
	var identity auth.Identity
	var ok bool
	rec := serve(Identity(auth.Resolve), authenticated("someone-else"), capturesIdentity(&identity, &ok))

	if ok {
		t.Errorf("identity = %+v, want none", identity)
	}
	if c := sessionCookie(t, rec); c == nil || c.MaxAge >= 0 {
		t.Errorf("cookie = %+v, want it expired", c)
	}
}

func TestIdentityWidensTheRequestLogger(t *testing.T) {
	logger, buf := capture()

	rec := httptest.NewRecorder()
	Chain(Logger(logger), Identity(auth.Resolve))(
		http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			logging.FromContext(r.Context()).Info("from the handler")
		}),
	).ServeHTTP(rec, authenticated(auth.Subject))

	var handlerLine map[string]any
	for _, record := range records(t, buf) {
		if record["msg"] == "from the handler" {
			handlerLine = record
		}
	}
	if handlerLine == nil {
		t.Fatalf("the handler's line is missing: %s", buf)
	}
	if got := handlerLine["actor"]; got != auth.Subject {
		t.Errorf("actor = %v, want %q", got, auth.Subject)
	}
}

func TestIdentityPanicsWithoutResolver(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("a nil resolver was accepted")
		}
	}()
	Identity(nil)
}

func TestRequireIdentityAdmitsAuthenticatedRequest(t *testing.T) {
	reached := false
	rec := httptest.NewRecorder()
	Chain(Identity(auth.Resolve), RequireIdentity(loginPath))(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true }),
	).ServeHTTP(rec, authenticated(auth.Subject))

	if !reached {
		t.Fatalf("the handler did not run; status = %d", rec.Code)
	}
}

func TestRequireIdentitySendsAnonymousBrowserToTheLogin(t *testing.T) {
	reached := false
	rec := serve(RequireIdentity(loginPath), httptest.NewRequest(http.MethodGet, "/board", nil),
		func(http.ResponseWriter, *http.Request) { reached = true })

	if reached {
		t.Fatal("the handler ran for an anonymous request")
	}
	if rec.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != loginPath {
		t.Errorf("Location = %q, want %q", got, loginPath)
	}
}

func TestRequireIdentityRedirectsHTMXRequestThroughTheHeader(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/styles/1/milestones", nil)
	r.Header.Set(headerHXRequest, "true")

	rec := httptest.NewRecorder()
	Chain(HTMX(), RequireIdentity(loginPath))(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("the handler ran") }),
	).ServeHTTP(rec, r)

	if got := rec.Header().Get("HX-Redirect"); got != loginPath {
		t.Errorf("HX-Redirect = %q, want %q", got, loginPath)
	}
	if rec.Header().Get("Location") != "" {
		t.Error("it also sent a Location, which htmx would follow into the swap")
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", rec.Body)
	}
}

func TestIdentityReportsAResolverFailure(t *testing.T) {
	sentinel := errors.New("directory unreachable")
	resolve := func(context.Context, string) (auth.Identity, error) {
		return auth.Identity{}, sentinel
	}

	logger, buf := capture()
	rec := httptest.NewRecorder()
	Chain(Logger(logger), Identity(resolve))(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	).ServeHTTP(rec, authenticated(auth.Subject))

	var warned bool
	for _, record := range records(t, buf) {
		if record["level"] == "WARN" {
			warned = true
		}
	}
	if !warned {
		t.Errorf("a failing resolver was not reported: %s", buf)
	}
}
