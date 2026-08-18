// Package requestid carries the identifier of the request in flight.
//
// The transport puts the identifier in the context once per request, and every
// layer below reads it from there. The audit trail stamps it onto each entry
package requestid

import "context"

type contextKey struct{}

// NewContext returns a context carrying id.
func NewContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

// FromContext returns the id of the request in flight, or the empty string
// outside a request.
func FromContext(ctx context.Context) string {
	id, _ := ctx.Value(contextKey{}).(string)
	return id
}
