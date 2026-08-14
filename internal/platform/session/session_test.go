package session_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/paveltessman/pilam/internal/platform/clock"
	"github.com/paveltessman/pilam/internal/platform/session"
)

const (
	ttl     = 24 * time.Hour
	subject = "01937b2e-0000-7000-8000-000000000001"
)

var secret = []byte("a signing key of a perfectly usable length")

func newManager(t *testing.T) (*session.Manager, *clock.FixedClock) {
	t.Helper()

	c := clock.Fixed(time.Date(2026, time.August, 14, 10, 0, 0, 0, time.UTC), time.UTC)
	return session.New(secret, ttl, c), c
}

func TestRoundTrip(t *testing.T) {
	m, _ := newManager(t)

	got, err := m.Verify(m.Issue(subject))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got != subject {
		t.Errorf("subject = %q, want %q", got, subject)
	}
}

// Whatever auth decides to put in the subject later, it survives — including
// the separator the value is cut on.
func TestSubjectSurvivesWhateverIsInIt(t *testing.T) {
	testData := map[string]string{
		"empty":             "",
		"a separator":       "one.two.three",
		"russian":           "менеджер",
		"spaces and equals": "role=manager id=1",
		"long":              strings.Repeat("x", 1000),
	}
	for name, in := range testData {
		t.Run(name, func(t *testing.T) {
			m, _ := newManager(t)

			got, err := m.Verify(m.Issue(in))
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if got != in {
				t.Errorf("subject = %q, want %q", got, in)
			}
		})
	}
}

func TestExpiry(t *testing.T) {
	m, c := newManager(t)
	value := m.Issue(subject)

	c.Advance(ttl - time.Second)
	if _, err := m.Verify(value); err != nil {
		t.Errorf("a session one second short of its TTL was rejected: %v", err)
	}

	// The moment it expires, not the moment after.
	c.Advance(time.Second)
	assertInvalid(t, m, value, "at the expiry instant")

	c.Advance(30 * 24 * time.Hour)
	assertInvalid(t, m, value, "a month past the expiry")
}

func TestExtendingTheExpiryIsRejected(t *testing.T) {
	m, c := newManager(t)

	encodedSubject, _, signature := parts(t, m.Issue(subject))
	forever := strconv.FormatInt(c.Now().Add(100*365*24*time.Hour).Unix(), 10)

	assertInvalid(t, m, encodedSubject+"."+forever+"."+signature, "an extended expiry")
}

func TestTamperingIsRejected(t *testing.T) {
	m, _ := newManager(t)
	issued := m.Issue(subject)
	encodedSubject, expiry, signature := parts(t, issued)

	testData := map[string]string{
		"a different subject":            encode("01937b2e-0000-7000-8000-000000000002") + "." + expiry + "." + signature,
		"an edited signature":            encodedSubject + "." + expiry + "." + flip(signature),
		"a dropped signature":            encodedSubject + "." + expiry + ".",
		"a truncated value":              issued[:len(issued)-4],
		"an appended part":               issued + ".extra",
		"a signature that is not base64": encodedSubject + "." + expiry + ".not base64!",
		"nothing at all":                 "",
		"one part":                       encodedSubject,
		"two parts":                      encodedSubject + "." + expiry,
	}

	for name, value := range testData {
		t.Run(name, func(t *testing.T) {
			assertInvalid(t, m, value, name)
		})
	}
}

func TestAnotherKeyCannotVerify(t *testing.T) {
	m, c := newManager(t)
	other := session.New([]byte("a different key of a perfectly usable length"), ttl, c)

	assertInvalid(t, other, m.Issue(subject), "a value signed with another key")
}

func TestWellSignedNonsenseIsRejected(t *testing.T) {
	m, c := newManager(t)
	future := strconv.FormatInt(c.Now().Add(ttl).Unix(), 10)

	testData := map[string]string{
		"subject is not base64url": sign("not base64!", future),
		"expiry is not a number":   sign(encode(subject), "next tuesday"),
	}

	for name, value := range testData {
		t.Run(name, func(t *testing.T) {
			assertInvalid(t, m, value, name)
		})
	}
}

// The wire format, pinned: changing it logs every open session out, so it
// should take a deliberate edit to this test to do it.
func TestWireFormat(t *testing.T) {
	m, c := newManager(t)

	value := m.Issue(subject)
	encodedSubject, expiry, signature := parts(t, value)

	if want := encode(subject); encodedSubject != want {
		t.Errorf("subject part = %q, want %q", encodedSubject, want)
	}
	if want := strconv.FormatInt(c.Now().Add(ttl).Unix(), 10); expiry != want {
		t.Errorf("expiry part = %q, want %q", expiry, want)
	}
	if want := sign(encodedSubject, expiry); value != want {
		t.Errorf("value = %q, want %q", value, want)
	}
	// A cookie value may not carry base64's padding character.
	if strings.Contains(signature, "=") {
		t.Errorf("signature %q is padded", signature)
	}
}

func TestNewRejectsUnusableWiring(t *testing.T) {
	c := clock.Fixed(time.Now(), time.UTC)

	testData := map[string]func(){
		"empty secret": func() { session.New(nil, ttl, c) },
		"zero ttl":     func() { session.New(secret, 0, c) },
		"negative ttl": func() { session.New(secret, -time.Hour, c) },
		"nil clock":    func() { session.New(secret, ttl, nil) },
	}

	for name, build := range testData {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("New accepted it")
				}
			}()
			build()
		})
	}
}

func assertInvalid(t *testing.T, m *session.Manager, value, what string) {
	t.Helper()

	got, err := m.Verify(value)
	if err == nil {
		t.Fatalf("Verify accepted %s, returning subject %q", what, got)
	}
	if !errors.Is(err, session.ErrInvalid) {
		t.Errorf("error for %s is %v, want it to wrap ErrInvalid", what, err)
	}
	if got != "" {
		t.Errorf("Verify returned subject %q beside the error", got)
	}
}

func parts(t *testing.T, value string) (subject, expiry, signature string) {
	t.Helper()

	split := strings.Split(value, ".")
	if len(split) != 3 {
		t.Fatalf("value %q has %d parts, want 3", value, len(split))
	}
	return split[0], split[1], split[2]
}

// sign builds a value the way Issue does, so a test can present parts Issue
// would never produce over a signature that checks out.
func sign(subjectPart, expiryPart string) string {
	payload := subjectPart + "." + expiryPart

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func encode(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

// flip changes one character of s, leaving it the same length and still
// base64url.
func flip(s string) string {
	last := len(s) - 1
	if s[last] == 'A' {
		return s[:last] + "B"
	}
	return s[:last] + "A"
}
