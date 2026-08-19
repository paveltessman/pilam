package middleware

import (
	"net/http"

	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/logging"
	"github.com/paveltessman/pilam/internal/platform/requestid"
)

// RequireRole refuses a user whose role does not cover required. A root covers
// every role, so RequireRole(auth.RootRole) is the narrowest guard there is.
func RequireRole(required auth.Role) Middleware {
	middleware := func(next http.Handler) http.Handler {
		handle := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			identity, ok := auth.FromContext(ctx)
			if ok && identity.Role.Allows(required) {
				next.ServeHTTP(w, r)
				return
			}

			logging.FromContext(ctx).Warn("request refused by role",
				"path", r.URL.Path, "role", identity.Role, "required", required)
			writeProblem(w, http.StatusForbidden, labels.ErrorForbidden, requestid.FromContext(ctx))
		}
		return http.HandlerFunc(handle)
	}
	return middleware
}
