// Package http is the transport layer: router, middleware, handlers and views.
package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/a-h/templ"

	"github.com/paveltessman/pilam/internal/http/middleware"
	"github.com/paveltessman/pilam/internal/http/static"
	"github.com/paveltessman/pilam/internal/http/views"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/logging"
	"github.com/paveltessman/pilam/internal/platform/session"
)

// How long the health check gives the database to answer.
const healthTimeout = 2 * time.Second

// Pinger is the health check's view of the database. Satisfied by *postgres.DB.
type Pinger interface {
	Ping(ctx context.Context) error
}

type Deps struct {
	DB              Pinger
	Logger          *slog.Logger
	IDs             ids.Generator
	SessionMgr      *session.Manager
	ResolveIdentity middleware.ResolveIdentityFunc
}

func NewRouter(deps Deps) http.Handler {
	switch {
	case deps.DB == nil:
		panic("http: nil database")
	case deps.Logger == nil:
		panic("http: nil logger")
	case deps.IDs == nil:
		panic("http: nil id generator")
	case deps.SessionMgr == nil:
		panic("http: nil session manager")
	case deps.ResolveIdentity == nil:
		panic("http: nil identity resolver")
	}

	mux := http.NewServeMux()

	mux.Handle("GET /static/", http.StripPrefix("/static/", static.Handler()))
	mux.HandleFunc("GET /healthz", reportHealth(deps.DB))
	mux.Handle("GET /{$}", templ.Handler(views.Home()))

	// The order of middleware chain:
	//   - request id first, so that every line the logger writes is tagged with it;
	//   - recover inside the logger, so a panicking request still produces its
	//     completion line, with the 500 it actually returned;
	//   - session before identity;
	//   - identity resolves what session read;
	//   - CSRF after identity, so a rejected cross-origin write is logged with the
	//     actor that attempted it;
	//   - HTMX last — it only records what the request said about itself.
	chain := middleware.Chain(
		middleware.RequestID(deps.IDs),
		middleware.Logger(deps.Logger),
		middleware.Recover(),
		middleware.Session(deps.SessionMgr),
		middleware.Identity(deps.ResolveIdentity),
		middleware.CSRF(),
		middleware.HTMX(),
	)

	return chain(mux)
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
