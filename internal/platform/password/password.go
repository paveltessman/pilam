// Package password turns a string into hash, and checks a submitted one against it.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"

	"github.com/paveltessman/pilam/internal/platform/validate"
)

var ErrInvalidHash = errors.New("invalid password hash")

// https://datatracker.ietf.org/doc/html/rfc9106#name-parameter-choice
const (
	defaultTime    = 3
	defaultMemory  = 64 * 1024
	defaultThreads = 4

	saltLen = 16
	keyLen  = 32
)

const (
	minTime    = 2
	maxTime    = 16
	minMemory  = 32 * 1024
	maxMemory  = 1024 * 1024
	minThreads = 1
	maxThreads = 16

	minSaltLen = 8

	minKeyLen = keyLen
	maxKeyLen = 64
)

// params is the cost of one hash.
type params struct {
	time    uint32
	memory  uint32 // KiB
	threads uint8
}

var current = params{time: defaultTime, memory: defaultMemory, threads: defaultThreads}

func Hash(plain string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("drawing a salt: %w", err)
	}
	derived := derive(current, plain, salt, keyLen)
	return encode(current, salt, derived), nil
}

// Verify reports whether the plain string is the password from 'encoded'.
//
// needsRehash reports that the stored hash is valid but was written with
// parameters other than the current ones.
//
// A wrong password is ok false and a nil error. An unreadable hash is an error.
func Verify(encoded, plain string) (ok bool, needsRehash bool, err error) {
	params, salt, key, err := decode(encoded)
	if err != nil {
		return false, false, err
	}

	got := derive(params, plain, salt, uint32(len(key)))
	if subtle.ConstantTimeCompare(got, key) != 1 {
		return false, false, nil
	}
	needsRehash = params != current || len(salt) != saltLen || len(key) != keyLen
	return true, needsRehash, nil
}

// Dummy returns one fixed valid hash, of a password nobody holds.
//
// The login path verifies against it when the submitted email names no user, so
// an unknown email costs the same time as a known one and the answer tells an
// attacker nothing about which addresses exist.
var Dummy = sync.OnceValue(func() string {
	// A fixed salt and a fixed password, so the value stays the same over the
	// life of the process and over restarts.
	const (
		salt  = "pilam.dummy.salt"
		plain = "no user holds this password"
	)
	derived := derive(current, plain, []byte(salt), keyLen)
	return encode(current, []byte(salt), derived)
})

// The length policy, per PRD §9.4. Length in characters, not bytes. There are
// no composition rules.
const (
	MinLen = 12
	MaxLen = 128
)

// Check reports whether plain meets the length policy, as validate.FieldErrors
// against field. A password inside the range returns nil.
func Check(field, plain string) error {
	var v validate.Validator
	v.MinLen(field, plain, MinLen)
	v.MaxLen(field, plain, MaxLen)
	return v.Err()
}

// Generate returns one random password, for a user who does not pick their own:
// the first password the CLI and the users section hand out.
func Generate() string { return rand.Text() }

func derive(p params, plain string, salt []byte, length uint32) []byte {
	return argon2.IDKey([]byte(plain), salt, p.time, p.memory, p.threads, length)
}

var b64 = base64.RawStdEncoding

func encode(p params, salt, key []byte) string {
	str := "$argon2id" +
		"$v=" + strconv.Itoa(argon2.Version) +
		"$m=" + strconv.FormatUint(uint64(p.memory), 10) +
		",t=" + strconv.FormatUint(uint64(p.time), 10) +
		",p=" + strconv.FormatUint(uint64(p.threads), 10) +
		"$" + b64.EncodeToString(salt) +
		"$" + b64.EncodeToString(key)
	return str
}

// decode reads a PHC string. Every failure is ErrInvalidHash.
func decode(encoded string) (p params, salt, key []byte, err error) {
	// A well-formed string starts with the separator, so the first part is empty.
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" {
		return params{}, nil, nil, fmt.Errorf("%w: want five '$'-separated parts", ErrInvalidHash)
	}

	if parts[1] != "argon2id" {
		return params{}, nil, nil, fmt.Errorf("%w: algorithm is %q, want argon2id", ErrInvalidHash, parts[1])
	}

	version, err := field(parts[2], "v")
	if err != nil {
		return params{}, nil, nil, err
	}
	if version != argon2.Version {
		return params{}, nil, nil, fmt.Errorf("%w: version is %d, want %d", ErrInvalidHash, version, argon2.Version)
	}

	p, err = parseParams(parts[3])
	if err != nil {
		return params{}, nil, nil, err
	}

	salt, err = b64.DecodeString(parts[4])
	if err != nil {
		return params{}, nil, nil, fmt.Errorf("%w: salt is not base64", ErrInvalidHash)
	}
	if len(salt) < minSaltLen {
		return params{}, nil, nil, fmt.Errorf("%w: salt is %d bytes, want at least %d", ErrInvalidHash, len(salt), minSaltLen)
	}

	key, err = b64.DecodeString(parts[5])
	if err != nil {
		return params{}, nil, nil, fmt.Errorf("%w: key is not base64", ErrInvalidHash)
	}
	if len(key) < minKeyLen || len(key) > maxKeyLen {
		return params{}, nil, nil, fmt.Errorf("%w: key is %d bytes, want %d to %d", ErrInvalidHash, len(key), minKeyLen, maxKeyLen)
	}

	return p, salt, key, nil
}

// parseParams reads the cost part, "m=65536,t=3,p=4", and holds it to the range
// above.
func parseParams(s string) (params, error) {
	values := strings.Split(s, ",")
	if len(values) != 3 {
		return params{}, fmt.Errorf("%w: parameters are %q, want m, t and p", ErrInvalidHash, s)
	}

	memory, err := field(values[0], "m")
	if err != nil {
		return params{}, err
	}
	time, err := field(values[1], "t")
	if err != nil {
		return params{}, err
	}
	threads, err := field(values[2], "p")
	if err != nil {
		return params{}, err
	}

	if memory < minMemory || memory > maxMemory {
		return params{}, fmt.Errorf("%w: memory is %d KiB, want %d to %d", ErrInvalidHash, memory, minMemory, maxMemory)
	}
	if time < minTime || time > maxTime {
		return params{}, fmt.Errorf("%w: time is %d, want %d to %d", ErrInvalidHash, time, minTime, maxTime)
	}
	if threads < minThreads || threads > maxThreads {
		return params{}, fmt.Errorf("%w: threads is %d, want %d to %d", ErrInvalidHash, threads, minThreads, maxThreads)
	}

	return params{time: uint32(time), memory: uint32(memory), threads: uint8(threads)}, nil
}

// field reads one "key=number" pair.
func field(s, key string) (int, error) {
	digits, found := strings.CutPrefix(s, key+"=")
	if !found {
		return 0, fmt.Errorf("%w: %q does not start with %q", ErrInvalidHash, s, key+"=")
	}

	n, err := strconv.Atoi(digits)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%w: %q is not a number", ErrInvalidHash, s)
	}
	return n, nil
}
