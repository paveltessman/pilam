package middleware

import (
	"context"
	"net/http"

	"github.com/paveltessman/pilam/internal/platform/ids"
)

const RequestIDHeader = "X-Request-Id"

type requestIDKey struct{}

// RequestID tags each request with a fresh identifier and echoes it back.
func RequestID(gen ids.Generator) Middleware {
	if gen == nil {
		panic("middleware: nil id generator")
	}

	middleware := func(next http.Handler) http.Handler {
		handler := func(w http.ResponseWriter, r *http.Request) {
			id := gen.New().String()
			w.Header().Set(RequestIDHeader, id)
			next.ServeHTTP(w, r.WithContext(NewRequestIDContext(r.Context(), id)))
		}
		return http.HandlerFunc(handler)
	}
	return middleware
}

// NewRequestIDContext returns a context carrying id.
func NewRequestIDContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestIDFromContext returns the id assigned to this request, or the empty
// string outside a request.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}
