package account_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pilamhttp "github.com/paveltessman/pilam/internal/http"
	"github.com/paveltessman/pilam/internal/http/paths"
	"github.com/paveltessman/pilam/internal/http/testkit"
	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

func TestLoginPageRendersTheForm(t *testing.T) {
	rec := testkit.Get(t, paths.Login)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	for _, want := range []string{
		`name="email"`,
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
	d := testkit.NewDeps(t)
	rec := testkit.Login(t, d, testkit.TestEmail, testkit.TestPasswd)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != paths.Models {
		t.Errorf("Location = %q, want %q", got, paths.Models)
	}

	cookie := testkit.SessionCookie(t, rec)
	if cookie == nil {
		t.Fatal("no session cookie was set")
	}
	if !cookie.HttpOnly {
		t.Error("the session cookie is readable from JavaScript")
	}

	board := testkit.GetAs(t, d, paths.Models, cookie)
	if board.Code != http.StatusOK {
		t.Fatalf("board status = %d, want %d", board.Code, http.StatusOK)
	}
}

func TestSessionSurvivesRefresh(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.SessionCookie(t, testkit.Login(t, d, testkit.TestEmail, testkit.TestPasswd))
	if cookie == nil {
		t.Fatal("no session cookie was set")
	}

	for range 2 {
		if rec := testkit.GetAs(t, d, paths.Models, cookie); rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	}
}

func TestWrongPasswordRendersFormAgain(t *testing.T) {
	rec := testkit.Login(t, testkit.NewDeps(t), testkit.TestEmail, "not-the-password")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if c := testkit.SessionCookie(t, rec); c != nil {
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
	d := testkit.NewDeps(t)
	wrongName := testkit.Login(t, d, "someone-else@example.com", testkit.TestPasswd)
	wrongPassword := testkit.Login(t, d, testkit.TestEmail, "not-the-password")

	if wrongName.Code != wrongPassword.Code {
		t.Errorf("status = %d for a wrong email, %d for a wrong password",
			wrongName.Code, wrongPassword.Code)
	}
	if !strings.Contains(wrongName.Body.String(), labels.LoginFailed) {
		t.Error("a wrong email does not give the shared message")
	}
}

func TestEmptyFieldsAreReportedPerField(t *testing.T) {
	rec := testkit.Login(t, testkit.NewDeps(t), "", "")

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
	rec := testkit.Get(t, paths.Models)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != paths.Login {
		t.Errorf("Location = %q, want %q", got, paths.Login)
	}
}

func TestLoginPageSendsALoggedInUserToTheBoard(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.SessionCookie(t, testkit.Login(t, d, testkit.TestEmail, testkit.TestPasswd))

	rec := testkit.GetAs(t, d, paths.Login, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != paths.Models {
		t.Errorf("Location = %q, want %q", got, paths.Models)
	}
}

func TestLogoutClearsTheSession(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.SessionCookie(t, testkit.Login(t, d, testkit.TestEmail, testkit.TestPasswd))

	r := httptest.NewRequest(http.MethodPost, paths.Logout, nil)
	r.AddCookie(cookie)
	rec := httptest.NewRecorder()
	pilamhttp.NewRouter(d).ServeHTTP(rec, r)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != paths.Login {
		t.Errorf("Location = %q, want %q", got, paths.Login)
	}

	cleared := testkit.SessionCookie(t, rec)
	if cleared == nil || cleared.MaxAge >= 0 {
		t.Errorf("cookie = %+v, want it expired", cleared)
	}
}

func TestDeactivatedUserLosesTheSession(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.SessionCookie(t, testkit.Login(t, d, testkit.TestEmail, testkit.TestPasswd))
	if cookie == nil {
		t.Fatal("no session cookie was set")
	}

	if err := testkit.Deactivate(t, d, testkit.TestUserID); err != nil {
		t.Fatalf("deactivating the user: %v", err)
	}

	rec := testkit.GetAs(t, d, paths.Models, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != paths.Login {
		t.Errorf("Location = %q, want %q", got, paths.Login)
	}

	cleared := testkit.SessionCookie(t, rec)
	if cleared == nil || cleared.MaxAge >= 0 {
		t.Errorf("cookie = %+v, want it expired", cleared)
	}
}

func TestDeactivatedUserCannotLogInAgain(t *testing.T) {
	d := testkit.NewDeps(t)
	if err := testkit.Deactivate(t, d, testkit.TestUserID); err != nil {
		t.Fatalf("deactivating the user: %v", err)
	}

	rec := testkit.Login(t, d, testkit.TestEmail, testkit.TestPasswd)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if c := testkit.SessionCookie(t, rec); c != nil {
		t.Errorf("a deactivated user got a session: %+v", c)
	}
	if !strings.Contains(rec.Body.String(), labels.LoginFailed) {
		t.Error("a deactivated user does not give the shared message")
	}
}

func TestPasswordChangeEndsTheOtherSessions(t *testing.T) {
	d := testkit.NewDeps(t)
	phone := testkit.SessionCookie(t, testkit.Login(t, d, testkit.TestEmail, testkit.TestPasswd))
	laptop := testkit.SessionCookie(t, testkit.Login(t, d, testkit.TestEmail, testkit.TestPasswd))
	if phone == nil || laptop == nil {
		t.Fatal("no session cookie was set")
	}

	const newPasswd = "another long enough password"
	if _, err := d.AuthSvc.ChangePassword(t.Context(), testkit.TestUserID, testkit.TestPasswd, newPasswd); err != nil {
		t.Fatalf("changing the password: %v", err)
	}

	for name, cookie := range map[string]*http.Cookie{"phone": phone, "laptop": laptop} {
		rec := testkit.GetAs(t, d, paths.Models, cookie)
		if rec.Code != http.StatusSeeOther {
			t.Errorf("%s: status = %d, want %d", name, rec.Code, http.StatusSeeOther)
		}
	}

	fresh := testkit.Login(t, d, testkit.TestEmail, newPasswd)
	if fresh.Code != http.StatusSeeOther {
		t.Fatalf("status = %d for the new password, want %d", fresh.Code, http.StatusSeeOther)
	}
	if cookie := testkit.SessionCookie(t, fresh); cookie == nil {
		t.Fatal("the new password did not start a session")
	} else if rec := testkit.GetAs(t, d, paths.Models, cookie); rec.Code != http.StatusOK {
		t.Errorf("board status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestBoardOffersTheWayOut(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.SessionCookie(t, testkit.Login(t, d, testkit.TestEmail, testkit.TestPasswd))

	rec := testkit.GetAs(t, d, paths.Models, cookie)
	for _, want := range []string{labels.NavLogOut, `action="/logout"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("the board does not contain %q", want)
		}
	}
}
