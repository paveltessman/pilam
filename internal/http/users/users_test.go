package users_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/auth"
	pilamhttp "github.com/paveltessman/pilam/internal/http"
	"github.com/paveltessman/pilam/internal/http/paths"
	"github.com/paveltessman/pilam/internal/http/testkit"
	"github.com/paveltessman/pilam/internal/http/users/views"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/validate"
)

var sectionRoutes = []testkit.Route{
	{Method: http.MethodGet, Path: paths.Users},
	{Method: http.MethodGet, Path: paths.Users + "/new"},
	{Method: http.MethodPost, Path: paths.Users},
	{Method: http.MethodGet, Path: paths.Users + "/" + testkit.TestUserID.String()},
	{Method: http.MethodPost, Path: paths.Users + "/" + testkit.TestUserID.String()},
	{Method: http.MethodPost, Path: paths.Users + "/" + testkit.TestUserID.String() + "/password"},
}

// userForm is what the create and the edit screen post.
func userForm(email, first, last string, role auth.Role, active bool) url.Values {
	form := url.Values{
		views.FieldUserEmail:     {email},
		views.FieldUserFirstName: {first},
		views.FieldUserLastName:  {last},
		views.FieldUserRole:      {string(role)},
	}
	if active {
		form.Set(views.FieldUserActive, "true")
	}
	return form
}

// passwordOf reads the password back off the screen.
func passwordOf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()

	body := rec.Body.String()
	const open, closed = "select-all\">", "</code>"
	_, rest, found := strings.Cut(body, open)
	if !found {
		t.Fatalf("the page shows no password: %s", body)
	}
	passwd, _, found := strings.Cut(rest, closed)
	if !found {
		t.Fatal("the password block does not close")
	}
	return strings.TrimSpace(passwd)
}

func TestUsersSectionRefusesMember(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	for _, r := range sectionRoutes {
		rec := testkit.Call(t, d, r, userForm("new@example.com", "Ada", "Lovelace", auth.MemberRole, true), cookie)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s status = %d, want %d", r.Method, r.Path, rec.Code, http.StatusForbidden)
		}
	}
}

func TestUsersSectionNeedsLogin(t *testing.T) {
	d := testkit.NewDeps(t)

	for _, r := range sectionRoutes {
		rec := testkit.Call(t, d, r, nil, nil)
		if rec.Code != http.StatusSeeOther {
			t.Errorf("%s %s status = %d, want %d", r.Method, r.Path, rec.Code, http.StatusSeeOther)
			continue
		}
		if got := rec.Header().Get("Location"); got != paths.Login {
			t.Errorf("%s %s Location = %q, want %q", r.Method, r.Path, got, paths.Login)
		}
	}
}

