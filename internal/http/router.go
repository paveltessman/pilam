// Package http is the transport layer: router, middleware, handlers and views.
package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/http/middleware"
	"github.com/paveltessman/pilam/internal/http/static"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/logging"
	"github.com/paveltessman/pilam/internal/platform/media"
	"github.com/paveltessman/pilam/internal/platform/session"
)

// How long the health check gives the database to answer.
const healthTimeout = 2 * time.Second

// Pinger is the health check's view of the database. Satisfied by *postgres.DB.
type Pinger interface {
	Ping(ctx context.Context) error
}

type Deps struct {
	DB         Pinger
	Logger     *slog.Logger
	IDs        ids.Generator
	Media      media.Store
	SessionMgr *session.Manager
	AuthSvc    *auth.Service
	CatalogSvc *catalog.Service
	AuditLog   *audit.Log
}

func NewRouter(deps Deps) http.Handler {
	switch {
	case deps.DB == nil:
		panic("http: nil database")
	case deps.Logger == nil:
		panic("http: nil logger")
	case deps.IDs == nil:
		panic("http: nil id generator")
	case deps.Media == nil:
		panic("http: nil media store")
	case deps.SessionMgr == nil:
		panic("http: nil session manager")
	case deps.AuthSvc == nil:
		panic("http: nil auth service")
	case deps.CatalogSvc == nil:
		panic("http: nil catalog service")
	case deps.AuditLog == nil:
		panic("http: nil audit log")
	}

	mux := http.NewServeMux()

	mux.Handle("GET /static/", http.StripPrefix("/static/", static.Handler()))
	mux.HandleFunc("GET /healthz", reportHealth(deps.DB))
	mux.HandleFunc("GET /media/{key...}", serveMedia(deps.Media))

	mux.HandleFunc("GET "+loginPath, showLogin())
	mux.HandleFunc("POST "+loginPath, submitLogin(deps.SessionMgr, deps.AuthSvc))
	mux.HandleFunc("POST "+logoutPath, submitLogout())

	// Every screen from here down needs a login, and a password that is not
	// expired. The guard lets the change screen itself through, and logout
	// stays above the guard, so a user with an expired password reaches those
	// two and nothing else.
	guard := middleware.Chain(
		middleware.RequireIdentity(loginPath),
		middleware.RequirePasswordChange(changePasswordPath),
	)
	// The root path holds no screen. It sends the browser to the model list.
	mux.Handle("GET /{$}", guard(redirectTo(modelsPath)))
	mux.Handle("GET "+changePasswordPath, guard(showChangePassword()))
	mux.Handle("POST "+changePasswordPath, guard(submitChangePassword(deps.SessionMgr, deps.AuthSvc)))

	mux.Handle("GET "+modelsPath, guard(showModels(deps.CatalogSvc, deps.Media)))
	mux.Handle("POST "+modelsPath, guard(createModel(deps.CatalogSvc)))
	mux.Handle("GET "+modelNewPath, guard(showNewModel(deps.CatalogSvc)))
	mux.Handle("GET "+modelPath, guard(showModel(deps.CatalogSvc, deps.AuthSvc, deps.AuditLog, deps.Media)))
	mux.Handle("POST "+modelPath, guard(saveModel(deps.CatalogSvc, deps.AuthSvc, deps.AuditLog, deps.Media)))
	mux.Handle("POST "+modelPhotosPath, guard(addModelPhotos(deps.CatalogSvc, deps.AuthSvc, deps.AuditLog, deps.Media)))
	mux.Handle("POST "+modelOrderPath, guard(reorderModelPhotos(deps.CatalogSvc, deps.AuthSvc, deps.AuditLog, deps.Media)))
	mux.Handle("POST "+modelPhotoPath, guard(removeModelPhoto(deps.CatalogSvc)))

	// S5, the users section. Root only: a member gets a 403 on every route of
	// it, and never sees the nav link that leads here.
	root := middleware.Chain(guard, middleware.RequireRole(auth.RootRole))
	mux.Handle("GET "+usersPath, root(showUsers(deps.AuthSvc)))
	mux.Handle("POST "+usersPath, root(createUser(deps.AuthSvc)))
	mux.Handle("GET "+userNewPath, root(showNewUser()))
	mux.Handle("GET "+userPath, root(showUser(deps.AuthSvc, deps.AuditLog)))
	mux.Handle("POST "+userPath, root(saveUser(deps.AuthSvc, deps.AuditLog)))
	mux.Handle("POST "+userPassPath, root(resetUserPasswd(deps.AuthSvc)))

	mux.Handle("GET "+seasonsPath, root(showSeasons(deps.CatalogSvc)))
	mux.Handle("POST "+seasonsPath, root(createSeason(deps.CatalogSvc)))
	mux.Handle("GET "+seasonNewPath, root(showNewSeason()))
	mux.Handle("GET "+seasonPath, root(showSeason(deps.CatalogSvc, deps.AuthSvc, deps.AuditLog)))
	mux.Handle("POST "+seasonPath, root(saveSeason(deps.CatalogSvc, deps.AuthSvc, deps.AuditLog)))

	mux.Handle("GET "+dropsPath, root(showDrops(deps.CatalogSvc)))
	mux.Handle("POST "+dropsPath, root(createDrop(deps.CatalogSvc)))
	mux.Handle("GET "+dropNewPath, root(showNewDrop(deps.CatalogSvc)))
	mux.Handle("GET "+dropPath, root(showDrop(deps.CatalogSvc, deps.AuthSvc, deps.AuditLog)))
	mux.Handle("POST "+dropPath, root(saveDrop(deps.CatalogSvc, deps.AuthSvc, deps.AuditLog)))

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
		middleware.Identity(deps.AuthSvc.Resolve),
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
