package http

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/http/middleware"
	"github.com/paveltessman/pilam/internal/http/views"
	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

// login posts a credential and returns what the router answered.
func login(t *testing.T, deps Deps, email, password string) *httptest.ResponseRecorder {
	t.Helper()

	form := url.Values{
		views.FieldUsername: {email},
		views.FieldPassword: {password},
	}
	r := httptest.NewRequest(http.MethodPost, loginPath, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	NewRouter(deps).ServeHTTP(rec, r)
	return rec
}

// sessionCookie returns the session cookie a response set, or nil.
func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == middleware.SessionCookie {
			return c
		}
	}
	return nil
}

// getAs repeats a GET with the session a successful login handed out.
func getAs(t *testing.T, deps Deps, path string, session *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.AddCookie(session)

	rec := httptest.NewRecorder()
	NewRouter(deps).ServeHTTP(rec, r)
	return rec
}

func TestLoginPageRendersTheForm(t *testing.T) {
	rec := get(t, loginPath)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	for _, want := range []string{
		`name="username"`,
		`type="password"`,
		`method="post"`,
		labels.LoginTitle,
		labels.ActionLogIn,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the page does not contain %q", want)
		}
	}
}

func TestLoginStartsSessionAndLandsOnBoard(t *testing.T) {
	d := deps(t)
	rec := login(t, d, testEmail, testPasswd)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != successPath {
		t.Errorf("Location = %q, want %q", got, successPath)
	}

	cookie := sessionCookie(t, rec)
	if cookie == nil {
		t.Fatal("no session cookie was set")
	}
	if !cookie.HttpOnly {
		t.Error("the session cookie is readable from JavaScript")
	}

	board := getAs(t, d, successPath, cookie)
	if board.Code != http.StatusOK {
		t.Fatalf("board status = %d, want %d", board.Code, http.StatusOK)
	}
}

func TestSessionSurvivesRefresh(t *testing.T) {
	d := deps(t)
	cookie := sessionCookie(t, login(t, d, testEmail, testPasswd))
	if cookie == nil {
		t.Fatal("no session cookie was set")
	}

	for range 2 {
		if rec := getAs(t, d, successPath, cookie); rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	}
}

func TestWrongPasswordRendersFormAgain(t *testing.T) {
	rec := login(t, deps(t), testEmail, "not-the-password")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if c := sessionCookie(t, rec); c != nil {
		t.Errorf("a rejected login set a session: %+v", c)
	}

	body := rec.Body.String()
	if !strings.Contains(body, labels.LoginFailed) {
		t.Errorf("the page does not say the credential was wrong: %q", body)
	}
	if strings.Contains(body, "not-the-password") {
		t.Error("the page carries the submitted password back to the browser")
	}
}

// An unknown email must read exactly like a wrong password.
func TestWrongEmailAnswersTheSameWay(t *testing.T) {
	d := deps(t)
	wrongName := login(t, d, "someone-else@example.com", testPasswd)
	wrongPassword := login(t, d, testEmail, "not-the-password")

	if wrongName.Code != wrongPassword.Code {
		t.Errorf("status = %d for a wrong username, %d for a wrong password",
			wrongName.Code, wrongPassword.Code)
	}
	if !strings.Contains(wrongName.Body.String(), labels.LoginFailed) {
		t.Error("a wrong email does not give the shared message")
	}
}

func TestEmptyFieldsAreReportedPerField(t *testing.T) {
	rec := login(t, deps(t), "", "")

	body := rec.Body.String()
	if strings.Contains(body, labels.LoginFailed) {
		t.Error("an empty form was answered as a wrong credential")
	}

	required := labels.Message(validate.FieldError{Code: validate.Required})
	if got := strings.Count(body, required); got != 2 {
		t.Errorf("%d fields carry %q, want 2", got, required)
	}
}

func TestAnonymousRequestIsSentToTheLogin(t *testing.T) {
	rec := get(t, successPath)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != loginPath {
		t.Errorf("Location = %q, want %q", got, loginPath)
	}
}

func TestLoginPageSendsALoggedInUserToTheBoard(t *testing.T) {
	d := deps(t)
	cookie := sessionCookie(t, login(t, d, testEmail, testPasswd))

	rec := getAs(t, d, loginPath, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != successPath {
		t.Errorf("Location = %q, want %q", got, successPath)
	}
}

func TestLogoutClearsTheSession(t *testing.T) {
	d := deps(t)
	cookie := sessionCookie(t, login(t, d, testEmail, testPasswd))

	r := httptest.NewRequest(http.MethodPost, logoutPath, nil)
	r.AddCookie(cookie)
	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, r)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != loginPath {
		t.Errorf("Location = %q, want %q", got, loginPath)
	}

	cleared := sessionCookie(t, rec)
	if cleared == nil || cleared.MaxAge >= 0 {
		t.Errorf("cookie = %+v, want it expired", cleared)
	}
}

func TestBoardOffersTheWayOut(t *testing.T) {
	d := deps(t)
	cookie := sessionCookie(t, login(t, d, testEmail, testPasswd))

	rec := getAs(t, d, successPath, cookie)
	for _, want := range []string{labels.NavLogOut, `action="/logout"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("the board does not contain %q", want)
		}
	}
}
