package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/logging"
	"github.com/paveltessman/pilam/internal/platform/requestid"
)

// Recover turns a panicking handler into a 500 instead of a dropped connection.
func Recover() Middleware {
	middleware := func(next http.Handler) http.Handler {
		handle := func(w http.ResponseWriter, r *http.Request) {
			rec := wrap(w)

			defer func() {
				v := recover()
				if v == nil {
					return
				}
				if v == http.ErrAbortHandler {
					panic(v)
				}

				ctx := r.Context()
				requestId := requestid.FromContext(ctx)
				logger := logging.FromContext(ctx)

				logger.Error("panic recovered",
					"panic", v,
					"stack", string(debug.Stack()),
				)

				// Once bytes are on the wire the status is already sent and the
				// body is already partly rendered; there is no 500 left to
				// write. The log line above is all the report there can be.
				if rec.wrote {
					return
				}
				writeProblem(rec, http.StatusInternalServerError, labels.ErrorUnexpected, requestId)
			}()

			next.ServeHTTP(rec, r)
		}
		return http.HandlerFunc(handle)
	}
	return middleware
}

// writeProblem renders the chain's own refusals — the ones that never reach a
// handler and so have no view model and no template.
func writeProblem(w http.ResponseWriter, status int, message, requestID string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)

	body := message
	if requestID != "" {
		body += "\n" + requestID
	}
	_, _ = w.Write([]byte(body + "\n"))
}
