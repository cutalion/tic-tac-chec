//go:build !dev

package main

import (
	"embed"
	"net/http"
)

//go:embed pages/index.html pages/notfound.html
var pages embed.FS

func registerStaticRoutes(mux *http.ServeMux) {
	// Caddy handles all other static files.

	mux.HandleFunc("GET /{$}", indexPage())
	mux.HandleFunc("GET /rules", indexPage())
	mux.HandleFunc("GET /lobby", indexPage())
	mux.HandleFunc("GET /lobby/{id}", indexPage())
	mux.HandleFunc("GET /room/{id}", indexPage())
	mux.HandleFunc("GET /", notFoundPage())
}

func indexPage() http.HandlerFunc {
	return renderPage("index.html", "text/html")
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

func renderPage(page string, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		content, err := pages.ReadFile("pages/" + page)
		if err != nil {
			http.Error(w, "page not found", http.StatusNotFound)
			return
		}
		writeTemplatedAsset(w, contentType, content)
	}
}
