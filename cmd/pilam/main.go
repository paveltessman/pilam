package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/paveltessman/pilam/internal/platform/config"
	"github.com/paveltessman/pilam/internal/platform/logging"
)

var errNotImplemented = errors.New("not implemented yet")

type command struct {
	name    string
	summary string
	run     func(ctx context.Context, cfg config.Config, args []string) error
}

func commands() []command {
	commands := []command{
		{"serve", "run the HTTP server", runServe},
		{"migrate", "apply or roll back database migrations", func(context.Context, config.Config, []string) error {
			return fmt.Errorf("migrate: %w", errNotImplemented)
		}},
		{"seed", "load the demo dataset", func(context.Context, config.Config, []string) error {
			return fmt.Errorf("seed: %w", errNotImplemented)
		}},
	}
	return commands
}

func main() {
	// Configuration is what says how to log, so the failure to read it has to be
	// reportable before that answer exists. run installs the configured logger
	// over this one as soon as it has loaded.
	slog.SetDefault(logging.New(logging.Options{
		Level:  slog.LevelInfo,
		Format: logging.FormatText,
	}))

	// every subcommand shuts down from there.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:]); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		usage()
		return errors.New("no subcommand given")
	}

	for _, c := range commands() {
		if c.name == args[0] {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			slog.SetDefault(logging.New(logging.Options{
				Level:  cfg.Log.Level,
				Format: cfg.Log.Format,
			}))
			if cfg.Session.Generated {
				slog.Warn("SESSION_SECRET is unset: sessions are signed with a key made at boot, so restarting logs everyone out")
			}
			return c.run(ctx, cfg, args[1:])
		}
	}

	usage()
	return fmt.Errorf("unknown subcommand %q", args[0])
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: pilam <command> [flags]")
	fmt.Fprintln(os.Stderr, "\ncommands:")
	for _, c := range commands() {
		fmt.Fprintf(os.Stderr, "  %-8s %s\n", c.name, c.summary)
	}
}
