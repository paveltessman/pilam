package account_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/http/account/views"
	"github.com/paveltessman/pilam/internal/http/paths"
	"github.com/paveltessman/pilam/internal/http/testkit"
	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

// changePassword submits the three fields of the change screen.
func changePassword(t *testing.T, deps testkit.Deps, session *http.Cookie, current, next, repeat string) *httptest.ResponseRecorder {
	t.Helper()

	form := url.Values{
		views.FieldCurrentPasswd: {current},
		views.FieldNewPasswd:     {next},
		views.FieldRepeatPasswd:  {repeat},
	}
	return testkit.PostAs(t, deps, paths.ChangePassword, form, session)
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
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.ExpiredEmail, testkit.ExpiredPasswd)

	rec := testkit.GetAs(t, d, paths.Models, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != paths.ChangePassword {
		t.Errorf("Location = %q, want %q", got, paths.ChangePassword)
	}
}

func TestExpiredPasswordReachesTheChangeScreen(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.ExpiredEmail, testkit.ExpiredPasswd)

	rec := testkit.GetAs(t, d, paths.ChangePassword, cookie)
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
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.ExpiredEmail, testkit.ExpiredPasswd)

	rec := testkit.PostAs(t, d, paths.Logout, nil, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != paths.Login {
		t.Errorf("Location = %q, want %q", got, paths.Login)
	}
	if cleared := testkit.SessionCookie(t, rec); cleared == nil || cleared.MaxAge >= 0 {
		t.Errorf("cookie = %+v, want it expired", cleared)
	}
}

func TestTheChangeScreenIsOpenToEveryUser(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	rec := testkit.GetAs(t, d, paths.ChangePassword, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if strings.Contains(rec.Body.String(), labels.PasswordExpired) {
		t.Error("the page holds a user whose password is current")
	}
}

func TestTheChangeScreenNeedsLogin(t *testing.T) {
	rec := testkit.Get(t, paths.ChangePassword)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != paths.Login {
		t.Errorf("Location = %q, want %q", got, paths.Login)
	}
}

func TestPasswordChangeOpensTheBoard(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.ExpiredEmail, testkit.ExpiredPasswd)

	const chosen = "the password they picked"
	rec := changePassword(t, d, cookie, testkit.ExpiredPasswd, chosen, chosen)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != paths.Models {
		t.Errorf("Location = %q, want %q", got, paths.Models)
	}

	// The device that made the change keeps working, under the cookie the
	// change handed it.
	reissued := testkit.SessionCookie(t, rec)
	if reissued == nil {
		t.Fatal("the change did not re-issue the session cookie")
	}
	if board := testkit.GetAs(t, d, paths.Models, reissued); board.Code != http.StatusOK {
		t.Errorf("board status = %d, want %d", board.Code, http.StatusOK)
	}

	// And the password it set is the one that logs in from now on.
	if again := testkit.Login(t, d, testkit.ExpiredEmail, chosen); again.Code != http.StatusSeeOther {
		t.Errorf("status = %d for the new password, want %d", again.Code, http.StatusSeeOther)
	}
}

func TestPasswordChangeEndsOtherDevices(t *testing.T) {
	d := testkit.NewDeps(t)
	phone := testkit.LoggedIn(t, d, testkit.ExpiredEmail, testkit.ExpiredPasswd)
	laptop := testkit.LoggedIn(t, d, testkit.ExpiredEmail, testkit.ExpiredPasswd)

	const chosen = "the password they picked"
	if rec := changePassword(t, d, laptop, testkit.ExpiredPasswd, chosen, chosen); rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	rec := testkit.GetAs(t, d, paths.Models, phone)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != paths.Login {
		t.Errorf("Location = %q, want %q", got, paths.Login)
	}
}

func TestPasswordChangeRefusesWrongCurrentPassword(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.ExpiredEmail, testkit.ExpiredPasswd)

	const chosen = "the password they picked"
	rec := changePassword(t, d, cookie, "not-the-password", chosen, chosen)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(rec.Body.String(), labels.Message(validate.FieldError{Code: validate.Incorrect})) {
		t.Error("the page does not say the current password was wrong")
	}

	// The old password still logs in, so nothing was written.
	if again := testkit.Login(t, d, testkit.ExpiredEmail, testkit.ExpiredPasswd); again.Code != http.StatusSeeOther {
		t.Errorf("status = %d for the old password, want %d", again.Code, http.StatusSeeOther)
	}
}

func TestPasswordChangeRefusesTwoDifferentNewPasswords(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.ExpiredEmail, testkit.ExpiredPasswd)

	rec := changePassword(t, d, cookie, testkit.ExpiredPasswd, "the password they picked", "the password they typed")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(rec.Body.String(), labels.Message(validate.FieldError{Code: validate.Mismatch})) {
		t.Error("the page does not say the two entries differ")
	}
}

func TestPasswordChangeRefusesShortPassword(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.ExpiredEmail, testkit.ExpiredPasswd)

	const short = "eleven char"
	rec := changePassword(t, d, cookie, testkit.ExpiredPasswd, short, short)
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
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.ExpiredEmail, testkit.ExpiredPasswd)

	rec := changePassword(t, d, cookie, "", "", "")
	required := labels.Message(validate.FieldError{Code: validate.Required})
	if got := strings.Count(rec.Body.String(), required); got != 3 {
		t.Errorf("%d fields carry %q, want 3", got, required)
	}
}

func TestChangeScreenCarriesNoPasswordBack(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.ExpiredEmail, testkit.ExpiredPasswd)

	rec := changePassword(t, d, cookie, testkit.ExpiredPasswd, "the password they picked", "typed something else")
	for _, secret := range []string{testkit.ExpiredPasswd, "the password they picked", "typed something else"} {
		if strings.Contains(rec.Body.String(), secret) {
			t.Errorf("the page carries %q back to the browser", secret)
		}
	}
}
