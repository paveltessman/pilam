package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/platform/ids"
	"github.com/paveltessman/pilam/internal/platform/logging"
)

// capture returns a JSON logger writing into buf, plus a decoder for whatever
// records the middleware wrote.
func capture() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	logger := logging.New(logging.Options{Level: slog.LevelDebug, Format: logging.FormatJSON, Output: buf})
	return logger, buf
}

// records decodes every line the logger wrote.
func records(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()

	var out []map[string]any
	for line := range strings.Lines(strings.TrimSpace(buf.String())) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("log line %q is not JSON: %v", line, err)
		}
		out = append(out, record)
	}
	return out
}

// only returns the single record written, failing when there was not exactly one.
func only(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()

	got := records(t, buf)
	if len(got) != 1 {
		t.Fatalf("wrote %d records, want 1: %s", len(got), buf)
	}
	return got[0]
}

func TestLoggerWritesOneCompletionLine(t *testing.T) {
	logger, buf := capture()

	r := httptest.NewRequest(http.MethodPost, "/styles/42", nil)
	serve(Logger(logger), r, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("hello"))
	})

	record := only(t, buf)

	wantedFields := map[string]any{
		"msg":    "request",
		"method": http.MethodPost,
		"path":   "/styles/42",
		"status": float64(http.StatusCreated),
		"bytes":  float64(5),
		"level":  "INFO",
	}
	for field, want := range wantedFields {
		if got := record[field]; got != want {
			t.Errorf("%s = %v, want %v", field, got, want)
		}
	}
	if _, ok := record["duration_ms"]; !ok {
		t.Error("no duration_ms recorded")
	}
}

func TestLoggerReportsTheImplicitStatus(t *testing.T) {
	logger, buf := capture()

	serve(Logger(logger), httptest.NewRequest(http.MethodGet, "/", nil),
		func(http.ResponseWriter, *http.Request) {})

	if got := only(t, buf)["status"]; got != float64(http.StatusOK) {
		t.Errorf("status = %v, want %d", got, http.StatusOK)
	}
}

func TestLoggerRaisesTheLevelForServerErrors(t *testing.T) {
	testData := []struct {
		status int
		want   string
	}{
		{http.StatusOK, "INFO"},
		{http.StatusNotFound, "INFO"},
		{http.StatusUnprocessableEntity, "INFO"},
		{http.StatusInternalServerError, "ERROR"},
		{http.StatusBadGateway, "ERROR"},
	}
	for _, tc := range testData {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			logger, buf := capture()

			serve(Logger(logger), httptest.NewRequest(http.MethodGet, "/", nil),
				func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(tc.status) })

			if got := only(t, buf)["level"]; got != tc.want {
				t.Errorf("level for %d = %v, want %s", tc.status, got, tc.want)
			}
		})
	}
}

func TestLoggerTagsRecordsWithRequestID(t *testing.T) {
	logger, buf := capture()
	gen := ids.NewDeterministic(7)
	want := ids.NewDeterministic(7).New().String()

	chain := Chain(RequestID(gen), Logger(logger))
	rec := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	handler := func(_ http.ResponseWriter, r *http.Request) {
		logging.FromContext(r.Context()).Info("from the handler")
	}
	chain(http.HandlerFunc(handler)).ServeHTTP(rec, request)

	got := records(t, buf)
	if len(got) != 2 {
		t.Fatalf("wrote %d records, want 2: %s", len(got), buf)
	}
	for _, record := range got {
		if record["request_id"] != want {
			t.Errorf("%v: request_id = %v, want %s", record["msg"], record["request_id"], want)
		}
	}
}

func TestLoggerLogsRequestThatPanicked(t *testing.T) {
	logger, buf := capture()

	chain := Chain(Logger(logger), Recover())
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	handler := func(http.ResponseWriter, *http.Request) { panic("boom") }
	chain(http.HandlerFunc(handler)).ServeHTTP(httptest.NewRecorder(), request)

	var completion map[string]any
	for _, record := range records(t, buf) {
		if record["msg"] == "request" {
			completion = record
		}
	}
	if completion == nil {
		t.Fatalf("no completion line: %s", buf)
	}
	if got := completion["status"]; got != float64(http.StatusInternalServerError) {
		t.Errorf("status = %v, want %d", got, http.StatusInternalServerError)
	}
}
