// Package http is the transport layer: router, middleware, handlers and views.
package http

import (
	"context"
	"net/http"
	"time"

	"github.com/a-h/templ"

	"github.com/paveltessman/pilam/internal/http/static"
	"github.com/paveltessman/pilam/internal/http/views"
	"github.com/paveltessman/pilam/internal/platform/logging"
)

// How long the health check gives the database to answer.
const healthTimeout = 2 * time.Second

// Pinger is the health check's view of the database. Satisfied by *postgres.DB.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Deps is everything the router hands to handlers, built in the composition root.
type Deps struct {
	DB Pinger
}

func NewRouter(deps Deps) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /static/", http.StripPrefix("/static/", static.Handler()))
	mux.HandleFunc("GET /healthz", reportHealth(deps.DB))
	mux.Handle("GET /{$}", templ.Handler(views.Home()))

	return mux
}

// reportHealth answers with the state of the dependencies the process cannot
// serve without. Right now that is the database and nothing else.
//
// The reason it round-trips the pool rather than returning a constant is that a
// process which is listening but cannot reach Postgres is not healthy, and the
// whole point of the endpoint is to tell those two states apart.
func reportHealth(db Pinger) http.HandlerFunc {
	f := func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), healthTimeout)
		defer cancel()

		status, body := http.StatusOK, `{"status":"ok"}`
		if err := db.Ping(ctx); err != nil {
			logging.FromContext(ctx).Error("health check failed", "err", err)
			status, body = http.StatusServiceUnavailable, `{"status":"bad", "reason": "postgres is not here"}`
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
	return f
}
