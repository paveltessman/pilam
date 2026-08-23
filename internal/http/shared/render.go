package shared

import (
	"context"
	"net/http"

	"github.com/a-h/templ"

	"github.com/paveltessman/pilam/internal/auth"
	"github.com/paveltessman/pilam/internal/http/shared/views"
	"github.com/paveltessman/pilam/internal/platform/labels"
	"github.com/paveltessman/pilam/internal/platform/logging"
)

// Render writes a page or a fragment as the response body.
func Render(w http.ResponseWriter, r *http.Request, status int, component templ.Component) {
	ctx := r.Context()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	if err := component.Render(ctx, w); err != nil {
		logging.FromContext(ctx).Error("rendering the page failed", "path", r.URL.Path, "err", err)
	}
}

// savedQuery marks a screen the browser reaches after a write the server
// accepted. The screen reads the mark and reports that the changes are saved.
const savedQuery = "saved"

// RedirectSaved sends the browser to path with the saved mark on it.
func RedirectSaved(w http.ResponseWriter, r *http.Request, path string) {
	http.Redirect(w, r, path+"?"+savedQuery+"=1", http.StatusSeeOther)
}

// IsSaved reports whether the request carries the mark redirectSaved sets.
func IsSaved(r *http.Request) bool {
	return r.URL.Query().Get(savedQuery) != ""
}

// RedirectTo is the handler of a path that carries no screen of its own.
func RedirectTo(path string) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, path, http.StatusSeeOther)
	}
	return handler
}

func WriteServerError(w http.ResponseWriter) {
	http.Error(w, labels.ErrorUnexpected, http.StatusInternalServerError)
}

// Chrome is the header every screen behind the login carries. It reads the
// identity, because the nav offers a root the screens only a root reaches.
func Chrome(ctx context.Context) views.Chrome {
	identity, _ := auth.FromContext(ctx)
	return views.Chrome{IsRoot: identity.Role.Allows(auth.RootRole)}
}
