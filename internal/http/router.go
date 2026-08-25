// Package http is the transport layer: the router, and the screens that are not
// yet moved into a section package of their own.
//
// The router is the one place that names every route. It imports a section
// package; a section package never imports it back.
package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/paveltessman/pilam/internal/audit"
	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/catalog"
	"github.com/paveltessman/pilam/internal/http/account"
	"github.com/paveltessman/pilam/internal/http/drops"
	"github.com/paveltessman/pilam/internal/http/middleware"
	milestoneshttp "github.com/paveltessman/pilam/internal/http/milestones"
	"github.com/paveltessman/pilam/internal/http/models"
	"github.com/paveltessman/pilam/internal/http/paths"
	"github.com/paveltessman/pilam/internal/http/seasons"
	"github.com/paveltessman/pilam/internal/http/shared"
	"github.com/paveltessman/pilam/internal/http/static"
	"github.com/paveltessman/pilam/internal/http/users"
	"github.com/paveltessman/pilam/internal/milestones"
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
	DB           Pinger
	Logger       *slog.Logger
	IDs          ids.Generator
	Media        media.Store
	SessionMgr   *session.Manager
	AuthSvc      *auth.Service
	CatalogSvc   *catalog.Service
	MilestoneSvc *milestones.Service
	AuditLog     *audit.Log
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
	case deps.MilestoneSvc == nil:
		panic("http: nil milestone service")
	case deps.AuditLog == nil:
		panic("http: nil audit log")
	}

	mux := http.NewServeMux()

	mux.Handle("GET /static/", http.StripPrefix("/static/", static.Handler()))
	mux.HandleFunc("GET /healthz", reportHealth(deps.DB))
	mux.HandleFunc("GET /media/{key...}", serveMedia(deps.Media))

	mux.HandleFunc("GET "+paths.Login, account.ShowLogin())
	mux.HandleFunc("POST "+paths.Login, account.SubmitLogin(deps.SessionMgr, deps.AuthSvc))
	mux.HandleFunc("POST "+paths.Logout, account.SubmitLogout())

	// Every screen from here down needs a login, and a password that is not
	// expired. The guard lets the change screen itself through, and logout
	// stays above the guard, so a user with an expired password reaches those
	// two and nothing else.
	guard := middleware.Chain(
		middleware.RequireIdentity(paths.Login),
		middleware.RequirePasswordChange(paths.ChangePassword),
	)
	// The root path holds no screen. It sends the browser to the model list.
	mux.Handle("GET "+paths.Root, guard(shared.RedirectTo(paths.Models)))
	mux.Handle("GET "+paths.ChangePassword, guard(account.ShowChangePassword()))
	mux.Handle("POST "+paths.ChangePassword, guard(account.SubmitChangePassword(deps.SessionMgr, deps.AuthSvc)))

	mux.Handle("GET "+paths.Models, guard(models.ShowList(deps.CatalogSvc, deps.Media)))
	mux.Handle("POST "+paths.Models, guard(models.Create(deps.CatalogSvc, deps.MilestoneSvc)))
	mux.Handle("GET "+paths.ModelNew, guard(models.ShowNew(deps.CatalogSvc, deps.MilestoneSvc)))
	mux.Handle("GET "+paths.Model, guard(models.Show(deps.CatalogSvc, deps.MilestoneSvc, deps.AuthSvc, deps.AuditLog, deps.Media)))
	mux.Handle("POST "+paths.Model, guard(models.Save(deps.CatalogSvc, deps.MilestoneSvc, deps.AuthSvc, deps.AuditLog, deps.Media)))
	mux.Handle("POST "+paths.ModelPhotos, guard(models.AddPhotos(deps.CatalogSvc, deps.MilestoneSvc, deps.AuthSvc, deps.AuditLog, deps.Media)))
	mux.Handle("POST "+paths.ModelOrder, guard(models.ReorderPhotos(deps.CatalogSvc, deps.MilestoneSvc, deps.AuthSvc, deps.AuditLog, deps.Media)))
	mux.Handle("POST "+paths.ModelPhoto, guard(models.RemovePhoto(deps.CatalogSvc)))

	// The calendar section of the model card. A member edits milestones, so
	// these sit under the ordinary guard and not under the root one.
	mux.Handle("POST "+paths.ModelTemplate,
		guard(models.ApplyTemplate(deps.CatalogSvc, deps.MilestoneSvc, deps.AuthSvc, deps.AuditLog, deps.Media)))
	mux.Handle("POST "+paths.ModelMilestones,
		guard(models.AddMilestone(deps.CatalogSvc, deps.MilestoneSvc, deps.AuthSvc, deps.AuditLog, deps.Media)))
	mux.Handle("POST "+paths.ModelMilestone,
		guard(models.SaveMilestone(deps.CatalogSvc, deps.MilestoneSvc, deps.AuthSvc, deps.AuditLog, deps.Media)))
	mux.Handle("POST "+paths.ModelMilestonePlan,
		guard(models.MovePlan(deps.CatalogSvc, deps.MilestoneSvc, deps.AuthSvc, deps.AuditLog, deps.Media)))

	// S5, the users section. Root only: a member gets a 403 on every route of
	// it, and never sees the nav link that leads here.
	root := middleware.Chain(guard, middleware.RequireRole(auth.RootRole))
	mux.Handle("GET "+paths.Users, root(users.ShowList(deps.AuthSvc)))
	mux.Handle("POST "+paths.Users, root(users.Create(deps.AuthSvc)))
	mux.Handle("GET "+paths.UserNew, root(users.ShowNew()))
	mux.Handle("GET "+paths.User, root(users.Show(deps.AuthSvc, deps.AuditLog)))
	mux.Handle("POST "+paths.User, root(users.Save(deps.AuthSvc, deps.AuditLog)))
	mux.Handle("POST "+paths.UserPass, root(users.ResetPasswd(deps.AuthSvc)))

	mux.Handle("GET "+paths.Seasons, root(seasons.ShowList(deps.CatalogSvc)))
	mux.Handle("POST "+paths.Seasons, root(seasons.Create(deps.CatalogSvc)))
	mux.Handle("GET "+paths.SeasonNew, root(seasons.ShowNew()))
	mux.Handle("GET "+paths.Season, root(seasons.Show(deps.CatalogSvc, deps.AuthSvc, deps.AuditLog)))
	mux.Handle("POST "+paths.Season, root(seasons.Save(deps.CatalogSvc, deps.AuthSvc, deps.AuditLog)))

	mux.Handle("GET "+paths.Drops, root(drops.ShowList(deps.CatalogSvc)))
	mux.Handle("POST "+paths.Drops, root(drops.Create(deps.CatalogSvc)))
	mux.Handle("GET "+paths.DropNew, root(drops.ShowNew(deps.CatalogSvc)))
	mux.Handle("GET "+paths.Drop, root(drops.Show(deps.CatalogSvc, deps.AuthSvc, deps.AuditLog)))
	mux.Handle("POST "+paths.Drop, root(drops.Save(deps.CatalogSvc, deps.AuthSvc, deps.AuditLog)))

	mux.Handle("GET "+paths.Milestones, root(milestoneshttp.ShowSection(deps.MilestoneSvc)))
	mux.Handle("POST "+paths.MilestoneTypes, root(milestoneshttp.CreateType(deps.MilestoneSvc)))
	mux.Handle("GET "+paths.MilestoneTypeNew, root(milestoneshttp.ShowNewType()))
	mux.Handle("GET "+paths.MilestoneType, root(milestoneshttp.ShowType(deps.MilestoneSvc, deps.AuthSvc, deps.AuditLog)))
	mux.Handle("POST "+paths.MilestoneType, root(milestoneshttp.SaveType(deps.MilestoneSvc, deps.AuthSvc, deps.AuditLog)))

	mux.Handle("POST "+paths.MilestoneTemplates, root(milestoneshttp.CreateTemplate(deps.MilestoneSvc)))
	mux.Handle("GET "+paths.MilestoneTemplateNew, root(milestoneshttp.ShowNewTemplate()))
	mux.Handle("GET "+paths.MilestoneTemplate, root(milestoneshttp.ShowTemplate(deps.MilestoneSvc, deps.AuthSvc, deps.AuditLog)))
	mux.Handle("POST "+paths.MilestoneTemplate, root(milestoneshttp.SaveTemplate(deps.MilestoneSvc, deps.AuthSvc, deps.AuditLog)))
	mux.Handle("POST "+paths.MilestoneItems, root(milestoneshttp.AddItem(deps.MilestoneSvc, deps.AuthSvc, deps.AuditLog)))
	mux.Handle("POST "+paths.MilestoneItemsOrder, root(milestoneshttp.ReorderItems(deps.MilestoneSvc, deps.AuthSvc, deps.AuditLog)))
	mux.Handle("POST "+paths.MilestoneItem, root(milestoneshttp.SaveItem(deps.MilestoneSvc, deps.AuthSvc, deps.AuditLog)))
	mux.Handle("POST "+paths.MilestoneItemRemove, root(milestoneshttp.RemoveItem(deps.MilestoneSvc)))

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
