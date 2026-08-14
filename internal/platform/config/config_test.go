package config

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/platform/logging"
)

// Turns a map into the lookup load expects. Absent keys read as empty.
func env(m map[string]string) func(string) string {
	return func(key string) string { return m[key] }
}

func mustLoad(t *testing.T, m map[string]string) Config {
	t.Helper()
	cfg, err := load(env(m))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return cfg
}

// An empty environment must produce a usable configuration, and — because
// defaults go through the same parsers as the environment — this is also what
// proves no default has been written that the package would reject.
func TestDefaultsAreValidAndComplete(t *testing.T) {
	cfg := mustLoad(t, nil)

	if cfg.HTTP.Addr != defaultHTTPAddr {
		t.Errorf("HTTP.Addr = %q, want %q", cfg.HTTP.Addr, defaultHTTPAddr)
	}

	want, err := time.ParseDuration(defaultShutdownGrace)
	if err != nil {
		t.Errorf("could not parse default shutdown grace time. value: %s", defaultShutdownGrace)
	}
	if cfg.HTTP.ShutdownGrace != want {
		t.Errorf("HTTP.ShutdownGrace = %s, want %s", cfg.HTTP.ShutdownGrace, want)
	}
	if cfg.Database.URL != defaultDatabaseURL {
		t.Errorf("Database.URL = %q, want %q", cfg.Database.URL, defaultDatabaseURL)
	}
	if !strings.HasSuffix(cfg.Media.Dir, "tmp/media") {
		t.Errorf("Media.Dir = %q, want it to end in tmp/media", cfg.Media.Dir)
	}
	if cfg.Timezone == nil || cfg.Timezone.String() != defaultTimezone {
		t.Errorf("Timezone = %v, want %s", cfg.Timezone, defaultTimezone)
	}
	if cfg.Log.Level != slog.LevelInfo {
		t.Errorf("Log.Level = %s, want %s", cfg.Log.Level, slog.LevelInfo)
	}
	if want := logging.Format(defaultLogFormat); cfg.Log.Format != want {
		t.Errorf("Log.Format = %q, want %q", cfg.Log.Format, want)
	}

	ttl, err := time.ParseDuration(defaultSessionTTL)
	if err != nil {
		t.Errorf("could not parse the default session TTL. value: %s", defaultSessionTTL)
	}
	if cfg.Session.TTL != ttl {
		t.Errorf("Session.TTL = %s, want %s", cfg.Session.TTL, ttl)
	}
}

// SESSION_SECRET is generated if unset
func TestUnsetSecretIsGeneratedAndSaysSo(t *testing.T) {
	cfg := mustLoad(t, nil)

	if !cfg.Session.Generated {
		t.Error("Session.Generated is false for an unset SESSION_SECRET")
	}
	if got := len(cfg.Session.Secret); got != generatedSecretLen {
		t.Errorf("generated key is %d bytes, want %d", got, generatedSecretLen)
	}

	// Two boots on the same environment must not agree, or "generated" would be
	// a fixed key with extra steps.
	other := mustLoad(t, nil)
	if bytes.Equal(cfg.Session.Secret, other.Session.Secret) {
		t.Error("two loads generated the same key")
	}
}

func TestRejectsAShortSecret(t *testing.T) {
	const short = "hunter2"

	_, err := load(env(map[string]string{"SESSION_SECRET": short}))
	if err == nil {
		t.Fatalf("SESSION_SECRET=%q was accepted", short)
	}
	if !strings.Contains(err.Error(), "SESSION_SECRET") {
		t.Errorf("error does not name the variable: %v", err)
	}
	if strings.Contains(err.Error(), short) {
		t.Errorf("the rejected key appears in the error: %v", err)
	}
}

