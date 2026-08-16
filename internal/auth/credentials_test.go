package auth

import (
	"testing"

	"github.com/paveltessman/pilam/internal/platform/validate"
)

func TestAuthenticateAcceptsTheSharedCredential(t *testing.T) {
	identity, err := Authenticate(t.Context(), Username, Password)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if identity.Subject != Subject || identity.Role != RoleManager {
		t.Errorf("identity = %+v, want the manager", identity)
	}
}

func TestAuthenticateRejectsEverythingElse(t *testing.T) {
	cases := map[string]struct{ username, password string }{
		"wrong password":       {Username, Password + "!"},
		"wrong username":       {"someone-else", Password},
		"both wrong":           {"someone-else", "guess"},
		"empty":                {"", ""},
		"username as password": {Username, Username},
		"different case":       {"MANAGER", Password},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			identity, err := Authenticate(t.Context(), c.username, c.password)
			if !identity.IsZero() {
				t.Errorf("identity = %+v, want the zero identity", identity)
			}

			errs, ok := validate.From(err)
			if !ok {
				t.Fatalf("error = %v, want validate.FieldErrors", err)
			}

			want := validate.FieldErrors{{Code: validate.Incorrect}}
			if len(errs) != len(want) || errs[0] != want[0] {
				t.Errorf("errors = %v, want %v", errs, want)
			}
		})
	}
}

// A wrong username and a wrong password must be one answer, against the form
// and not against either input. Anything else tells a guess which half to keep.
func TestAuthenticateBlamesNeitherField(t *testing.T) {
	_, err := Authenticate(t.Context(), "someone-else", Password)

	errs, _ := validate.From(err)
	for _, e := range errs {
		if e.Field != "" {
			t.Errorf("rejection names the field %q, want the form", e.Field)
		}
	}
}
