package http

import (
	"context"
	"net/http"

	"github.com/a-h/templ"

	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/http/views"
	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/logging"
)

// render writes a page or a fragment as the response body.
func render(w http.ResponseWriter, r *http.Request, status int, component templ.Component) {
	ctx := r.Context()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	if err := component.Render(ctx, w); err != nil {
		logging.FromContext(ctx).Error("rendering the page failed", "path", r.URL.Path, "err", err)
	}
}

func writeServerError(w http.ResponseWriter) {
	http.Error(w, labels.ErrorUnexpected, http.StatusInternalServerError)
}

// chrome is the header every screen behind the login carries. It reads the
// identity, because the nav offers a root the screens only a root reaches.
func chrome(ctx context.Context) views.Chrome {
	identity, _ := auth.FromContext(ctx)
	return views.Chrome{IsRoot: identity.Role.Allows(auth.RootRole)}
}
