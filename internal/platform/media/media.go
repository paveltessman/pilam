// Package media stores the files users upload.
package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"

	"github.com/paveltessman/pilam/internal/platform/config"
)

var (
	ErrNotFound     = errors.New("media: blob not found")
	ErrTooLarge     = errors.New("media: blob is too large")
	ErrTypeRejected = errors.New("media: content type is not accepted")
	ErrInvalidKey   = errors.New("media: malformed key")
	ErrUnknownKind  = errors.New("media: unknown kind")
)

type Kind string

const (
	Image    Kind = "images"
	Document Kind = "documents"
)

// The shape is <kind>/<2 hex>/<2 hex>/<64 hex>.<ext>.
type Key string

var keyPattern = regexp.MustCompile(`^(images|documents)/[0-9a-f]{2}/[0-9a-f]{2}/[0-9a-f]{64}\.[a-z0-9]{2,4}$`)

// Valid reports whether k is a key this package issued.
func (k Key) Valid() bool { return keyPattern.MatchString(string(k)) }

type Blob struct {
	Kind Kind
	// Name is the filename the client sent.
	Name    string
	Content io.Reader
}

type Stored struct {
	Key         Key
	Name        string
	ContentType string
	Size        int64
}

// Info is what the transport needs to serve a blob.
type Info struct {
	ContentType string
	Size        int64
}

type Store interface {
	// Put reads blob.Content in full, checks it against the policy for
	// blob.Kind, and writes it.
	//
	// The same bytes always produce the same key, and storing them a second
	// time leaves one copy. It fails with ErrTooLarge, ErrTypeRejected or
	// ErrUnknownKind.
	Put(ctx context.Context, blob Blob) (Stored, error)

	// Open returns a reader over the blob stored under key, and what is needed
	// to serve it. The caller closes the reader.
	//
	// It fails with ErrInvalidKey or ErrNotFound.
	Open(ctx context.Context, key Key) (io.ReadCloser, Info, error)

	// URL is the address a browser fetches the blob from.
	URL(key Key) string
}

// New returns the store the configuration asks for.
func New(cfg config.Media) (Store, error) {
	store, err := newLocal(cfg.Dir)
	if err != nil {
		return nil, fmt.Errorf("media: opening the local store: %w", err)
	}
	return store, nil
}
