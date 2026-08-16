package media_test

import (
	"io"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/platform/config"
	"github.com/paveltessman/pilam/internal/platform/media"
)

// The package sniffs a file rather than decoding it, so a fixture only has to
// carry the leading bytes the sniffer reads. Writing them out this way keeps visible
// what each test depends on, and keeps binary blobs out of the tree.
const (
	onePixelPNG = "\x89PNG\r\n\x1a\n" + "and the rest of a png"
	smallJPEG   = "\xff\xd8\xff\xe0" + "and the rest of a jpeg"
	smallGIF    = "GIF89a" + "and the rest of a gif"
	smallWebP   = "RIFF\x1c\x00\x00\x00WEBPVP8 " + "and the rest of a webp"
	shortPDF    = "%PDF-1.7\nand the rest of a pdf"
	plainText   = "a line of text with nothing that looks like a file header\n"
)

func newStore(t *testing.T) media.Store {
	t.Helper()
	store, _ := newStoreIn(t)
	return store
}

func newStoreIn(t *testing.T) (media.Store, string) {
	t.Helper()
	dir := t.TempDir()
	store, err := media.New(config.Media{Dir: dir})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return store, dir
}

// sized returns a reader of exactly n bytes beginning with header.
func sized(header string, n int64) io.Reader {
	return io.MultiReader(strings.NewReader(header), io.LimitReader(filler{}, n-int64(len(header))))
}

// filler is an endless reader of zero bytes.
type filler struct{}

func (filler) Read(p []byte) (int, error) { return len(p), nil }
