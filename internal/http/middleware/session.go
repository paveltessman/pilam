package middleware

import (
	"context"
	"net/http"

	"github.com/paveltessman/pilam/internal/platform/logging"
	"github.com/paveltessman/pilam/internal/platform/session"
)

const SessionCookie = "pilam_session"

type sessionKey struct{}

// Session reads the session cookie and, if it verifies, puts the subject it was
// issued for into the context.
//
// A cookie that fails to verify is cleared on the way past, so that an expired
// session stops being re-sent on every subsequent request.
func Session(manager *session.Manager) Middleware {
	if manager == nil {
		panic("middleware: nil session manager")
	}

	middleware := func(next http.Handler) http.Handler {

		process := func(w http.ResponseWriter, r *http.Request) {
			logger := logging.FromContext(r.Context())

			cookie, err := r.Cookie(SessionCookie)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			subject, err := manager.Verify(cookie.Value)
			if err != nil {
				logger.Debug("discarding session cookie", "err", err)
				ClearSession(w, r)
				next.ServeHTTP(w, r)
				return
			}

			next.ServeHTTP(w, r.WithContext(newSubjectContext(r.Context(), subject)))
		}
		return http.HandlerFunc(process)
	}
	return middleware
}

func SetSession(w http.ResponseWriter, r *http.Request, manager *session.Manager, subject string) {
	c := cookie(r, manager.Issue(subject), int(manager.TTL().Seconds()))
	http.SetCookie(w, c)
}

func ClearSession(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, cookie(r, "", -1))
}

// SubjectFromContext returns the subject of the verified session on this
// request, and whether there was one.
func SubjectFromContext(ctx context.Context) (string, bool) {
	subject, ok := ctx.Value(sessionKey{}).(string)
	return subject, ok && subject != ""
}

func cookie(r *http.Request, value string, maxAge int) *http.Cookie {
	c := &http.Cookie{
		Name:     SessionCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure(r),
	}
	return c
}

// secure reports whether the request arrived over TLS, directly or through a
// terminating proxy.
//
// Derived per request rather than configured, so the same binary is correct
// both behind TLS and on the plain-http localhost.
func secure(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

func newSubjectContext(ctx context.Context, subject string) context.Context {
	return context.WithValue(ctx, sessionKey{}, subject)
}
