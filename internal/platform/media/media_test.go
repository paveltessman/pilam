package media_test

import (
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/platform/config"
	"github.com/paveltessman/pilam/internal/platform/media"
)

// A key is the only thing a caller hands back to the store, so what Valid
// accepts is the whole of the package's input checking. Path traversal never
// reaches the filesystem because these shapes are rejected here.
func TestKeyValid(t *testing.T) {
	const digest = "3fa9c1e2b47d05a6f8130c9e4b72a5d1e6038f4c9a2b71d5e08c364af197b2e0"
	testData := map[string]struct {
		key  media.Key
		want bool
	}{
		"a photo key":            {media.Key("images/3f/a9/" + digest + ".jpg"), true},
		"a document key":         {media.Key("documents/3f/a9/" + digest + ".pdf"), true},
		"a four letter suffix":   {media.Key("images/3f/a9/" + digest + ".webp"), true},
		"empty":                  {"", false},
		"an unknown kind":        {media.Key("avatars/3f/a9/" + digest + ".jpg"), false},
		"a parent segment":       {media.Key("images/../../etc/passwd"), false},
		"an absolute path":       {media.Key("/images/3f/a9/" + digest + ".jpg"), false},
		"a trailing slash":       {media.Key("images/3f/a9/" + digest + ".jpg/"), false},
		"a short digest":         {media.Key("images/3f/a9/3fa9c1e2.jpg"), false},
		"a long digest":          {media.Key("images/3f/a9/" + digest + "00.jpg"), false},
		"an uppercase digest":    {media.Key("images/3F/A9/" + strings.ToUpper(digest) + ".jpg"), false},
		"a mismatched shard":     {media.Key("images/zz/a9/" + digest + ".jpg"), false},
		"no extension":           {media.Key("images/3f/a9/" + digest), false},
		"a windows separator":    {media.Key(`images\3f\a9\` + digest + ".jpg"), false},
		"an embedded null byte":  {media.Key("images/3f/a9/" + digest + ".jpg\x00"), false},
		"a single shard segment": {media.Key("images/3f/" + digest + ".jpg"), false},
	}

	for name, tcase := range testData {
		t.Run(name, func(t *testing.T) {
			if got := tcase.key.Valid(); got != tcase.want {
				t.Errorf("Key(%q).Valid() = %v, want %v", tcase.key, got, tcase.want)
			}
		})
	}
}

func TestPutIssuesKeysThatValidAccepts(t *testing.T) {
	store := newStore(t)
	testData := map[string]media.Blob{
		"a photo":    {Kind: media.Image, Name: "front.png", Content: strings.NewReader(onePixelPNG)},
		"a document": {Kind: media.Document, Name: "spec.pdf", Content: strings.NewReader(shortPDF)},
	}
	for name, blob := range testData {
		t.Run(name, func(t *testing.T) {
			stored, err := store.Put(t.Context(), blob)
			if err != nil {
				t.Fatalf("Put: %v", err)
			}
			if !stored.Key.Valid() {
				t.Errorf("Put issued %q, which Valid rejects", stored.Key)
			}
		})
	}
}

func TestNewRejectsAnEmptyDirectory(t *testing.T) {
	if _, err := media.New(config.Media{}); err == nil {
		t.Error("New accepted an unconfigured directory")
	}
}

func TestURLIsTheServingPath(t *testing.T) {
	store := newStore(t)

	stored, err := store.Put(t.Context(), media.Blob{Kind: media.Image, Content: strings.NewReader(onePixelPNG)})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	if got, want := store.URL(stored.Key), "/media/"+string(stored.Key); got != want {
		t.Errorf("URL = %q, want %q", got, want)
	}
}
