package logging

import (
	"context"
	"log/slog"
)

type contextKey struct{}

// NewContext returns a context carrying logger. The logger middleware puts the
// request-scoped logger — the one already tagged with the request id — here,
// and every layer below reaches it through FromContext.
func NewContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, logger)
}

// FromContext returns the logger carried by ctx, or slog's default logger when
// there is none. It never returns nil, so callers never guard the call site.
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(contextKey{}).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return slog.Default()
}

// With derives a context whose logger carries args in addition to whatever the
// current one has. This is how the chain accumulates: request id in one
// middleware, actor in the next, without either knowing about the other.
func With(ctx context.Context, args ...any) context.Context {
	if len(args) == 0 {
		return ctx
	}
	return NewContext(ctx, FromContext(ctx).With(args...))
}