func TestUsersSectionAdmitsRoot(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	rec := testkit.GetAs(t, d, paths.Users, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestUsersListShowsEveryUser(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	body := testkit.GetAs(t, d, paths.Users, cookie).Body.String()
	for _, want := range []string{
		testkit.TestEmail,
		testkit.ExpiredEmail,
		testkit.RootEmail,
		"Ada Lovelace",
		labels.UsersRoleRoot,
		labels.UsersRoleMember,
		labels.UsersActive,
		labels.Date(testkit.SeedChange),
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the list does not contain %q", want)
		}
	}
}

func TestUsersSearchKeepsRowsThatMatch(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	cases := []struct {
		query string
		want  []string
		gone  []string
	}{
		{"lovelace", []string{testkit.TestEmail}, []string{testkit.ExpiredEmail, testkit.RootEmail}},
		{"GRACE", []string{testkit.ExpiredEmail}, []string{testkit.TestEmail, testkit.RootEmail}},
		{"root@", []string{testkit.RootEmail}, []string{testkit.TestEmail, testkit.ExpiredEmail}},
		{"example.com", []string{testkit.TestEmail, testkit.ExpiredEmail, testkit.RootEmail}, nil},
		{"", []string{testkit.TestEmail, testkit.ExpiredEmail, testkit.RootEmail}, nil},
	}

	for _, c := range cases {
		body := testkit.GetAs(t, d, paths.Users+"?q="+url.QueryEscape(c.query), cookie).Body.String()
		for _, want := range c.want {
			if !strings.Contains(body, want) {
				t.Errorf("the search for %q drops %q", c.query, want)
			}
		}
		for _, gone := range c.gone {
			if strings.Contains(body, gone) {
				t.Errorf("the search for %q keeps %q", c.query, gone)
			}
		}
	}
}

func TestUsersSearchCarriesQueryBackToTheBox(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	body := testkit.GetAs(t, d, paths.Users+"?q=lovelace", cookie).Body.String()
	if !strings.Contains(body, `value="lovelace"`) {
		t.Error("the search box comes back empty after a search")
	}
}

func TestUsersSearchReportsThatNobodyMatches(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	body := testkit.GetAs(t, d, paths.Users+"?q=nobody", cookie).Body.String()
	if !strings.Contains(body, labels.UsersNoMatch) {
		t.Error("a search that matches nobody does not say so")
	}
	if strings.Contains(body, labels.UsersEmpty) {
		t.Error("a search that matches nobody claims the section holds no users")
	}
}

func TestUsersSearchAnswersHTMXWithTableAlone(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	body := testkit.GetAsHTMX(t, d, paths.Users+"?q=lovelace", cookie).Body.String()
	if !strings.Contains(body, testkit.TestEmail) {
		t.Fatalf("the fragment does not hold the matching row: %s", body)
	}
	for _, chrome := range []string{"<!doctype html>", "<header", "<main", labels.NavLogOut} {
		if strings.Contains(strings.ToLower(body), strings.ToLower(chrome)) {
			t.Errorf("the fragment repeats the chrome: %q", chrome)
		}
	}
}

func TestUsersListStillWholePageForBoostedRequest(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	r := httptest.NewRequest(http.MethodGet, paths.Users, nil)
	r.Header.Set("HX-Request", "true")
	r.Header.Set("HX-History-Restore-Request", "true")
	r.AddCookie(cookie)

	rec := httptest.NewRecorder()
	pilamhttp.NewRouter(d).ServeHTTP(rec, r)

	if !strings.Contains(rec.Body.String(), "<main") {
		t.Error("a history restore gets a fragment rather than the page")
	}
}

func TestUsersListCarriesNoHash(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	if body := testkit.GetAs(t, d, paths.Users, cookie).Body.String(); strings.Contains(body, "$argon2id$") {
		t.Error("the list carries a password hash to the browser")
	}
}

func TestOnlyRootSeesTheSection(t *testing.T) {
	d := testkit.NewDeps(t)

	root := testkit.GetAs(t, d, paths.Models, testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd))
	if !strings.Contains(root.Body.String(), `href="`+paths.Users+`"`) {
		t.Error("the board does not offer a root the users section")
	}

	member := testkit.GetAs(t, d, paths.Models, testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd))
	if strings.Contains(member.Body.String(), `href="`+paths.Users+`"`) {
		t.Error("the board offers a member a section they cannot reach")
	}
}

func TestCreateUserShowsPasswordOnceAndUserLogsIn(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	const email = "alan@example.com"
	form := userForm(email, "Alan", "Turing", auth.MemberRole, true)
	rec := testkit.PostAs(t, d, paths.Users, form, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	passwd := passwordOf(t, rec)
	if passwd == "" {
		t.Fatal("the page shows an empty password")
	}

	// The new user logs in with it, and lands on the change screen, because a
	// first password is expired from the start.
	login := testkit.Login(t, d, email, passwd)
	if login.Code != http.StatusSeeOther {
		t.Fatalf("login status = %d, want %d", login.Code, http.StatusSeeOther)
	}
	board := testkit.GetAs(t, d, paths.Models, testkit.SessionCookie(t, login))
	if got := board.Header().Get("Location"); got != paths.ChangePassword {
		t.Errorf("Location = %q, want %q", got, paths.ChangePassword)
	}

	// The list holds them from now on.
	if !strings.Contains(testkit.GetAs(t, d, paths.Users, cookie).Body.String(), email) {
		t.Error("the list does not hold the user that was just created")
	}

	if got := trail.Actions(); !slices.Equal(got, []string{audit.ActionCreated}) {
		t.Errorf("the trail holds %v, want one %q", got, audit.ActionCreated)
	}
	if trail.Holds(passwd) {
		t.Error("the trail holds the password")
	}
}

func TestCreateUserRefusesTakenEmail(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	form := userForm(strings.ToUpper(testkit.TestEmail), "Ada", "Lovelace", auth.MemberRole, true)
	rec := testkit.PostAs(t, d, paths.Users, form, cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}

	want := labels.Message(validate.FieldError{Code: validate.Taken})
	if !strings.Contains(rec.Body.String(), want) {
		t.Errorf("the page does not carry %q", want)
	}
	if got := trail.Actions(); len(got) != 0 {
		t.Errorf("the trail holds %v, want nothing", got)
	}
}

func TestCreateUserReportsEmptyFieldsPerField(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

	rec := testkit.PostAs(t, d, paths.Users, userForm("", "", "", auth.MemberRole, true), cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}

	required := labels.Message(validate.FieldError{Code: validate.Required})
	if got := strings.Count(rec.Body.String(), required); got != 3 {
		t.Errorf("%d fields carry %q, want 3", got, required)
	}
}

func TestEditUserWritesNameAndRole(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)
	path := paths.Users + "/" + testkit.TestUserID.String()

	form := userForm("", "Augusta", "King", auth.RootRole, true)
	rec := testkit.PostAs(t, d, path, form, cookie)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got, want := rec.Header().Get("Location"), path+"?saved=1"; got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}

	// The card the browser lands on reports the save. A plain visit does not.
	if saved := testkit.GetAs(t, d, path+"?saved=1", cookie).Body.String(); !strings.Contains(saved, labels.Saved) {
		t.Error("the card after the save does not report it")
	}

	body := testkit.GetAs(t, d, path, cookie).Body.String()
	if strings.Contains(body, labels.Saved) {
		t.Error("a plain visit to the card reports a save")
	}
	for _, want := range []string{`value="Augusta"`, `value="King"`, `value="root" selected`} {
		if !strings.Contains(body, want) {
			t.Errorf("the form does not carry %q", want)
		}
	}

	// The address is the login, so the edit screen shows it and never posts it.
	if !strings.Contains(body, `value="`+testkit.TestEmail+`"`) || !strings.Contains(body, "readonly") {
		t.Error("the edit screen does not hold the address read-only")
	}

	want := []string{audit.ActionNameChanged, audit.ActionNameChanged, audit.ActionRoleChanged}
	if got := trail.Actions(); !slices.Equal(got, want) {
		t.Errorf("the trail holds %v, want %v", got, want)
	}
}

