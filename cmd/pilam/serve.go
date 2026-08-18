package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"time"

	"github.com/paveltessman/pilam/internal/auth"
	pilamhttp "github.com/paveltessman/pilam/internal/http"
	"github.com/paveltessman/pilam/internal/platform/clock"
	"github.com/paveltessman/pilam/internal/platform/config"
	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/media"
	"github.com/paveltessman/pilam/internal/platform/session"
	"github.com/paveltessman/pilam/internal/postgres"
)

const readHeaderTimeout = 5 * time.Second

func runServe(ctx context.Context, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	slog.Info("configuration", "config", cfg)

	db, err := postgres.Open(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer db.Close()
	slog.Info("connected to postgres")

	mediaStore, err := media.New(cfg.Media)
	if err != nil {
		return err
	}
	slog.Info("media store ready", "dir", cfg.Media.Dir)

	clk := clock.New(cfg.Timezone)
	sessionMgr := session.New(cfg.Session.Secret, cfg.Session.TTL, clk)
	idGen := ids.NewGenerator()
	authSvc := auth.NewService(postgres.NewUsers(db), db, auth.NewThrottle(clk), idGen)

	srv := &http.Server{
		Addr: cfg.HTTP.Addr,
		Handler: pilamhttp.NewRouter(pilamhttp.Deps{
			DB:         db,
			Logger:     slog.Default(),
			IDs:        idGen,
			Media:      mediaStore,
			SessionMgr: sessionMgr,
			AuthSvc:    authSvc,
		}),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	errs := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", cfg.HTTP.Addr)
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs <- err
			return
		}
		errs <- nil
	}()

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.HTTP.ShutdownGrace)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
