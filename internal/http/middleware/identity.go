package middleware

import (
	"context"
	"net/http"

	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/platform/logging"
)

type Resolver func(ctx context.Context, subject string) (auth.Identity, error)

// Identity resolves the session's subject to the user it names and puts them in
// the context.
func Identity(resolver Resolver) Middleware {
	if resolver == nil {
		panic("middleware: nil identity resolver")
	}

	middleware := func(next http.Handler) http.Handler {
		handle := func(w http.ResponseWriter, r *http.Request) {
			logger := logging.FromContext(r.Context())

			subject, ok := SubjectFromContext(r.Context())
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			identity, err := resolver(r.Context(), subject)
			if err != nil {
				logger.Warn("session names an unknown subject",
					"subject", subject, "err", err)
				ClearSession(w, r)
				next.ServeHTTP(w, r)
				return
			}

			// The request logger widens here: every line a handler writes from
			// now on says who it was for.
			ctx := auth.NewContext(r.Context(), identity)
			ctx = logging.With(ctx, "actor", identity)

			next.ServeHTTP(w, r.WithContext(ctx))
		}
		return http.HandlerFunc(handle)
	}
	return middleware
}

// RequireIdentity refuses anonymous requests, sending them to loginPath.
func RequireIdentity(loginPath string) Middleware {
	middleware := func(next http.Handler) http.Handler {
		handle := func(w http.ResponseWriter, r *http.Request) {

			if _, ok := auth.FromContext(r.Context()); ok {
				next.ServeHTTP(w, r)
				return
			}

			SendTo(w, r, loginPath)
		}
		return http.HandlerFunc(handle)
	}
	return middleware
}
