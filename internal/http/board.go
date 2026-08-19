package http

import (
	"net/http"

	"github.com/paveltessman/pilam/internal/http/views"
)

// showBoard renders the post-login destination.
func showBoard() http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		render(w, r, http.StatusOK, views.Board(chrome(r.Context())))
	}
	return handler
}
