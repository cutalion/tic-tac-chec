//go:build !dev

package router

import (
	"embed"
	"net/http"
	"tic-tac-chec/internal/web/assets"
	"tic-tac-chec/internal/web/config"
)

//go:embed pages/index.html pages/notfound.html
var pages embed.FS

func registerStaticRoutes(mux *http.ServeMux, cfg config.Config) {
	// Caddy handles all other static files.

	mux.HandleFunc("GET /{$}", indexPage(cfg))
	mux.HandleFunc("GET /rules", indexPage(cfg))
	mux.HandleFunc("GET /lobby", indexPage(cfg))
	mux.HandleFunc("GET /lobby/{id}", indexPage(cfg))
	mux.HandleFunc("GET /room/{id}", indexPage(cfg))
	mux.HandleFunc("GET /", notFoundPage())
}

func indexPage(cfg config.Config) http.HandlerFunc {
	return renderPage("index.html", "text/html", cfg)
}

func notFoundPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		content, err := pages.ReadFile("pages/notfound.html")
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(content)
	}
}

func renderPage(page string, contentType string, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		content, err := pages.ReadFile("pages/" + page)
		if err != nil {
			http.Error(w, "page not found", http.StatusNotFound)
			return
		}
		assets.WriteTemplatedAsset(w, contentType, content, *cfg.Analytics)
	}
}
