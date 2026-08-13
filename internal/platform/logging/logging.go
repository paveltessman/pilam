// Package logging is the one place slog is configured: level, format, and
// destination. Everything else in the tree takes a *slog.Logger, or pulls the
// request-scoped one out of the context.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
)

type Format string

const (
	// FormatText is line-oriented and meant to be read by a person. The default.
	FormatText Format = "text"
	// FormatJSON is one object per record, for a log collector.
	FormatJSON Format = "json"
)

// Options configures the handler. Level and Format come from config, which has
// already rejected anything unusable; Output defaults to stderr.
type Options struct {
	Level  slog.Level
	Format Format
	Output io.Writer
}

// New builds the application logger.
func New(opts Options) *slog.Logger {
	out := opts.Output
	if out == nil {
		out = os.Stderr
	}

	handlerOpts := &slog.HandlerOptions{Level: opts.Level}

	var h slog.Handler

	switch opts.Format {
	case FormatJSON:
		h = slog.NewJSONHandler(out, handlerOpts)
	case FormatText:
		h = slog.NewTextHandler(out, handlerOpts)
	default:
		panic(fmt.Errorf("unknown logging format value: %s", opts.Format))
	}

	return slog.New(h)
}
