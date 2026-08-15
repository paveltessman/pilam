// Package middleware holds the per-request links the router wraps every handler in.
package middleware

import "net/http"

type Middleware func(http.Handler) http.Handler

// Chain composes middlewares into one, outermost first: the handler produced by
// Chain(a, b, c) enters a, then b, then c, then the wrapped handler, and
// unwinds in reverse.
func Chain(middlewares ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}
