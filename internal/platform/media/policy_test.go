package media_test

import (
	"errors"
	"path"
	"strings"
	"testing"

	"github.com/paveltessman/pilam/internal/platform/media"
)

func TestPutStoresEachAcceptedTypeUnderItsOwnExtension(t *testing.T) {
	store := newStore(t)
	testData := map[string]struct {
		kind            media.Kind
		content         string
		wantContentType string
		wantExt         string
	}{
		"png":        {media.Image, onePixelPNG, "image/png", ".png"},
		"jpeg":       {media.Image, smallJPEG, "image/jpeg", ".jpg"},
		"gif":        {media.Image, smallGIF, "image/gif", ".gif"},
		"webp":       {media.Image, smallWebP, "image/webp", ".webp"},
		"pdf":        {media.Document, shortPDF, "application/pdf", ".pdf"},
		"plain text": {media.Document, plainText, "text/plain", ".txt"},
	}
	for name, tcase := range testData {
		t.Run(name, func(t *testing.T) {
			stored, err := store.Put(t.Context(), media.Blob{
				Kind:    tcase.kind,
				Name:    "whatever.exe",
				Content: strings.NewReader(tcase.content),
			})
			if err != nil {
				t.Fatalf("Put: %v", err)
			}
			if stored.ContentType != tcase.wantContentType {
				t.Errorf("ContentType = %q, want %q", stored.ContentType, tcase.wantContentType)
			}
			if got := path.Ext(string(stored.Key)); got != tcase.wantExt {
				t.Errorf("extension = %q, want %q", got, tcase.wantExt)
			}
			if got, want := path.Dir(string(stored.Key)), string(tcase.kind); !strings.HasPrefix(got, want) {
				t.Errorf("key is under %q, want it under %q", got, want)
			}
		})
	}
}

func TestPutAcceptsTextDespiteItsCharsetParameter(t *testing.T) {
	store := newStore(t)

	stored, err := store.Put(t.Context(), media.Blob{Kind: media.Document, Content: strings.NewReader(plainText)})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if got, want := stored.ContentType, "text/plain"; got != want {
		t.Errorf("ContentType = %q, want %q with no parameters", got, want)
	}
}

// A policy belongs to one Kind. A PDF is a perfectly good file and still has no
// business being stored as a colorway photo.
func TestPutRejectsATypeTheKindDoesNotAccept(t *testing.T) {
	store := newStore(t)
	testData := map[string]struct {
		kind    media.Kind
		content string
	}{
		"a pdf as a photo":    {media.Image, shortPDF},
		"text as a photo":     {media.Image, plainText},
		"a png as a document": {media.Document, onePixelPNG},
	}
	for name, tcase := range testData {
		t.Run(name, func(t *testing.T) {
			_, err := store.Put(t.Context(), media.Blob{Kind: tcase.kind, Content: strings.NewReader(tcase.content)})
			if !errors.Is(err, media.ErrTypeRejected) {
				t.Errorf("Put returned %v, want %v", err, media.ErrTypeRejected)
			}
		})
	}
}

func TestPutRejectsKindWithNoPolicy(t *testing.T) {
	store := newStore(t)

	_, err := store.Put(t.Context(), media.Blob{Kind: "avatars", Content: strings.NewReader(onePixelPNG)})
	if !errors.Is(err, media.ErrUnknownKind) {
		t.Errorf("Put returned %v, want %v", err, media.ErrUnknownKind)
	}
}

func TestOpenReportsTheTypePutRecorded(t *testing.T) {
	store := newStore(t)
	testData := map[string]struct {
		kind    media.Kind
		content string
	}{
		"png":  {media.Image, onePixelPNG},
		"webp": {media.Image, smallWebP},
		"pdf":  {media.Document, shortPDF},
		"text": {media.Document, plainText},
	}
	for name, tcase := range testData {
		t.Run(name, func(t *testing.T) {
			stored, err := store.Put(t.Context(), media.Blob{Kind: tcase.kind, Content: strings.NewReader(tcase.content)})
			if err != nil {
				t.Fatalf("Put: %v", err)
			}

			content, info, err := store.Open(t.Context(), stored.Key)
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			defer func() { _ = content.Close() }()

			if info.ContentType != stored.ContentType {
				t.Errorf("Open reports %q, Put reported %q", info.ContentType, stored.ContentType)
			}
			if info.Size != stored.Size {
				t.Errorf("Open reports %d bytes, Put reported %d", info.Size, stored.Size)
			}
		})
	}
}
