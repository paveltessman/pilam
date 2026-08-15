package auth_test

import (
	"errors"
	"testing"

	"github.com/paveltessman/pilam/internal/auth"
)

func TestResolveReturnsTheSharedIdentity(t *testing.T) {
	identity, err := auth.Resolve(t.Context(), auth.Subject)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if identity.Subject != auth.Subject {
		t.Errorf("subject = %q, want %q", identity.Subject, auth.Subject)
	}
	if identity.Role != auth.RoleManager {
		t.Errorf("role = %q, want %q", identity.Role, auth.RoleManager)
	}
	if identity.IsZero() {
		t.Error("a resolved identity reports itself anonymous")
	}
}

func TestResolveRejectsAnUnknownSubject(t *testing.T) {
	for _, subject := range []string{"", "someone-else", "MANAGER"} {
		identity, err := auth.Resolve(t.Context(), subject)
		if !errors.Is(err, auth.ErrUnknownSubject) {
			t.Errorf("Resolve(%q) error = %v, want ErrUnknownSubject", subject, err)
		}
		if !identity.IsZero() {
			t.Errorf("Resolve(%q) returned %+v, want the zero identity", subject, identity)
		}
	}
}

// The zero Identity is the anonymous request, and it must never read as a user.
func TestFromContextTreatsTheZeroIdentityAsAnonymous(t *testing.T) {
	if _, ok := auth.FromContext(t.Context()); ok {
		t.Error("an empty context resolved to an identity")
	}

	ctx := auth.NewContext(t.Context(), auth.Identity{})
	if identity, ok := auth.FromContext(ctx); ok {
		t.Errorf("the zero identity resolved to %+v", identity)
	}
}

func TestNewContextRoundTrips(t *testing.T) {
	want := auth.Identity{Subject: auth.Subject, Role: auth.RoleManager}

	got, ok := auth.FromContext(auth.NewContext(t.Context(), want))
	if !ok {
		t.Fatal("the identity did not come back")
	}
	if got != want {
		t.Errorf("identity = %+v, want %+v", got, want)
	}
}

func TestIdentityLogsAsItsSubject(t *testing.T) {
	identity := auth.Identity{Subject: auth.Subject, Role: auth.RoleManager}
	if got := identity.LogValue().String(); got != auth.Subject {
		t.Errorf("LogValue = %q, want %q", got, auth.Subject)
	}
	if got := (auth.Identity{}).LogValue().String(); got != "anonymous" {
		t.Errorf("anonymous LogValue = %q, want %q", got, "anonymous")
	}
}
