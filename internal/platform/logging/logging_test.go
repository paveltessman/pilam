package logging_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/platform/logging"
)

func TestNewJSONRecordCarriesLevelMessageAndAttrs(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.New(logging.Options{Format: logging.FormatJSON, Output: &buf})

	logger.Info("listening", "addr", ":8080")

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("output is not JSON: %v (%q)", err, buf.String())
	}
	for key, want := range map[string]string{
		slog.LevelKey:   "INFO",
		slog.MessageKey: "listening",
		"addr":          ":8080",
	} {
		if got := rec[key]; got != want {
			t.Errorf("%s = %v, want %v", key, got, want)
		}
	}
}

func TestNewTextRecordCarriesLevelMessageAndAttrs(t *testing.T) {
	var buf bytes.Buffer
	logging.New(logging.Options{Format: logging.FormatText, Output: &buf}).
		Info("listening", "addr", ":8080")

	out := buf.String()
	for _, want := range []string{"level=INFO", "msg=listening", "addr=:8080"} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q does not contain %q", out, want)
		}
	}
}

func TestNewDropsRecordsBelowLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.New(logging.Options{Level: slog.LevelInfo, Format: logging.FormatText, Output: &buf})

	logger.Debug("suppressed")

	if buf.Len() != 0 {
		t.Errorf("debug record logged at info level: %q", buf.String())
	}
}

func TestNewHonoursLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := logging.New(logging.Options{Level: slog.LevelDebug, Format: logging.FormatText, Output: &buf})

	logger.Debug("visible")

	if !strings.Contains(buf.String(), "visible") {
		t.Errorf("debug record dropped at debug level: %q", buf.String())
	}
}

func TestNewPanicsOnUnknownFormat(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("New accepted an unknown format")
		}
	}()
	logging.New(logging.Options{Format: "logfmt", Output: &bytes.Buffer{}})
}
