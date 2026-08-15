package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/platform/clock"
	"github.com/paveltessman/pilam/internal/platform/session"
)

const testTTL = 24 * time.Hour

func manager(t *testing.T) (*session.Manager, *clock.FixedClock) {
	t.Helper()
	clk := clock.Fixed(time.Date(2026, time.August, 15, 12, 0, 0, 0, time.UTC), time.UTC)
	mgr := session.New([]byte("test signing key"), testTTL, clk)
	return mgr, clk
}

// withSession returns a request carrying value in the session cookie.
func withSession(value string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookie, Value: value})
	return r
}

// sessionCookie finds the session cookie in a response, if it set one.
func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()

	for _, c := range rec.Result().Cookies() {
		if c.Name == SessionCookie {
			return c
		}
	}
	return nil
}

// capturesSubject records what the session middleware resolved, if anything.
func capturesSubject(subject *string, ok *bool) http.HandlerFunc {
	return func(_ http.ResponseWriter, r *http.Request) {
		*subject, *ok = SubjectFromContext(r.Context())
	}
}

func TestSessionResolvesValidCookie(t *testing.T) {
	sessions, _ := manager(t)

	var subject string
	var ok bool
	serve(Session(sessions), withSession(sessions.Issue("manager")), capturesSubject(&subject, &ok))

	if !ok || subject != "manager" {
		t.Errorf("subject = %q, %v; want %q, true", subject, ok, "manager")
	}
}

func TestSessionLetsAnAnonymousRequestThrough(t *testing.T) {
	mgr, _ := manager(t)

	var subject string
	var ok bool
	reached := false
	serve(Session(mgr), httptest.NewRequest(http.MethodGet, "/", nil),
		func(w http.ResponseWriter, r *http.Request) {
			reached = true
			capturesSubject(&subject, &ok)(w, r)
		})

	if !reached {
		t.Fatal("the handler did not run: Session is guarding, and it should not")
	}
	if ok {
		t.Errorf("subject = %q, want none", subject)
	}
}

func TestSessionLetsRejectedCookieThrough(t *testing.T) {
	mgr, _ := manager(t)

	testData := map[string]string{
		"tampered":      mgr.Issue("manager") + "x",
		"not a session": "garbage",
		"empty":         "",
	}

	for name, value := range testData {
		t.Run(name, func(t *testing.T) {
			var subject string
			var ok bool
			reached := false
			rec := serve(Session(mgr), withSession(value), func(w http.ResponseWriter, r *http.Request) {
				reached = true
				capturesSubject(&subject, &ok)(w, r)
			})

			if !reached {
				t.Fatal("the handler did not run")
			}
			if ok {
				t.Errorf("subject = %q, want none", subject)
			}
			if c := sessionCookie(t, rec); c == nil || c.MaxAge >= 0 {
				t.Errorf("cookie = %+v, want it expired", c)
			}
		})
	}
}

func TestSessionRejectsExpiredCookie(t *testing.T) {
	mgr, clk := manager(t)
	value := mgr.Issue("manager")
	clk.Advance(testTTL + time.Second)

	var subject string
	var ok bool
	rec := serve(Session(mgr), withSession(value), capturesSubject(&subject, &ok))

	if ok {
		t.Errorf("subject = %q, want none", subject)
	}
	if c := sessionCookie(t, rec); c == nil || c.MaxAge >= 0 {
		t.Errorf("cookie = %+v, want it expired", c)
	}
}

func TestSetSessionIssuesVerifiableCookie(t *testing.T) {
	mgr, _ := manager(t)

	rec := httptest.NewRecorder()
	SetSession(rec, httptest.NewRequest(http.MethodGet, "/", nil), mgr, "manager")

	cookie := sessionCookie(t, rec)
	if cookie == nil {
		t.Fatal("no session cookie was set")
	}
	subject, err := mgr.Verify(cookie.Value)
	if err != nil {
		t.Fatalf("the cookie it set does not verify: %v", err)
	}
	if subject != "manager" {
		t.Errorf("subject = %q, want %q", subject, "manager")
	}
	if want := int(testTTL.Seconds()); cookie.MaxAge != want {
		t.Errorf("MaxAge = %d, want %d", cookie.MaxAge, want)
	}
}

func TestSessionCookieIsNotReadableFromJavaScript(t *testing.T) {
	mgr, _ := manager(t)

	rec := httptest.NewRecorder()
	SetSession(rec, httptest.NewRequest(http.MethodGet, "/", nil), mgr, "manager")

	cookie := sessionCookie(t, rec)
	if !cookie.HttpOnly {
		t.Error("HttpOnly is not set")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", cookie.SameSite)
	}
	if cookie.Path != "/" {
		t.Errorf("Path = %q, want /", cookie.Path)
	}
}

func TestSessionCookieIsSecureWhenTheRequestWas(t *testing.T) {
	mgr, _ := manager(t)

	overTLS := httptest.NewRequest(http.MethodGet, "https://pilam.example/", nil)
	proxied := httptest.NewRequest(http.MethodGet, "/", nil)
	proxied.Header.Set("X-Forwarded-Proto", "https")

	testData := map[string]struct {
		request *http.Request
		want    bool
	}{
		"plain http": {httptest.NewRequest(http.MethodGet, "/", nil), false},
		"tls":        {overTLS, true},
		"proxied":    {proxied, true},
	}

	for name, tc := range testData {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			SetSession(rec, tc.request, mgr, "manager")

			if got := sessionCookie(t, rec).Secure; got != tc.want {
				t.Errorf("Secure = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestClearSessionMatchesTheCookieItClears(t *testing.T) {
	mgr, _ := manager(t)
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	set := httptest.NewRecorder()
	SetSession(set, r, mgr, "manager")
	was := sessionCookie(t, set)

	cleared := httptest.NewRecorder()
	ClearSession(cleared, r)
	now := sessionCookie(t, cleared)

	if now.Value != "" {
		t.Errorf("value = %q, want empty", now.Value)
	}
	if now.MaxAge >= 0 {
		t.Errorf("MaxAge = %d, want negative", now.MaxAge)
	}
	if now.Path != was.Path || now.SameSite != was.SameSite || now.Secure != was.Secure || now.HttpOnly != was.HttpOnly {
		t.Errorf("attributes = %+v, want them to match the set cookie %+v", now, was)
	}
}

func TestSubjectOutsideARequestIsAbsent(t *testing.T) {
	if subject, ok := SubjectFromContext(t.Context()); ok {
		t.Errorf("subject = %q, want none", subject)
	}
}
