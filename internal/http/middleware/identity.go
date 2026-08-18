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

			// A 303 would be swapped into the page as HTML by HTMX, because the
			// browser follows the redirect transparently and hands htmx the
			// login page as if it were the fragment that was asked for.
			// HX-Redirect navigates the whole window instead, which is what an
			// expired session should do.
			if IsHTMX(r.Context()) {
				w.Header().Set("HX-Redirect", loginPath)
				w.WriteHeader(http.StatusOK)
				return
			}

			http.Redirect(w, r, loginPath, http.StatusSeeOther)
		}
		return http.HandlerFunc(handle)
	}
	return middleware
}