func TestDeactivatingEndsSession(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	root := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)
	member := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	form := userForm("", "Ada", "Lovelace", auth.MemberRole, false)
	rec := testkit.PostAs(t, d, paths.Users+"/"+testkit.TestUserID.String(), form, root)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	board := testkit.GetAs(t, d, paths.Models, member)
	if got := board.Header().Get("Location"); got != paths.Login {
		t.Errorf("Location = %q, want %q", got, paths.Login)
	}
	if again := testkit.Login(t, d, testkit.TestEmail, testkit.TestPasswd); again.Code != http.StatusUnauthorized {
		t.Errorf("login status = %d, want %d", again.Code, http.StatusUnauthorized)
	}

	if got := trail.Actions(); !slices.Equal(got, []string{audit.ActionDeactivated}) {
		t.Errorf("the trail holds %v, want one %q", got, audit.ActionDeactivated)
	}
}

func TestRootCannotRemoveTheirOwnAccess(t *testing.T) {
	cases := map[string]url.Values{
		"own deactivation": userForm("", "Barbara", "Liskov", auth.RootRole, false),
		"own root role":    userForm("", "Barbara", "Liskov", auth.MemberRole, true),
	}

	for name, form := range cases {
		t.Run(name, func(t *testing.T) {
			d := testkit.NewDeps(t)
			cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)

			rec := testkit.PostAs(t, d, paths.Users+"/"+testkit.RootUserID.String(), form, cookie)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
			}
			if !strings.Contains(rec.Body.String(), labels.UsersSelfLockout) {
				t.Error("the page does not say why the edit was refused")
			}

			// The section is still theirs.
			if list := testkit.GetAs(t, d, paths.Users, cookie); list.Code != http.StatusOK {
				t.Errorf("list status = %d, want %d", list.Code, http.StatusOK)
			}
		})
	}
}

func TestResetPasswordHandsOutNewFirstPassword(t *testing.T) {
	d, trail := testkit.AuditedDeps(t)
	root := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)
	member := testkit.LoggedIn(t, d, testkit.TestEmail, testkit.TestPasswd)

	rec := testkit.PostAs(t, d, paths.Users+"/"+testkit.TestUserID.String()+"/password", nil, root)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	passwd := passwordOf(t, rec)

	// The old password is gone, and every session of that user with it.
	if old := testkit.Login(t, d, testkit.TestEmail, testkit.TestPasswd); old.Code != http.StatusUnauthorized {
		t.Errorf("the old password still logs in: status = %d", old.Code)
	}
	if board := testkit.GetAs(t, d, paths.Models, member); board.Header().Get("Location") != paths.Login {
		t.Error("the reset left the other device logged in")
	}

	// The new one logs in, and holds the user on the change screen.
	fresh := testkit.Login(t, d, testkit.TestEmail, passwd)
	if fresh.Code != http.StatusSeeOther {
		t.Fatalf("login status = %d, want %d", fresh.Code, http.StatusSeeOther)
	}
	board := testkit.GetAs(t, d, paths.Models, testkit.SessionCookie(t, fresh))
	if got := board.Header().Get("Location"); got != paths.ChangePassword {
		t.Errorf("Location = %q, want %q", got, paths.ChangePassword)
	}

	if got := trail.Actions(); !slices.Equal(got, []string{audit.ActionPasswdReset}) {
		t.Errorf("the trail holds %v, want one %q", got, audit.ActionPasswdReset)
	}
	if trail.Holds(passwd) {
		t.Error("the trail holds the password")
	}
}

