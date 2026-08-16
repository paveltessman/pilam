package media

import (
	"fmt"
	"mime"
	"net/http"
	"path"
	"strings"
)

const (
	imageMaxSize    = 10 << 20 // 10 MiB
	documentMaxSize = 25 << 20 // 25 MiB
)

const sniffLen = 512

type Policy struct {
	MaxSize int64

	// Types maps each accepted media type to the extension it is stored under.
	//
	// The extension comes from here rather than from the uploaded filename, so
	// a key always describes its own content.
	Types map[string]string
}

// Accepts returns the extension for a media type, and whether the type is
// accepted at all.
func (p Policy) Accepts(contentType string) (string, bool) {
	ext, ok := p.Types[contentType]
	return ext, ok
}

var policies = map[Kind]Policy{
	Image: {
		MaxSize: imageMaxSize,
		Types: map[string]string{
			"image/jpeg": ".jpg",
			"image/png":  ".png",
			"image/webp": ".webp",
			"image/gif":  ".gif",
		},
	},

	Document: {
		MaxSize: documentMaxSize,
		Types: map[string]string{
			"application/pdf": ".pdf",
			"text/plain":      ".txt",
		},
	},
}

func policyFor(kind Kind) (Policy, error) {
	p, ok := policies[kind]
	if !ok {
		return Policy{}, fmt.Errorf("%w: %q", ErrUnknownKind, kind)
	}
	return p, nil
}

// typeByExt inverts every policy table, so Open can name the content type of a
// stored blob from its key alone.
var typeByExt = buildTypeByExt()

func buildTypeByExt() map[string]string {
	byExt := make(map[string]string)
	for _, p := range policies {
		for contentType, ext := range p.Types {
			if other, taken := byExt[ext]; taken && other != contentType {
				// Two policies disagree about what an extension means.
				panic(fmt.Errorf("media: extension %q maps to both %q and %q", ext, other, contentType))
			}
			byExt[ext] = contentType
		}
	}
	return byExt
}

// contentTypeOf names what is stored under a key, from its extension.
//
// Every key this package issues carries an extension, so the
// fallback only covers a key that came from somewhere else.
func contentTypeOf(key Key) string {
	if contentType, known := typeByExt[path.Ext(string(key))]; known {
		return contentType
	}
	return "application/octet-stream"
}

// detect returns the media type of head, without parameters.
//
// CSV arrives as text/plain.
// Docx and xlsx arrive as application/zip.
func detect(head []byte) string {
	sniffed := http.DetectContentType(head)

	bare, _, err := mime.ParseMediaType(sniffed)
	if err != nil {
		return sniffed
	}
	return strings.ToLower(bare)
}