func TestEnvironmentOverridesEveryField(t *testing.T) {
	const secret = "a-configured-signing-key-of-usable-length"

	cfg := mustLoad(t, map[string]string{
		"HTTP_ADDR":           "127.0.0.1:9000",
		"HTTP_SHUTDOWN_GRACE": "45s",
		"DATABASE_URL":        "postgresql://u:p@db:5432/pilam",
		"MEDIA_DIR":           "/var/lib/pilam/media",
		"SESSION_SECRET":      secret,
		"SESSION_TTL":         "24h",
		"BUSINESS_TZ":         "Asia/Tokyo",
		"LOG_LEVEL":           "debug",
		"LOG_FORMAT":          "json",
	})

	if string(cfg.Session.Secret) != secret {
		t.Errorf("Session.Secret = %q, want the configured key", cfg.Session.Secret)
	}
	if cfg.Session.Generated {
		t.Error("Session.Generated is true for a configured key")
	}
	if want := 24 * time.Hour; cfg.Session.TTL != want {
		t.Errorf("Session.TTL = %s, want %s", cfg.Session.TTL, want)
	}

	if want := "127.0.0.1:9000"; cfg.HTTP.Addr != want {
		t.Errorf("HTTP.Addr = %q, want %q", cfg.HTTP.Addr, want)
	}
	if want := 45 * time.Second; cfg.HTTP.ShutdownGrace != want {
		t.Errorf("HTTP.ShutdownGrace = %s, want %s", cfg.HTTP.ShutdownGrace, want)
	}
	if want := "postgresql://u:p@db:5432/pilam"; cfg.Database.URL != want {
		t.Errorf("Database.URL = %q, want %q", cfg.Database.URL, want)
	}
	if want := "/var/lib/pilam/media"; cfg.Media.Dir != want {
		t.Errorf("Media.Dir = %q, want %q", cfg.Media.Dir, want)
	}
	if want := "Asia/Tokyo"; cfg.Timezone.String() != want {
		t.Errorf("Timezone = %v, want %s", cfg.Timezone, want)
	}
	if want := slog.LevelDebug; cfg.Log.Level != want {
		t.Errorf("Log.Level = %s, want %s", cfg.Log.Level, want)
	}
	if want := logging.FormatJSON; cfg.Log.Format != want {
		t.Errorf("Log.Format = %q, want %q", cfg.Log.Format, want)
	}
}

// Level names come off an .env line or a compose file, where nobody types case
// carefully; the offset form is slog's, and it comes along for free.
func TestLogLevelAcceptsCaseAndOffsets(t *testing.T) {
	for in, want := range map[string]slog.Level{
		"DEBUG":  slog.LevelDebug,
		"Warn":   slog.LevelWarn,
		"error":  slog.LevelError,
		"warn+2": slog.LevelWarn + 2,
	} {
		t.Run(in, func(t *testing.T) {
			cfg := mustLoad(t, map[string]string{"LOG_LEVEL": in})
			if cfg.Log.Level != want {
				t.Errorf("Log.Level = %s, want %s", cfg.Log.Level, want)
			}
		})
	}

	if cfg := mustLoad(t, map[string]string{"LOG_FORMAT": "JSON"}); cfg.Log.Format != logging.FormatJSON {
		t.Errorf("Log.Format = %q, want %q", cfg.Log.Format, logging.FormatJSON)
	}
}

// A variable present but blank is an unfilled line in an .env file, not a
// request for the empty string.
func TestBlankValueFallsBackToDefault(t *testing.T) {
	cfg := mustLoad(t, map[string]string{"HTTP_ADDR": "", "BUSINESS_TZ": ""})

	if cfg.HTTP.Addr != defaultHTTPAddr {
		t.Errorf("HTTP.Addr = %q, want the default %q", cfg.HTTP.Addr, defaultHTTPAddr)
	}
	if cfg.Timezone.String() != defaultTimezone {
		t.Errorf("Timezone = %v, want the default %s", cfg.Timezone, defaultTimezone)
	}
}

