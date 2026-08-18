package requestid

import "testing"

func TestFromContextReturnsWhatNewContextCarries(t *testing.T) {
	ctx := NewContext(t.Context(), "abc")

	if got := FromContext(ctx); got != "abc" {
		t.Errorf("FromContext = %q, want %q", got, "abc")
	}
}

func TestFromContextWithoutIDReturnsEmpty(t *testing.T) {
	if got := FromContext(t.Context()); got != "" {
		t.Errorf("FromContext = %q, want the empty string", got)
	}
}

func TestNewContextReplacesTheHeldID(t *testing.T) {
	ctx := NewContext(NewContext(t.Context(), "first"), "second")

	if got := FromContext(ctx); got != "second" {
		t.Errorf("FromContext = %q, want %q", got, "second")
	}
}
