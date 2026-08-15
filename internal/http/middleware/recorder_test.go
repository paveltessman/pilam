package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWrapIsIdempotent(t *testing.T) {
	first := wrap(httptest.NewRecorder())
	if second := wrap(first); second != first {
		t.Error("wrapping an already-wrapped response made a second recorder")
	}
}

func TestRecorderRemembersTheStatusAndSize(t *testing.T) {
	rec := wrap(httptest.NewRecorder())
	rec.WriteHeader(http.StatusCreated)
	_, _ = io.WriteString(rec, "twelve bytes")

	if rec.status != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.status, http.StatusCreated)
	}
	if rec.written != 12 {
		t.Errorf("written = %d, want 12", rec.written)
	}
}

func TestRecorderDefaultsToOKOnTheFirstWrite(t *testing.T) {
	rec := wrap(httptest.NewRecorder())
	_, _ = io.WriteString(rec, "body")

	if rec.status != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.status, http.StatusOK)
	}
	if !rec.wrote {
		t.Error("the response is not marked as started")
	}
}

func TestRecorderKeepsTheStatusThatWentOut(t *testing.T) {
	underlying := httptest.NewRecorder()
	rec := wrap(underlying)
	rec.WriteHeader(http.StatusNotFound)
	rec.WriteHeader(http.StatusOK)

	if rec.status != http.StatusNotFound {
		t.Errorf("recorded status = %d, want %d", rec.status, http.StatusNotFound)
	}
	if underlying.Code != http.StatusNotFound {
		t.Errorf("sent status = %d, want %d", underlying.Code, http.StatusNotFound)
	}
}

func TestRecorderStaysAReaderFrom(t *testing.T) {
	underlying := httptest.NewRecorder()
	rec := wrap(underlying)

	var from io.ReaderFrom = rec
	n, err := from.ReadFrom(strings.NewReader("streamed"))
	if err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	if n != 8 || rec.written != 8 {
		t.Errorf("n/written = %d/%d, want 8/8", n, rec.written)
	}
	if got := underlying.Body.String(); got != "streamed" {
		t.Errorf("body = %q, want %q", got, "streamed")
	}
	if rec.status != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.status, http.StatusOK)
	}
}

func TestRecorderKeepsTheResponseControllerWorking(t *testing.T) {
	underlying := httptest.NewRecorder()
	rec := wrap(underlying)
	_, _ = io.WriteString(rec, "partial")

	if err := http.NewResponseController(rec).Flush(); err != nil {
		t.Fatalf("Flush through the wrapper: %v", err)
	}
	if !underlying.Flushed {
		t.Error("the flush did not reach the underlying response")
	}
}
