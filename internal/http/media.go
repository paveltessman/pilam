package http

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/paveltessman/pilam/internal/platform/logging"
	"github.com/paveltessman/pilam/internal/platform/media"
)

// A blob is named after its own content, so what is served under a key never changes.
const mediaCacheControl = "public, max-age=31536000, immutable"

// serveMedia streams an uploaded file.
func serveMedia(store media.Store) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		key := media.Key(r.PathValue("key"))

		content, info, err := store.Open(ctx, key)
		switch {
		case errors.Is(err, media.ErrInvalidKey), errors.Is(err, media.ErrNotFound):
			http.NotFound(w, r)
			return

		case err != nil:
			logging.FromContext(ctx).Error("opening media failed", "key", key, "err", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		defer func() { _ = content.Close() }()

		w.Header().Set("Content-Type", info.ContentType)
		w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
		w.Header().Set("Cache-Control", mediaCacheControl)

		// The type was settled by reading the bytes at upload. This stops the
		// browser reaching its own conclusion about a file a user supplied.
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// The status is already written by now, so a failure part way through
		// has no way to reach the client. It is logged and nothing else.
		if _, err := io.Copy(w, content); err != nil {
			logging.FromContext(ctx).Error("media transfer cut short", "key", key, "err", err)
		}
	}
	return handler
}
