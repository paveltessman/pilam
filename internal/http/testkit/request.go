package testkit

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	pilamhttp "github.com/paveltessman/pilam/internal/http"
	"github.com/paveltessman/pilam/internal/http/account/views"
	"github.com/paveltessman/pilam/internal/http/middleware"
	"github.com/paveltessman/pilam/internal/http/paths"
)

// Route is one route of a section: the method it answers, and the path it is
// mounted on.
type Route struct {
	Method string
	Path   string
}

// Call sends one request of a section, with the session it is made under.
func Call(t *testing.T, deps Deps, r Route, form url.Values, session *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	if r.Method == http.MethodPost {
		return PostAs(t, deps, r.Path, form, session)
	}
	if session == nil {
		return GetWith(t, deps, r.Path)
	}
	return GetAs(t, deps, r.Path, session)
}

// Get serves a GET on a router the call builds itself.
func Get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	return GetWith(t, NewDeps(t), path)
}

// GetWith serves a GET with no session on it.
func GetWith(t *testing.T, deps Deps, path string) *httptest.ResponseRecorder {
	t.Helper()
	return serve(deps, httptest.NewRequest(http.MethodGet, path, nil))
}

// GetAs repeats a GET with the session a successful login handed out.
func GetAs(t *testing.T, deps Deps, path string, session *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.AddCookie(session)
	return serve(deps, r)
}

// GetAsHTMX asks as the search box does: the same URL, with the header htmx
// puts on a request it makes itself.
func GetAsHTMX(t *testing.T, deps Deps, path string, session *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.Header.Set("HX-Request", "true")
	r.AddCookie(session)
	return serve(deps, r)
}

// PostAs posts a form with the session a login handed out.
func PostAs(t *testing.T, deps Deps, path string, form url.Values, session *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if session != nil {
		r.AddCookie(session)
	}
	return serve(deps, r)
}

// Serve sends a request a test built itself, such as a file upload.
func Serve(deps Deps, r *http.Request) *httptest.ResponseRecorder { return serve(deps, r) }

func serve(deps Deps, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	pilamhttp.NewRouter(deps).ServeHTTP(rec, r)
	return rec
}

// Login posts a credential and returns what the router answered.
func Login(t *testing.T, deps Deps, email, password string) *httptest.ResponseRecorder {
	t.Helper()

	form := url.Values{
		views.FieldEmail:    {email},
		views.FieldPassword: {password},
	}
	r := httptest.NewRequest(http.MethodPost, paths.Login, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return serve(deps, r)
}

// LoggedIn logs the user in and returns the session cookie.
func LoggedIn(t *testing.T, deps Deps, email, password string) *http.Cookie {
	t.Helper()

	cookie := SessionCookie(t, Login(t, deps, email, password))
	if cookie == nil {
		t.Fatal("no session cookie was set")
	}
	return cookie
}

// SessionCookie returns the session cookie a response set, or nil.
func SessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == middleware.SessionCookie {
			return c
		}
	}
	return nil
}
