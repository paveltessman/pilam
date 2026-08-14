package config

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/paveltessman/pilam/internal/platform/logging"
)

// Read the environment one variable at a time, accumulating failures.
type loader struct {
	getenv func(string) string
	errs   []error
}

// Read key and parse it with parse, falling back to default when the
// variable is unset or empty.
//
// Empty counts as unset: `MEDIA_DIR=` in an .env
// file is considered as the author left it blank, not that they wanted an empty string.
//
// An unparseable value is recorded against its key and the zero value returned,
// so loading continues and every bad variable is reported together.
//
// Default is parsed by the same code path.
func value[T any](loader *loader, key, def string, parse func(string) (T, error)) T {
	raw := def
	if envVal := loader.getenv(key); envVal != "" {
		raw = envVal
	}

	parsed, err := parse(raw)
	if err != nil {
		loader.errs = append(loader.errs, fmt.Errorf("%s=%q: %w", key, raw, err))
		var zero T
		return zero
	}
	return parsed
}

// Bounds on the session signing key.
const minSecretLen = 32
const generatedSecretLen = 32

// Read a signing key, generating one when the variable is unset.
// Generating causes a logout on every restart.
func (l *loader) secret(key string) ([]byte, bool) {
	raw := l.getenv(key)
	if raw == "" {
		return generateSecret(), true
	}
	if len(raw) < minSecretLen {
		err := fmt.Errorf("%s: must be at least %d chars, got %d", key, minSecretLen, len(raw))
		l.errs = append(l.errs, err)
		return nil, false
	}
	return []byte(raw), false
}

func generateSecret() []byte {
	key := make([]byte, generatedSecretLen)
	if _, err := rand.Read(key); err != nil {
		// crypto/rand failing is a condition the runtime already treats as
		// fatal, and there is no configuration to return without a key.
		panic(fmt.Errorf("config: reading crypto/rand: %w", err))
	}
	return key
}

// Return every failure as one error, or nil.
func (l *loader) err() error {
	if len(l.errs) == 0 {
		return nil
	}
	return &ErrInvalidValue{errs: l.errs}
}

// ErrInvalidValue reports every rejected variable at once.
//
// It exists instead of errors.Join because Join separates with newlines, and
// this error's destination is a slog call in main, where a newline is escaped
// to a literal \n and such message is hard to read.
//
// Unwrap keeps errors.Is and errors.As working across the whole set anyway.
type ErrInvalidValue struct{ errs []error }

func (e *ErrInvalidValue) Error() string {
	msgs := make([]string, len(e.errs))
	for i, err := range e.errs {
		msgs[i] = err.Error()
	}
	return "invalid configuration: " + strings.Join(msgs, "; ")
}

func (e *ErrInvalidValue) Unwrap() []error { return e.errs }

// Accepts a host:port listen address.
func parseListenAddr(s string) (string, error) {
	if _, _, err := net.SplitHostPort(s); err != nil {
		return "", fmt.Errorf("not a host:port listen address: %w", err)
	}
	return s, nil
}

// Reject zero and negative time durations.
func parsePositiveTimeDuration(s string) (time.Duration, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("not a duration: %w", err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("must be positive, got %s", d)
	}
	return d, nil
}

// Check the shape of a connection URL without connecting.
func parsePostgresURL(s string) (string, error) {
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("not a URL: %w", err)
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return "", fmt.Errorf("scheme is %q, want postgres:// or postgresql://", u.Scheme)
	}
	if u.Host == "" {
		return "", errors.New("host part is missing")
	}
	if u.Path == "" || u.Path == "/" {
		return "", errors.New("database name is missing")
	}
	return s, nil
}

// Resolve a directory path to an absolute one.
//
// Relative paths are allowed and resolved against the working directory,
// which is what lets the default work on a developer's machine; storing
// the absolute form means nothing downstream depends on the process never
// changing directory.
func parseDir(s string) (string, error) {
	abs, err := filepath.Abs(s)
	if err != nil {
		return "", fmt.Errorf("not a usable path: %w", err)
	}
	return abs, nil
}

// Load an IANA zone name.
//
// "Local" is rejected on purpose, so that the business timezone does not depend on
// the machine the app is hosted on.
func parseTimezone(s string) (*time.Location, error) {
	if s == "Local" {
		return nil, errors.New(`must be an explicit IANA zone such as "Europe/Moscow", not "Local"`)
	}
	loc, err := time.LoadLocation(s)
	if err != nil {
		return nil, fmt.Errorf("unknown timezone: %w", err)
	}
	return loc, nil
}

// Read a log level name — debug, info, warn or error, in any case, optionally
// with an offset such as "warn+2".
//
// slog's own parser is the authority on the vocabulary, so a level this accepts
// is a level the handler understands.
func parseLogLevel(s string) (slog.Level, error) {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(s)); err != nil {
		return 0, fmt.Errorf("not a level name: %w", err)
	}
	return lvl, nil
}

// Read a log format name. Unknown names are rejected here so that logging.New,
// which is called with the result, can treat any other value as a bug.
func parseLogFormat(s string) (logging.Format, error) {
	switch f := logging.Format(strings.ToLower(s)); f {
	case logging.FormatText, logging.FormatJSON:
		return f, nil
	default:
		return "", fmt.Errorf("want %q or %q", logging.FormatText, logging.FormatJSON)
	}
}

// Replace the password in a connection URL with "xxxxx", falling back to a placeholder
// if the URL will not parse.
func redactURL(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return "<unparseable>"
	}
	return u.Redacted()
}
