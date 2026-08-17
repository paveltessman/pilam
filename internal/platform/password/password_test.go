package password_test

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"

	"github.com/paveltessman/pilam/internal/platform/password"
)

const plain = "correct horse battery staple"

func TestVerifyAcceptsSamePassword(t *testing.T) {
	encoded, err := password.Hash(plain)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	ok, needsRehash, err := password.Verify(encoded, plain)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !ok {
		t.Errorf("Verify(%q, plain) = false, want true", encoded)
	}
	if needsRehash {
		t.Error("a hash just written needs a rehash")
	}
}

func TestVerifyRefusesAnotherPassword(t *testing.T) {
	encoded, err := password.Hash(plain)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	testData := map[string]string{
		"a different password": "correct horse battery stapl3",
		"a longer password":    plain + "!",
		"the empty password":   "",
	}
	for name, submitted := range testData {
		t.Run(name, func(t *testing.T) {
			ok, _, err := password.Verify(encoded, submitted)
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if ok {
				t.Errorf("Verify accepted %q", submitted)
			}
		})
	}
}

func TestHashDrawsNewSaltEveryTime(t *testing.T) {
	first, err := password.Hash(plain)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	second, err := password.Hash(plain)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	if first == second {
		t.Errorf("two hashes of one password are equal: %q", first)
	}

	// Both still verify, so the difference is the salt and not a broken write.
	for _, encoded := range []string{first, second} {
		ok, _, err := password.Verify(encoded, plain)
		if err != nil || !ok {
			t.Errorf("Verify(%q) = %v, %v, want true, nil", encoded, ok, err)
		}
	}
}

func TestHashWritesTheCurrentParameters(t *testing.T) {
	encoded, err := password.Hash(plain)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	const want = "$argon2id$v=19$m=65536,t=3,p=4$"
	if !strings.HasPrefix(encoded, want) {
		t.Errorf("Hash wrote %q, want the prefix %q", encoded, want)
	}
}

func TestVerifyAsksForRehashOfOlderParameters(t *testing.T) {
	encoded := custom(t, "m=32768,t=2,p=4", 2, 32*1024, 4, plain)

	ok, needsRehash, err := password.Verify(encoded, plain)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !ok {
		t.Fatal("Verify refused a hash written under older parameters")
	}
	if !needsRehash {
		t.Error("needsRehash = false, want true for older parameters")
	}
}

func TestVerifyRefusesAnUnreadableHash(t *testing.T) {
	good, err := password.Hash(plain)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	testData := map[string]string{
		"empty":                     "",
		"not a hash at all":         "hunter2",
		"truncated at the key":      good[:len(good)-12],
		"truncated at the salt":     strings.Join(strings.Split(good, "$")[:5], "$"),
		"no leading separator":      strings.TrimPrefix(good, "$"),
		"another algorithm":         strings.Replace(good, "$argon2id$", "$argon2i$", 1),
		"another version":           strings.Replace(good, "v=19", "v=16", 1),
		"a cost below the floor":    custom(t, "m=1024,t=1,p=1", 1, 1024, 1, plain),
		"a cost over the ceiling":   custom(t, "m=4194304,t=3,p=4", 3, 64*1024, 4, plain),
		"parameters out of order":   strings.Replace(good, "m=65536,t=3,p=4", "t=3,m=65536,p=4", 1),
		"a parameter that is text":  strings.Replace(good, "t=3", "t=three", 1),
		"a missing parameter":       strings.Replace(good, "m=65536,t=3,p=4", "m=65536,t=3", 1),
		"a salt that is not base64": "$argon2id$v=19$m=65536,t=3,p=4$!!!!!!!!!!!!$" + strings.Split(good, "$")[5],
		"a key that is not base64":  strings.Join(append(strings.Split(good, "$")[:5], "!!!!!!!!!!!!"), "$"),
		"a salt of four bytes":      "$argon2id$v=19$m=65536,t=3,p=4$AAAAAA$" + strings.Split(good, "$")[5],
	}

	for name, encoded := range testData {
		t.Run(name, func(t *testing.T) {
			ok, needsRehash, err := password.Verify(encoded, plain)
			if !errors.Is(err, password.ErrInvalidHash) {
				t.Errorf("Verify(%q) error = %v, want ErrInvalidHash", encoded, err)
			}
			if ok {
				t.Errorf("Verify(%q) = true, want false", encoded)
			}
			if needsRehash {
				t.Error("needsRehash = true on a hash that did not verify")
			}
		})
	}
}

func TestVerifyRefusesAnEditedKey(t *testing.T) {
	good, err := password.Hash(plain)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	parts := strings.Split(good, "$")
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		t.Fatalf("decoding the key: %v", err)
	}
	key[0] ^= 0xff
	parts[5] = base64.RawStdEncoding.EncodeToString(key)
	edited := strings.Join(parts, "$")

	ok, _, err := password.Verify(edited, plain)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if ok {
		t.Error("Verify accepted a hash with an edited key")
	}
}

func TestDummyIsFixedValidHash(t *testing.T) {
	first := password.Dummy()
	if first != password.Dummy() {
		t.Error("Dummy returned two different values")
	}

	ok, needsRehash, err := password.Verify(first, plain)
	if err != nil {
		t.Errorf("Verify(Dummy()) error = %v, want nil", err)
	}
	if ok {
		t.Error("Verify(Dummy(), plain) = true, want false")
	}
	if needsRehash {
		t.Error("Dummy carries parameters other than the current ones")
	}
}

// custom writes a PHC string with the cost line the caller names, and a key
// derived with the cost the caller names. The two are apart on purpose: a test
// edits the cost line alone to make a hash that no longer describes its key.
func custom(t *testing.T, costLine string, time, memory uint32, threads uint8, plain string) string {
	t.Helper()

	salt := []byte("0123456789abcdef")
	key := argon2.IDKey([]byte(plain), salt, time, memory, threads, 32)

	str := "$argon2id$v=19$" + costLine +
		"$" + base64.RawStdEncoding.EncodeToString(salt) +
		"$" + base64.RawStdEncoding.EncodeToString(key)
	return str
}
