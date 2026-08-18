package auth

import (
	"errors"
	"testing"

	"github.com/paveltessman/pilam/internal/platform/ids"
)

const testID = "01912345-6789-7abc-def0-123456789abc"

func TestPrincipalStringFollowsFormat(t *testing.T) {
	p := Principal{UserID: ids.MustParse(testID), Epoch: 1}

	const want = testID + ":1"

	if got := p.String(); got != want {
		t.Errorf("Incorrect principal text: want=%q, got=%q", want, got)
	}
}

func TestParsePrincipalReadsWhatStringWrites(t *testing.T) {
	want := Principal{UserID: ids.MustParse(testID), Epoch: 42}

	got, err := ParsePrincipal(want.String())
	if err != nil {
		t.Fatalf("ParsePrincipal(%q) failed: %v", want.String(), err)
	}
	if got != want {
		t.Errorf("Round trip lost the principal: want=%+v, got=%+v", want, got)
	}
}

func TestParsePrincipalRefusesBadSubjects(t *testing.T) {
	cases := map[string]string{
		"empty":         "",
		"no separator":  testID,
		"no epoch":      testID + ":",
		"no id":         ":1",
		"epoch is text": testID + ":one",
		"three parts":   testID + ":1:2",
		"unhyphenated":  "0191234567897abcdef0123456789abc:1",
		"just the id":   "not-a-uuid:1",
	}

	for name, subject := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ParsePrincipal(subject)
			if !errors.Is(err, ErrUnknownSubject) {
				t.Fatalf("ParsePrincipal(%q) = %+v, %v, want ErrUnknownSubject", subject, got, err)
			}
		})
	}
}
