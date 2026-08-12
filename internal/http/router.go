// Package http is the transport layer: router, middleware, handlers and views.
package http

import (
	"net/http"

	"github.com/a-h/templ"

	"github.com/paveltessman/pilam/internal/http/static"
	"github.com/paveltessman/pilam/internal/http/views"
)

func reportHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func NewRouter() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /static/", http.StripPrefix("/static/", static.Handler()))
	mux.HandleFunc("GET /health", reportHealth)
	mux.Handle("GET /{$}", templ.Handler(views.Home()))

	return mux
}
