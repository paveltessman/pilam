package media_test

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"sync"
	"testing"

	"github.com/paveltessman/pilam/internal/platform/media"
)

func TestPutThenOpenReturnsTheSameBytes(t *testing.T) {
	store := newStore(t)

	stored, err := store.Put(t.Context(), media.Blob{Kind: media.Image, Content: strings.NewReader(onePixelPNG)})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if got, want := stored.Size, int64(len(onePixelPNG)); got != want {
		t.Errorf("Size = %d, want %d", got, want)
	}

	content, info, err := store.Open(t.Context(), stored.Key)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = content.Close() }()

	got, err := io.ReadAll(content)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if string(got) != onePixelPNG {
		t.Errorf("read back %q, want %q", got, onePixelPNG)
	}
	if info.Size != stored.Size {
		t.Errorf("Info.Size = %d, want %d", info.Size, stored.Size)
	}
}

func TestPutIsIdempotent(t *testing.T) {
	store, dir := newStoreIn(t)

	first, err := store.Put(t.Context(), media.Blob{Kind: media.Image, Name: "front.png", Content: strings.NewReader(onePixelPNG)})
	if err != nil {
		t.Fatalf("first Put: %v", err)
	}
	second, err := store.Put(t.Context(), media.Blob{Kind: media.Image, Name: "back.png", Content: strings.NewReader(onePixelPNG)})
	if err != nil {
		t.Fatalf("second Put: %v", err)
	}

	if first.Key != second.Key {
		t.Errorf("keys differ: %q then %q", first.Key, second.Key)
	}
	// The key is the content, but the name belongs to the upload, so each call
	// reports the name it was given.
	if first.Name != "front.png" || second.Name != "back.png" {
		t.Errorf("names = %q and %q, want %q and %q", first.Name, second.Name, "front.png", "back.png")
	}
	if files := storedFiles(t, dir); len(files) != 1 {
		t.Errorf("directory holds %v, want one file", files)
	}
}

func TestPutSeparatesDifferentContent(t *testing.T) {
	store, dir := newStoreIn(t)

	png, err := store.Put(t.Context(), media.Blob{Kind: media.Image, Content: strings.NewReader(onePixelPNG)})
	if err != nil {
		t.Fatalf("Put png: %v", err)
	}
	jpeg, err := store.Put(t.Context(), media.Blob{Kind: media.Image, Content: strings.NewReader(smallJPEG)})
	if err != nil {
		t.Fatalf("Put jpeg: %v", err)
	}

	if png.Key == jpeg.Key {
		t.Fatalf("both stored under %q", png.Key)
	}
	if files := storedFiles(t, dir); len(files) != 2 {
		t.Errorf("directory holds %v, want two files", files)
	}
}

// The two short segments in a key shard the digest. Reading them back off the
// key has to land on the same digest the filename carries, or Open looks in the
// wrong directory.
func TestKeyShardsMatchTheDigest(t *testing.T) {
	store := newStore(t)

	stored, err := store.Put(t.Context(), media.Blob{Kind: media.Image, Content: strings.NewReader(onePixelPNG)})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	segments := strings.Split(string(stored.Key), "/")
	if len(segments) != 4 {
		t.Fatalf("key %q has %d segments, want 4", stored.Key, len(segments))
	}
	digest := strings.TrimSuffix(segments[3], path.Ext(segments[3]))
	if got, want := segments[1], digest[:2]; got != want {
		t.Errorf("first shard = %q, want %q", got, want)
	}
	if got, want := segments[2], digest[2:4]; got != want {
		t.Errorf("second shard = %q, want %q", got, want)
	}
}

func TestPutRejectsAnOversizedBlobAndLeavesNothingBehind(t *testing.T) {
	store, dir := newStoreIn(t)

	const photoLimit = 10 << 20
	_, err := store.Put(t.Context(), media.Blob{Kind: media.Image, Content: sized(onePixelPNG, photoLimit+1)})
	if !errors.Is(err, media.ErrTooLarge) {
		t.Fatalf("Put returned %v, want %v", err, media.ErrTooLarge)
	}

	if files := storedFiles(t, dir); len(files) != 0 {
		t.Errorf("directory holds %v, want nothing", files)
	}
}

