// Package auth owns who the user is.
//
// v1 is a single shared login with a single role that sees and edits
// everything.
//
// The shape is chosen so real accounts can arrive without a rewrite. Callers
// receive an Identity, never a username string, and Identity already carries a
// Role. Password checking — Authenticate — lands with the login screen.
package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// Role is what an identity is allowed to do. v1 issues exactly one.
type Role string

const RoleManager Role = "manager"

// The value carried in the session cookie
const Subject = "manager"

var ErrUnknownSubject = errors.New("auth: unknown subject")

// Identity is the authenticated user, as every layer below transport sees them.
type Identity struct {
	Subject string
	Role    Role
}

// IsZero reports whether the request is anonymous (not logged in).
func (i Identity) IsZero() bool { return i.Subject == "" }

func (i Identity) LogValue() slog.Value {
	if i.IsZero() {
		return slog.StringValue("anonymous")
	}
	return slog.StringValue(i.Subject)
}

// Resolve returns the identity a session subject names.
func Resolve(_ context.Context, subject string) (Identity, error) {
	if subject != Subject {
		return Identity{}, fmt.Errorf("%w: %q", ErrUnknownSubject, subject)
	}
	return Identity{Subject: Subject, Role: RoleManager}, nil
}

type contextKey struct{}

// NewContext returns a context carrying identity. The identity middleware puts
// it there once per request.
func NewContext(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, contextKey{}, identity)
}

// FromContext returns the identity ctx was resolved to, and whether the request
// is authenticated at all. An anonymous request yields the zero Identity and false.
func FromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(contextKey{}).(Identity)
	if !ok || identity.IsZero() {
		return Identity{}, false
	}
	return identity, true
}
