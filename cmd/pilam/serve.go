package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"time"

	pilamhttp "github.com/paveltessman/pilam/internal/http"
	"github.com/paveltessman/pilam/internal/platform/config"
)

const readHeaderTimeout = 5 * time.Second

func runServe(ctx context.Context, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	slog.Info("configuration", "config", cfg)

	srv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           pilamhttp.NewRouter(),
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
