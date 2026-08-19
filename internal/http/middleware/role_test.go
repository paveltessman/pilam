package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/labels"
)

// as returns a request already resolved to a user of that role, as the identity
// middleware would have left it.
func as(role auth.Role) *http.Request {
	identity := auth.Identity{
		UserID: ids.MustParse("01912345-6789-7abc-def0-123456789abc"),
		Email:  "ada@example.com",
		Role:   role,
	}
	r := httptest.NewRequest(http.MethodGet, "/users", nil)
	return r.WithContext(auth.NewContext(r.Context(), identity))
}

// reached records that the request got past the guard.
func reached(got *bool) http.HandlerFunc {
	return func(http.ResponseWriter, *http.Request) { *got = true }
}

func TestRequireRoleLetsTheRoleThrough(t *testing.T) {
	cases := map[string]struct {
		held, required auth.Role
	}{
		"the role itself": {held: auth.RootRole, required: auth.RootRole},
		"root elsewhere":  {held: auth.RootRole, required: auth.MemberRole},
		"member of it":    {held: auth.MemberRole, required: auth.MemberRole},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			var got bool
			rec := serve(RequireRole(c.required), as(c.held), reached(&got))

			if !got {
				t.Error("the guard refused the request")
			}
			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
			}
		})
	}
}

func TestRequireRoleRefusesNarrowerRole(t *testing.T) {
	var got bool
	rec := serve(RequireRole(auth.RootRole), as(auth.MemberRole), reached(&got))

	if got {
		t.Error("the guard let the request through")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if !strings.Contains(rec.Body.String(), labels.ErrorForbidden) {
		t.Errorf("body = %q, want %q", rec.Body.String(), labels.ErrorForbidden)
	}
}

func TestRequireRoleRefusesAnonymousRequest(t *testing.T) {
	var got bool
	r := httptest.NewRequest(http.MethodGet, "/users", nil)
	rec := serve(RequireRole(auth.RootRole), r, reached(&got))

	if got {
		t.Error("the guard let an anonymous request through")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}