func TestPutAcceptsABlobExactlyAtLimit(t *testing.T) {
	store := newStore(t)

	const photoLimit = 10 << 20
	stored, err := store.Put(t.Context(), media.Blob{Kind: media.Image, Content: sized(onePixelPNG, photoLimit)})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if stored.Size != photoLimit {
		t.Errorf("Size = %d, want %d", stored.Size, photoLimit)
	}
}

func TestPutLeavesNothingBehindWhenTheTypeIsRejected(t *testing.T) {
	store, dir := newStoreIn(t)

	_, err := store.Put(t.Context(), media.Blob{Kind: media.Image, Content: strings.NewReader(shortPDF)})
	if !errors.Is(err, media.ErrTypeRejected) {
		t.Fatalf("Put returned %v, want %v", err, media.ErrTypeRejected)
	}
	if files := storedFiles(t, dir); len(files) != 0 {
		t.Errorf("directory holds %v, want nothing", files)
	}
}

func TestPutRejectsMissingContent(t *testing.T) {
	store := newStore(t)

	if _, err := store.Put(t.Context(), media.Blob{Kind: media.Image}); err == nil {
		t.Error("Put accepted a blob with no content")
	}
}

func TestOpenReportsMissingBlob(t *testing.T) {
	store := newStore(t)

	const absent = media.Key("images/3f/a9/3fa9c1e2b47d05a6f8130c9e4b72a5d1e6038f4c9a2b71d5e08c364af197b2e0.jpg")
	if _, _, err := store.Open(t.Context(), absent); !errors.Is(err, media.ErrNotFound) {
		t.Errorf("Open returned %v, want %v", err, media.ErrNotFound)
	}
}

// A key never comes from a user, but it does come back from the database and
// through a URL. Anything that is not a key this package issued is refused
// before it reaches the filesystem.
func TestOpenRejectsAKeyThisPackageDidNotIssue(t *testing.T) {
	store := newStore(t)
	testData := map[string]media.Key{
		"a parent segment": "images/../../etc/passwd",
		"an absolute path": "/etc/passwd",
		"an unknown kind":  "avatars/3f/a9/3fa9c1e2b47d05a6f8130c9e4b72a5d1e6038f4c9a2b71d5e08c364af197b2e0.jpg",
		"a short digest":   "images/3f/a9/3fa9c1e2.jpg",
		"empty":            "",
	}
	for name, key := range testData {
		t.Run(name, func(t *testing.T) {
			if _, _, err := store.Open(t.Context(), key); !errors.Is(err, media.ErrInvalidKey) {
				t.Errorf("Open returned %v, want %v", err, media.ErrInvalidKey)
			}
		})
	}
}

// Two uploads of the same image arriving together must still leave one file and
// one key. Run under -race.
func TestPutIsSafeForConcurrentUse(t *testing.T) {
	store, dir := newStoreIn(t)

	const writers = 8
	keys := make([]media.Key, writers)
	errs := make([]error, writers)

	var wg sync.WaitGroup
	for i := range writers {
		wg.Go(func() {
			stored, err := store.Put(t.Context(), media.Blob{Kind: media.Image, Content: strings.NewReader(onePixelPNG)})
			keys[i], errs[i] = stored.Key, err
		})
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("writer %d: %v", i, err)
		}
		if keys[i] != keys[0] {
			t.Errorf("writer %d stored under %q, writer 0 under %q", i, keys[i], keys[0])
		}
	}
	if files := storedFiles(t, dir); len(files) != 1 {
		t.Errorf("directory holds %v, want one file", files)
	}
}

// storedFiles lists every regular file under dir, as slash-separated relative
// paths. Temporary files count, so a test can tell a clean failure from one
// that left a half-written blob behind.
func storedFiles(t *testing.T, dir string) []string {
	t.Helper()

	var found []string
	err := fs.WalkDir(os.DirFS(dir), ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			found = append(found, name)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
	return found
}
