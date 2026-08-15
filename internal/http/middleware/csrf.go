package middleware

import (
	"net/http"

	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/logging"
)

// CSRF rejects cross-origin state-changing requests.
// It delegates to net/http's CrossOriginProtection
func CSRF(trustedOrigins ...string) Middleware {
	protection := http.NewCrossOriginProtection()
	for _, origin := range trustedOrigins {
		if err := protection.AddTrustedOrigin(origin); err != nil {
			panic("middleware: trusted origin " + origin + ": " + err.Error())
		}
	}
	protection.SetDenyHandler(http.HandlerFunc(deny))
	return protection.Handler
}

func deny(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)

	logger.Warn("cross-origin request rejected",
		"method", r.Method,
		"path", r.URL.Path,
		"origin", r.Header.Get("Origin"),
		"sec_fetch_site", r.Header.Get("Sec-Fetch-Site"),
	)
	writeProblem(w, http.StatusForbidden, labels.ErrorForbidden, RequestIDFromContext(ctx))
}
