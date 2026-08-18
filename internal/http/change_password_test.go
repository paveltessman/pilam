package http

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/http/views"
	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

// postAs posts a form with the session a login handed out.
func postAs(t *testing.T, deps Deps, path string, form url.Values, session *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if session != nil {
		r.AddCookie(session)
	}

	rec := httptest.NewRecorder()
	NewRouter(deps).ServeHTTP(rec, r)
	return rec
}

// changePassword submits the three fields of the change screen.
func changePassword(t *testing.T, deps Deps, session *http.Cookie, current, next, repeat string) *httptest.ResponseRecorder {
	t.Helper()

	form := url.Values{
		views.FieldCurrentPasswd: {current},
		views.FieldNewPasswd:     {next},
		views.FieldRepeatPasswd:  {repeat},
	}
	return postAs(t, deps, changePasswordPath, form, session)
}

// loggedIn logs the user in and returns the session cookie.
func loggedIn(t *testing.T, deps Deps, email, password string) *http.Cookie {
	t.Helper()

	cookie := sessionCookie(t, login(t, deps, email, password))
	if cookie == nil {
		t.Fatal("no session cookie was set")
	}
	return cookie
}

func TestScreenNamesFieldsThatServiceRejects(t *testing.T) {
	if views.FieldCurrentPasswd != auth.FieldCurrentPass {
		t.Errorf("the view posts %q, the service rejects %q",
			views.FieldCurrentPasswd, auth.FieldCurrentPass)
	}
	if views.FieldNewPasswd != auth.FieldNewPass {
		t.Errorf("the view posts %q, the service rejects %q",
			views.FieldNewPasswd, auth.FieldNewPass)
	}
}

func TestExpiredPasswordCannotReachTheBoard(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, expiredEmail, expiredPasswd)

	rec := getAs(t, d, successPath, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != changePasswordPath {
		t.Errorf("Location = %q, want %q", got, changePasswordPath)
	}
}

func TestExpiredPasswordReachesTheChangeScreen(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, expiredEmail, expiredPasswd)

	rec := getAs(t, d, changePasswordPath, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	for _, want := range []string{
		`name="` + views.FieldCurrentPasswd + `"`,
		`name="` + views.FieldNewPasswd + `"`,
		`name="` + views.FieldRepeatPasswd + `"`,
		labels.PasswordExpired,
		labels.PasswordHint,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the page does not contain %q", want)
		}
	}
}

func TestExpiredPasswordStillReachesLogout(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, expiredEmail, expiredPasswd)

	rec := postAs(t, d, logoutPath, nil, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != loginPath {
		t.Errorf("Location = %q, want %q", got, loginPath)
	}
	if cleared := sessionCookie(t, rec); cleared == nil || cleared.MaxAge >= 0 {
		t.Errorf("cookie = %+v, want it expired", cleared)
	}
}

func TestTheChangeScreenIsOpenToEveryUser(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, testEmail, testPasswd)

	rec := getAs(t, d, changePasswordPath, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if strings.Contains(rec.Body.String(), labels.PasswordExpired) {
		t.Error("the page holds a user whose password is current")
	}
}

func TestTheChangeScreenNeedsLogin(t *testing.T) {
	rec := get(t, changePasswordPath)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != loginPath {
		t.Errorf("Location = %q, want %q", got, loginPath)
	}
}

func TestPasswordChangeOpensTheBoard(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, expiredEmail, expiredPasswd)

	const chosen = "the password they picked"
	rec := changePassword(t, d, cookie, expiredPasswd, chosen, chosen)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != successPath {
		t.Errorf("Location = %q, want %q", got, successPath)
	}

	// The device that made the change keeps working, under the cookie the
	// change handed it.
	reissued := sessionCookie(t, rec)
	if reissued == nil {
		t.Fatal("the change did not re-issue the session cookie")
	}
	if board := getAs(t, d, successPath, reissued); board.Code != http.StatusOK {
		t.Errorf("board status = %d, want %d", board.Code, http.StatusOK)
	}

	// And the password it set is the one that logs in from now on.
	if again := login(t, d, expiredEmail, chosen); again.Code != http.StatusSeeOther {
		t.Errorf("status = %d for the new password, want %d", again.Code, http.StatusSeeOther)
	}
}

func TestPasswordChangeEndsOtherDevices(t *testing.T) {
	d := deps(t)
	phone := loggedIn(t, d, expiredEmail, expiredPasswd)
	laptop := loggedIn(t, d, expiredEmail, expiredPasswd)

	const chosen = "the password they picked"
	if rec := changePassword(t, d, laptop, expiredPasswd, chosen, chosen); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	rec := getAs(t, d, successPath, phone)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != loginPath {
		t.Errorf("Location = %q, want %q", got, loginPath)
	}
}

func TestPasswordChangeRefusesWrongCurrentPassword(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, expiredEmail, expiredPasswd)

	const chosen = "the password they picked"
	rec := changePassword(t, d, cookie, "not-the-password", chosen, chosen)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(rec.Body.String(), labels.Message(validate.FieldError{Code: validate.Incorrect})) {
		t.Error("the page does not say the current password was wrong")
	}

	// The old password still logs in, so nothing was written.
	if again := login(t, d, expiredEmail, expiredPasswd); again.Code != http.StatusSeeOther {
		t.Errorf("status = %d for the old password, want %d", again.Code, http.StatusSeeOther)
	}
}

func TestPasswordChangeRefusesTwoDifferentNewPasswords(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, expiredEmail, expiredPasswd)

	rec := changePassword(t, d, cookie, expiredPasswd, "the password they picked", "the password they typed")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(rec.Body.String(), labels.Message(validate.FieldError{Code: validate.Mismatch})) {
		t.Error("the page does not say the two entries differ")
	}
}

func TestPasswordChangeRefusesShortPassword(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, expiredEmail, expiredPasswd)

	const short = "eleven char"
	rec := changePassword(t, d, cookie, expiredPasswd, short, short)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}

	want := labels.Message(validate.FieldError{Code: validate.TooShort, Arg: "12"})
	if !strings.Contains(rec.Body.String(), want) {
		t.Errorf("the page does not carry %q", want)
	}

	// The rejection states the rule the hint states, so it takes the hint's
	// place rather than sitting under it.
	if strings.Contains(rec.Body.String(), labels.PasswordHint) {
		t.Errorf("the page carries both %q and %q", labels.PasswordHint, want)
	}
}

func TestPasswordChangeReportsEmptyFieldsPerField(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, expiredEmail, expiredPasswd)

	rec := changePassword(t, d, cookie, "", "", "")
	required := labels.Message(validate.FieldError{Code: validate.Required})
	if got := strings.Count(rec.Body.String(), required); got != 3 {
		t.Errorf("%d fields carry %q, want 3", got, required)
	}
}

func TestChangeScreenCarriesNoPasswordBack(t *testing.T) {
	d := deps(t)
	cookie := loggedIn(t, d, expiredEmail, expiredPasswd)

	rec := changePassword(t, d, cookie, expiredPasswd, "the password they picked", "typed something else")
	for _, secret := range []string{expiredPasswd, "the password they picked", "typed something else"} {
		if strings.Contains(rec.Body.String(), secret) {
			t.Errorf("the page carries %q back to the browser", secret)
		}
	}
}