func TestUsersSectionReportsUnknownUser(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)
	gone := ids.MustParse("01912345-6789-7abc-def0-1234567890ff")

	for _, r := range []testkit.Route{
		{Method: http.MethodGet, Path: paths.Users + "/" + gone.String()},
		{Method: http.MethodPost, Path: paths.Users + "/" + gone.String()},
		{Method: http.MethodPost, Path: paths.Users + "/" + gone.String() + "/password"},
		{Method: http.MethodGet, Path: paths.Users + "/not-a-uuid"},
	} {
		rec := testkit.Call(t, d, r, userForm("", "Ada", "Lovelace", auth.MemberRole, true), cookie)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s status = %d, want %d", r.Method, r.Path, rec.Code, http.StatusNotFound)
		}
	}
}

func TestCreateScreenNamesFieldsThatServiceRejects(t *testing.T) {
	pairs := map[string]string{
		views.FieldUserEmail:     auth.FieldEmail,
		views.FieldUserFirstName: auth.FieldFirstName,
		views.FieldUserLastName:  auth.FieldLastName,
		views.FieldUserRole:      auth.FieldRole,
	}
	for posted, rejected := range pairs {
		if posted != rejected {
			t.Errorf("the view posts %q, the service rejects %q", posted, rejected)
		}
	}
}

// The card of one user carries what the trail recorded about that user: when,
// who, and what changed.
func TestUserCardShowsTheAuditTrail(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)
	card := paths.Users + "/" + testkit.TestUserID.String()

	// Nothing is recorded about the seeded user yet.
	if body := testkit.GetAs(t, d, card, cookie).Body.String(); !strings.Contains(body, labels.AuditEmpty) {
		t.Error("the card of a user with no history does not say the history is empty")
	}

	// A rename and a role change, both by the root.
	edit := userForm(testkit.TestEmail, "Ada", "Byron", auth.RootRole, true)
	if rec := testkit.PostAs(t, d, card, edit, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("edit status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	body := testkit.GetAs(t, d, card, cookie).Body.String()
	name := labels.UsersAuditLastName + ": Lovelace → Byron"
	role := labels.UsersAuditRole + ": " + labels.UsersRoleMember + " → " + labels.UsersRoleRoot

	for _, want := range []string{
		labels.AuditTitle,
		labels.DateTime(testkit.Now),
		labels.Name("Barbara", "Liskov"),
		name,
		role,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the trail on the card does not hold %q", want)
		}
	}

	// Newest first: the role change was recorded after the rename.
	if strings.Index(body, role) > strings.Index(body, name) {
		t.Error("the trail does not show the newest change first")
	}
}

// A deactivation and a password reset read as the action alone. The flag and
// the marker the entries carry say nothing the action does not.
func TestUserCardReadsTheTrailWithoutTheStoredValues(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)
	card := paths.Users + "/" + testkit.TestUserID.String()

	reset := testkit.PostAs(t, d, card+"/password", nil, cookie)
	if reset.Code != http.StatusOK {
		t.Fatalf("reset status = %d, want %d", reset.Code, http.StatusOK)
	}
	passwd := passwordOf(t, reset)

	off := userForm(testkit.TestEmail, "Ada", "Lovelace", auth.MemberRole, false)
	if rec := testkit.PostAs(t, d, card, off, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("edit status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	body := testkit.GetAs(t, d, card, cookie).Body.String()
	for _, want := range []string{labels.UsersAuditOff, labels.UsersAuditReset} {
		if !strings.Contains(body, want) {
			t.Errorf("the trail on the card does not hold %q", want)
		}
	}
	for _, unwanted := range []string{passwd, audit.Marker, "true", "false"} {
		if strings.Contains(body, ": "+unwanted) {
			t.Errorf("the trail on the card reads out the stored value %q", unwanted)
		}
	}
}

// A refused edit comes back with the trail still under the form.
func TestRefusedEditKeepsTheTrailOnTheCard(t *testing.T) {
	d := testkit.NewDeps(t)
	cookie := testkit.LoggedIn(t, d, testkit.RootEmail, testkit.RootPasswd)
	card := paths.Users + "/" + testkit.TestUserID.String()

	named := userForm(testkit.TestEmail, "Ada", "Byron", auth.MemberRole, true)
	if rec := testkit.PostAs(t, d, card, named, cookie); rec.Code != http.StatusSeeOther {
		t.Fatalf("edit status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	empty := userForm(testkit.TestEmail, "", "Byron", auth.MemberRole, true)
	rec := testkit.PostAs(t, d, card, empty, cookie)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(rec.Body.String(), labels.UsersAuditLastName+": Lovelace → Byron") {
		t.Error("the refused edit comes back without the trail")
	}
}
