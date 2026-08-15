package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/paveltessman/pilam/internal/platform/logging"
)

// Logger puts a request-scoped logger in the context and writes one line per
// request when the handler returns.
func Logger(base *slog.Logger) Middleware {
	if base == nil {
		panic("middleware: nil logger")
	}

	middleware := func(next http.Handler) http.Handler {
		handle := func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()

			request_id := RequestIDFromContext(r.Context())
			logger := base.With("request_id", request_id)
			ctx := logging.NewContext(r.Context(), logger)
			r = r.WithContext(ctx)

			rec := wrap(w)

			defer func() {
				// Deferred so that a panic still produces a completion line.
				// Recover runs inside this one and has already turned the panic
				// into a 500 by the time this executes.
				logger = logging.FromContext(ctx)
				logger.Log(ctx, levelFor(rec.status), "request",
					"method", r.Method,
					"path", r.URL.Path,
					"status", rec.status,
					"duration_ms", time.Since(started).Milliseconds(),
					"bytes", rec.written,
				)
			}()

			next.ServeHTTP(rec, r)
		}
		return http.HandlerFunc(handle)
	}
	return middleware
}

// levelFor keeps a failing request out of the noise floor: a 500 is something
// someone has to look at, a 404 is a typo in a URL bar.
func levelFor(status int) slog.Level {
	if status >= http.StatusInternalServerError {
		return slog.LevelError
	}
	return slog.LevelInfo
}
