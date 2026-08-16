package http

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/platform/media"
)

const pngFixture = "\x89PNG\r\n\x1a\n" + "and the rest of a png"

func TestMediaIsServedWithItsOwnTypeAndAnImmutableCache(t *testing.T) {
	deps := deps(t)
	stored, err := deps.Media.Put(t.Context(), media.Blob{Kind: media.Image, Content: strings.NewReader(pngFixture)})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	rec := getWith(t, deps, deps.Media.URL(stored.Key))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != pngFixture {
		t.Errorf("body = %q, want %q", got, pngFixture)
	}

	for header, want := range map[string]string{
		"Content-Type":           "image/png",
		"Content-Length":         strconv.Itoa(len(pngFixture)),
		"Cache-Control":          mediaCacheControl,
		"X-Content-Type-Options": "nosniff",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}

func TestMediaIs404ForAnythingThatDoesNotNameAFile(t *testing.T) {
	testData := map[string]string{
		"a well-formed key with no file": "/media/photos/3f/a9/3fa9c1e2b47d05a6f8130c9e4b72a5d1e6038f4c9a2b71d5e08c364af197b2e0.jpg",
		"a malformed key":                "/media/photos/3f/a9/short.jpg",
		"an unknown kind":                "/media/avatars/3f/a9/3fa9c1e2b47d05a6f8130c9e4b72a5d1e6038f4c9a2b71d5e08c364af197b2e0.jpg",
		"no key at all":                  "/media/",
	}
	for name, target := range testData {
		t.Run(name, func(t *testing.T) {
			if rec := get(t, target); rec.Code != http.StatusNotFound {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
			}
		})
	}
}