func TestRejectsBadValues(t *testing.T) {
	cases := []struct {
		name string
		key  string
		val  string
		want string // fragment the message must carry, beyond the key
	}{
		{"addr without port", "HTTP_ADDR", "8080", "host:port"},
		{"grace not a duration", "HTTP_SHUTDOWN_GRACE", "10 seconds", "not a duration"},
		{"grace zero", "HTTP_SHUTDOWN_GRACE", "0s", "must be positive"},
		{"grace negative", "HTTP_SHUTDOWN_GRACE", "-1s", "must be positive"},
		{"url wrong scheme", "DATABASE_URL", "mysql://u:p@db:3306/pilam", "want postgres://"},
		{"url no host", "DATABASE_URL", "postgres:///pilam", "host part is missing"},
		{"url no database", "DATABASE_URL", "postgres://u:p@db:5432", "database name is missing"},
		{"url not a url", "DATABASE_URL", "postgres://%zz", "not a URL"},
		{"session ttl zero", "SESSION_TTL", "0s", "must be positive"},
		{"session ttl not a duration", "SESSION_TTL", "a week", "not a duration"},
		{"unknown timezone", "BUSINESS_TZ", "Europe/Atlantis", "unknown timezone"},
		{"timezone Local", "BUSINESS_TZ", "Local", "explicit IANA zone"},
		{"unknown log level", "LOG_LEVEL", "chatty", "not a level name"},
		{"log level bad offset", "LOG_LEVEL", "warn+x", "not a level name"},
		{"unknown log format", "LOG_FORMAT", "logfmt", `want "text" or "json"`},
	}

	for _, tcase := range cases {
		t.Run(tcase.name, func(t *testing.T) {
			_, err := load(env(map[string]string{tcase.key: tcase.val}))
			if err == nil {
				t.Fatalf("%s=%q was accepted", tcase.key, tcase.val)
			}
			// The key has to be in the message
			if !strings.Contains(err.Error(), tcase.key) {
				t.Errorf("error does not name %s: %v", tcase.key, err)
			}
			if !strings.Contains(err.Error(), tcase.want) {
				t.Errorf("error does not explain %q: %v", tcase.want, err)
			}
		})
	}
}

// Fixing configuration one restart at a time is the failure mode this avoids.
func TestReportsEveryProblemAtOnce(t *testing.T) {
	_, err := load(env(map[string]string{
		"HTTP_ADDR":    "8080",
		"DATABASE_URL": "mysql://u:p@db:3306/pilam",
		"BUSINESS_TZ":  "Europe/Atlantis",
	}))
	if err == nil {
		t.Fatal("three bad values were accepted")
	}
	for _, key := range []string{"HTTP_ADDR", "DATABASE_URL", "BUSINESS_TZ"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error does not mention %s:\n%v", key, err)
		}
	}
}

// The business timezone exists to make "today" independent of where the
// process runs.
func TestBusinessTimezoneIsOffsetFromUTC(t *testing.T) {
	cfg := mustLoad(t, nil)

	// 22:30 UTC on the 13th is already the 14th in Moscow.
	utc := time.Date(2026, 8, 13, 22, 30, 0, 0, time.UTC)
	if got, want := utc.In(cfg.Timezone).Day(), 14; got != want {
		t.Errorf("day in %v = %d, want %d", cfg.Timezone, got, want)
	}
}

func TestLogValueRedactsThePassword(t *testing.T) {
	cfg := mustLoad(t, map[string]string{
		"DATABASE_URL":   "postgres://pilam:hunter2@db:5432/pilam?sslmode=disable",
		"SESSION_SECRET": "correct-horse-battery-staple-and-then-some",
	})

	// Logged the way main logs it, so this covers the wiring and not just the
	// method: slog only calls LogValue when a handler resolves the value.
	var buf strings.Builder
	slog.New(slog.NewTextHandler(&buf, nil)).Info("configuration", "config", cfg)

	logged := buf.String()
	if strings.Contains(logged, "hunter2") {
		t.Errorf("password survives logging: %s", logged)
	}
	if strings.Contains(logged, "correct-horse") {
		t.Errorf("session signing key survives logging: %s", logged)
	}
	// Everything else about the URL is diagnostic and must stay.
	for _, want := range []string{"db:5432", "pilam", "sslmode=disable"} {
		if !strings.Contains(logged, want) {
			t.Errorf("logged config lost %q: %s", want, logged)
		}
	}
}

// Proves that Load is wired to the real environment.
func TestLoadReadsTheProcessEnvironment(t *testing.T) {
	t.Setenv("HTTP_ADDR", "127.0.0.1:9999")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := "127.0.0.1:9999"; cfg.HTTP.Addr != want {
		t.Errorf("HTTP.Addr = %q, want %q", cfg.HTTP.Addr, want)
	}
}
