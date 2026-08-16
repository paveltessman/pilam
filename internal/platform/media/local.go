package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	tempSubdir             = ".tmp"
	dirMode    fs.FileMode = 0o755
	blobMode   fs.FileMode = 0o644
)

const shardLen = 2

var _ Store = (*local)(nil)

// local writes blobs to a directory tree.
type local struct {
	dir string
}

// newLocal prepares dir as a storage root and returns a store over it.
func newLocal(dir string) (*local, error) {
	if dir == "" {
		return nil, errors.New("directory can't be an empty string")
	}
	if err := os.MkdirAll(filepath.Join(dir, tempSubdir), dirMode); err != nil {
		return nil, fmt.Errorf("preparing %s: %w", dir, err)
	}
	return &local{dir: dir}, nil
}

func (l *local) Put(ctx context.Context, blob Blob) (Stored, error) {
	policy, err := policyFor(blob.Kind)
	if err != nil {
		return Stored{}, err
	}
	if blob.Content == nil {
		return Stored{}, errors.New("media: no content to store")
	}
	if err := ctx.Err(); err != nil {
		return Stored{}, err
	}

	// The type is settled before a single byte is written, so a rejected upload
	// costs one read rather than a whole file.
	head, err := readHead(blob.Content)
	if err != nil {
		return Stored{}, err
	}
	contentType := detect(head)
	ext, accepted := policy.Accepts(contentType)
	if !accepted {
		return Stored{}, fmt.Errorf("%w: %s is not stored as a %s", ErrTypeRejected, contentType, blob.Kind)
	}

	temp, err := os.CreateTemp(filepath.Join(l.dir, tempSubdir), "put-*")
	if err != nil {
		return Stored{}, fmt.Errorf("media: opening a temporary file: %w", err)
	}
	// Removed on every path that does not rename it away.
	published := false
	defer func() {
		_ = temp.Close()
		if !published {
			_ = os.Remove(temp.Name())
		}
	}()

	digest := sha256.New()
	content := io.MultiReader(bytes.NewReader(head), blob.Content)

	size, err := io.Copy(io.MultiWriter(temp, digest), io.LimitReader(content, policy.MaxSize+1))
	if err != nil {
		return Stored{}, fmt.Errorf("media: writing content: %w", err)
	}
	if size > policy.MaxSize {
		return Stored{}, fmt.Errorf("%w: over the %d byte limit for a %s", ErrTooLarge, policy.MaxSize, blob.Kind)
	}
	if err := ctx.Err(); err != nil {
		return Stored{}, err
	}

	key := keyOf(blob.Kind, digest.Sum(nil), ext)
	stored := Stored{Key: key, Name: blob.Name, ContentType: contentType, Size: size}

	target := l.path(key)
	if _, err := os.Stat(target); err == nil {
		// The key is the content, so a file already sitting there holds these exact
		// bytes. There is nothing to write and nothing to overwrite.
		return stored, nil
	}

	if err := temp.Chmod(blobMode); err != nil {
		return Stored{}, fmt.Errorf("media: setting permissions: %w", err)
	}
	// Flushed before the rename, so a crash cannot leave a named but empty blob.
	if err := temp.Sync(); err != nil {
		return Stored{}, fmt.Errorf("media: flushing content: %w", err)
	}
	if err := temp.Close(); err != nil {
		return Stored{}, fmt.Errorf("media: closing the temporary file: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(target), dirMode); err != nil {
		return Stored{}, fmt.Errorf("media: preparing the blob directory: %w", err)
	}
	if err := os.Rename(temp.Name(), target); err != nil {
		return Stored{}, fmt.Errorf("media: publishing the blob: %w", err)
	}
	published = true

	return stored, nil
}

func (l *local) Open(_ context.Context, key Key) (io.ReadCloser, Info, error) {
	if !key.Valid() {
		return nil, Info{}, fmt.Errorf("%w: %q", ErrInvalidKey, key)
	}

	f, err := os.Open(l.path(key))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, Info{}, fmt.Errorf("%w: %s", ErrNotFound, key)
	}
	if err != nil {
		return nil, Info{}, fmt.Errorf("media: opening %s: %w", key, err)
	}

	stat, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, Info{}, fmt.Errorf("media: sizing %s: %w", key, err)
	}

	return f, Info{ContentType: contentTypeOf(key), Size: stat.Size()}, nil
}

func (l *local) URL(key Key) string { return "/media/" + string(key) }

// path is where a key lives on disk.
//
// Only ever called with a key that Valid accepts or that Put just built, so the
// join cannot escape the storage root.
func (l *local) path(key Key) string {
	if strings.Contains(string(key), "..") {
		panic("path is trying to escape the storage root")
	}
	return filepath.Join(l.dir, filepath.FromSlash(string(key)))
}

// keyOf names a blob after its content.
func keyOf(kind Kind, digest []byte, ext string) Key {
	hexDigest := hex.EncodeToString(digest)
	return Key(path.Join(
		string(kind),
		hexDigest[:shardLen],
		hexDigest[shardLen:2*shardLen],
		hexDigest+ext,
	))
}

// readHead returns the first bytes of r, as many as the sniffer looks at. A
// shorter reader gives back what it has.
func readHead(r io.Reader) ([]byte, error) {
	head := make([]byte, sniffLen)
	n, err := io.ReadFull(r, head)
	switch {
	case err == nil, errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return head[:n], nil
	default:
		return nil, fmt.Errorf("media: reading content: %w", err)
	}
}
