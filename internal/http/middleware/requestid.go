package middleware

import (
	"net/http"

	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/requestid"
)

const RequestIDHeader = "X-Request-Id"

// RequestID tags each request with a fresh identifier and echoes it back.
//
// The identifier goes into the context, where the layers below the transport
// read it through requestid.FromContext.
func RequestID(gen ids.Generator) Middleware {
	if gen == nil {
		panic("middleware: nil id generator")
	}

	middleware := func(next http.Handler) http.Handler {
		handler := func(w http.ResponseWriter, r *http.Request) {
			id := gen.New().String()
			w.Header().Set(RequestIDHeader, id)
			ctx := requestid.NewContext(r.Context(), id)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
		return http.HandlerFunc(handler)
	}
	return middleware
}
