package ids_test

import (
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/paveltessman/pilam/internal/platform/ids"
)

func TestNewGeneratorReturnsDistinctIdentifiers(t *testing.T) {
	const n = 10_000

	g := ids.NewGenerator()
	seen := make(map[ids.ID]struct{}, n)

	for range n {
		id := g.New()
		if id == ids.Nil {
			t.Fatal("generator returned the nil identifier")
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("generator repeated %s", id)
		}
		seen[id] = struct{}{}
	}
}

// The point of v7 over v4 is that the text form sorts by creation order. Assert
// it, because a later swap of the generation strategy that quietly gave that up
// would show as index bloat months later rather than as a failing test.
func TestNewGeneratorIdentifiersSortByCreationOrder(t *testing.T) {
	g := ids.NewGenerator()

	got := make([]string, 100)
	for i := range got {
		got[i] = g.New().String()
	}

	if !sort.StringsAreSorted(got) {
		t.Errorf("identifiers do not sort by creation order: %v", got)
	}
}

func TestNewGeneratorProducesVersion7Identifiers(t *testing.T) {
	id := ids.NewGenerator().New()

	if id.Version() != 7 {
		t.Errorf("version = %s, want 7", id.Version())
	}
	if id.Variant() != uuid.RFC4122 {
		t.Errorf("variant = %s, want %s", id.Variant(), uuid.RFC4122)
	}
}

func TestParseRoundTripsTheCanonicalForm(t *testing.T) {
	want := ids.NewGenerator().New()

	got, err := ids.Parse(want.String())
	if err != nil {
		t.Fatalf("Parse(%q): %v", want, err)
	}
	if got != want {
		t.Errorf("Parse(%q) = %s, want %s", want, got, want)
	}
}

// A pasted identifier that differs only in hex case is the same resource.
func TestParseAcceptsUppercaseHex(t *testing.T) {
	want := ids.NewGenerator().New()

	got, err := ids.Parse(strings.ToUpper(want.String()))
	if err != nil {
		t.Fatalf("Parse of the uppercase form: %v", err)
	}
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestParseRejects(t *testing.T) {
	canonical := ids.NewGenerator().New().String()

	for name, in := range map[string]string{
		"empty":                 "",
		"not hex":               "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz",
		"truncated":             canonical[:35],
		"trailing space":        canonical + " ",
		"unhyphenated":          strings.ReplaceAll(canonical, "-", ""),
		"braced":                "{" + canonical + "}",
		"urn":                   "urn:uuid:" + canonical,
		"hyphens misplaced":     "0198b3f4d3-c7-73f2-8e1a-5f6b7c8d9e0f",
		"nil identifier":        ids.Nil.String(),
		"sql injection-ish":     "1' OR '1'='1",
		"an article, not an ID": "SS27-001",
	} {
		t.Run(name, func(t *testing.T) {
			got, err := ids.Parse(in)
			if err == nil {
				t.Fatalf("Parse(%q) = %s, want an error", in, got)
			}
			if !errors.Is(err, ids.ErrInvalid) {
				t.Errorf("Parse(%q) error is %v, want it to wrap ErrInvalid", in, err)
			}
			if got != ids.Nil {
				t.Errorf("Parse(%q) returned %s beside the error, want Nil", in, got)
			}
		})
	}
}

func TestMustParseReturnsTheIdentifier(t *testing.T) {
	want := ids.NewGenerator().New()

	if got := ids.MustParse(want.String()); got != want {
		t.Errorf("MustParse(%q) = %s, want %s", want, got, want)
	}
}

func TestMustParsePanicsOnJunk(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("MustParse accepted a value Parse rejects")
		}
	}()
	ids.MustParse("not-an-identifier")
}
