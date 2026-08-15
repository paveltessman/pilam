package middleware

import (
	"context"
	"net/http"
)

const (
	headerHXRequest = "HX-Request"
	headerHXBoosted = "HX-Boosted"
	headerHXRestore = "HX-History-Restore-Request"
	headerHXTarget  = "HX-Target"
	headerHXTrigger = "HX-Trigger"
)

type htmxKey struct{}

type HX struct {
	Active  bool
	Boosted bool
	Restore bool
	Target  string
	Trigger string
}

// HTMX records how the request arrived, so that handlers do not read headers.
func HTMX() Middleware {
	middleware := func(next http.Handler) http.Handler {

		handle := func(w http.ResponseWriter, r *http.Request) {
			hx := HX{
				Active:  r.Header.Get(headerHXRequest) == "true",
				Boosted: r.Header.Get(headerHXBoosted) == "true",
				Restore: r.Header.Get(headerHXRestore) == "true",
				Target:  r.Header.Get(headerHXTarget),
				Trigger: r.Header.Get(headerHXTrigger),
			}

			w.Header().Add("Vary", headerHXRequest)

			next.ServeHTTP(w, r.WithContext(newHXContext(r.Context(), hx)))
		}
		return http.HandlerFunc(handle)
	}
	return middleware
}

func newHXContext(ctx context.Context, hx HX) context.Context {
	return context.WithValue(ctx, htmxKey{}, hx)
}

func HXFromContext(ctx context.Context) HX {
	hx, _ := ctx.Value(htmxKey{}).(HX)
	return hx
}

// IsHTMX reports whether htmx issued the request
func IsHTMX(ctx context.Context) bool { return HXFromContext(ctx).Active }

// IsFragment reports whether the response should be the bare component rather
// than a full page.
func IsFragment(ctx context.Context) bool {
	hx := HXFromContext(ctx)
	return hx.Active && !hx.Boosted && !hx.Restore
}
