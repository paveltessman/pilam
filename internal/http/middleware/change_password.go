package middleware

import (
	"net/http"

	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/platform/logging"
)

// RequirePasswordChange holds a user whose password expired on changePath.
// Every other route it guards sends them there, so the change screen is the
// only screen they reach. Logout stays outside the guard.
func RequirePasswordChange(changePath string) Middleware {
	middleware := func(next http.Handler) http.Handler {
		handle := func(w http.ResponseWriter, r *http.Request) {
			identity, ok := auth.FromContext(r.Context())
			if !ok || !identity.PasswdExpired || r.URL.Path == changePath {
				next.ServeHTTP(w, r)
				return
			}

			logging.FromContext(r.Context()).Info("password expired, sending the user to the change screen",
				"path", r.URL.Path)
			SendTo(w, r, changePath)
		}
		return http.HandlerFunc(handle)
	}
	return middleware
}
