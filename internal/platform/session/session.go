// Package session carries a login across requests.

package session

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/paveltessman/pilam/internal/platform/clock"
)

// ErrInvalid is every failure Verify can report.
var ErrInvalid = errors.New("invalid session")

// The separator between the parts of a value. Chosen so because it's not in the
// base64url alphabet, so it cannot occur inside a part.
const separator = "."

// Manager issues and verifies values signed with one key.
//
// Safe for concurrent use: read-only after construction.
type Manager struct {
	secret []byte
	ttl    time.Duration
	clock  clock.Clock
}

// New returns a Manager signing with secret and issuing sessions that last ttl.
func New(secret []byte, ttl time.Duration, c clock.Clock) *Manager {
	if len(secret) == 0 {
		panic(errors.New("session: empty signing key"))
	}
	if ttl <= 0 {
		panic(fmt.Errorf("session: session lifetime is %s, want a positive duration", ttl))
	}
	if c == nil {
		panic(errors.New("session: nil clock"))
	}
	return &Manager{secret: secret, ttl: ttl, clock: c}
}

// TTL is how long an issued session lasts.
func (m *Manager) TTL() time.Duration { return m.ttl }

// Issue returns the signed value for subject
func (m *Manager) Issue(subject string) string {
	expires := m.clock.Now().Add(m.ttl)

	payload := payload(subject, expires)
	return payload + separator + encode(m.sign(payload))
}

// Verify returns the subject a value was issued for.
//
// It fails when the value has been edited, was signed with a different key, has
// expired, or is not a session value at all. Every failure is ErrInvalid.
func (m *Manager) Verify(value string) (string, error) {
	encodedSubject, expiry, signature, found := split(value)
	if !found {
		return "", fmt.Errorf("%w: want three %q-separated parts", ErrInvalid, separator)
	}

	// Authenticate
	payload := encodedSubject + separator + expiry
	mac, err := decode(signature)
	if err != nil {
		return "", fmt.Errorf("%w: signature is not base64url", ErrInvalid)
	}
	if !hmac.Equal(mac, m.sign(payload)) {
		return "", fmt.Errorf("%w: signature does not match", ErrInvalid)
	}

	// Extract the value
	subject, err := decode(encodedSubject)
	if err != nil {
		return "", fmt.Errorf("%w: subject is not base64url", ErrInvalid)
	}
	seconds, err := strconv.ParseInt(expiry, 10, 64)
	if err != nil {
		return "", fmt.Errorf("%w: expiry is not a unix timestamp", ErrInvalid)
	}

	if expires := time.Unix(seconds, 0); !m.clock.Now().Before(expires) {
		return "", fmt.Errorf("%w: expired at %s", ErrInvalid, expires.UTC().Format(time.RFC3339))
	}

	return string(subject), nil
}

// sign returns MAC of the payload
func (m *Manager) sign(payload string) []byte {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(payload))
	return mac.Sum(nil)
}

// payload is the signed half of a value.
func payload(subject string, expires time.Time) string {
	return encode([]byte(subject)) + separator + strconv.FormatInt(expires.Unix(), 10)
}

// split cuts a value into its three parts, reporting whether it had exactly
// three.
func split(value string) (subject, expiry, signature string, found bool) {
	subject, rest, found := strings.Cut(value, separator)
	if !found {
		return "", "", "", false
	}
	expiry, signature, found = strings.Cut(rest, separator)
	if !found || strings.Contains(signature, separator) {
		return "", "", "", false
	}
	return subject, expiry, signature, true
}

func encode(b []byte) string {
	// Values travel in a cookie, so the encoding is the URL-safe alphabet, and
	// unpadded because "=" is one of the characters a cookie value may not carry.
	return base64.RawURLEncoding.EncodeToString(b)
}

func decode(s string) ([]byte, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("decoding: %w", err)
	}
	return b, nil
}
