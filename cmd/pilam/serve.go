package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"time"

	pilamhttp "github.com/paveltessman/pilam/internal/http"
)

const (
	httpAddr      = ":8080"
	shutdownGrace = 10 * time.Second
)

func runServe(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)

	// TODO-P1: replace this with internal/config
	addr := fs.String("addr", httpAddr, "address to listen on")
	if err := fs.Parse(args); err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           pilamhttp.NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errs := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", *addr)
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
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownGrace)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
