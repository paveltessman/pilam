package logging_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/platform/logging"
)

func TestFromContextFallsBackToDefault(t *testing.T) {
	if got := logging.FromContext(context.Background()); got != slog.Default() {
		t.Errorf("FromContext on a bare context = %p, want slog.Default() %p", got, slog.Default())
	}
}

func TestFromContextReturnsStoredLogger(t *testing.T) {
	want := logging.New(logging.Options{Format: logging.FormatText, Output: &bytes.Buffer{}})
	ctx := logging.NewContext(context.Background(), want)

	if got := logging.FromContext(ctx); got != want {
		t.Errorf("FromContext = %p, want %p", got, want)
	}
}

// The middleware chain adds attrs one layer at a time — request id, then actor —
// and each layer sees everything the layers above it added.
func TestWithAccumulatesAttrs(t *testing.T) {
	var buf bytes.Buffer
	ctx := logging.NewContext(context.Background(), logging.New(logging.Options{Format: logging.FormatText, Output: &buf}))

	ctx = logging.With(ctx, "request_id", "abc123")
	ctx = logging.With(ctx, "actor", "merch")
	logging.FromContext(ctx).Info("handled")

	out := buf.String()
	for _, want := range []string{"request_id=abc123", "actor=merch", "msg=handled"} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q does not contain %q", out, want)
		}
	}
}

// A derived context must not leak its attrs back up: the parent request's
// logger cannot start carrying a sub-request's fields.
func TestWithDoesNotMutateParent(t *testing.T) {
	var buf bytes.Buffer
	parent := logging.NewContext(context.Background(), logging.New(logging.Options{Format: logging.FormatText, Output: &buf}))

	_ = logging.With(parent, "actor", "merch")
	logging.FromContext(parent).Info("handled")

	if strings.Contains(buf.String(), "actor") {
		t.Errorf("parent logger picked up the child's attr: %q", buf.String())
	}
}

func TestWithNoArgsReturnsSameContext(t *testing.T) {
	parent := context.Background()
	if got := logging.With(parent); got != parent {
		t.Error("With() with no args wrapped the context anyway")
	}
}
