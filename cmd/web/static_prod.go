//go:build !dev

package main

import (
	"embed"
	"net/http"
)

//go:embed pages/index.html
var pages embed.FS

func registerStaticRoutes(mux *http.ServeMux) {
	// Caddy handles all other static files.

	mux.HandleFunc("GET /", indexPage())
	mux.HandleFunc("GET /lobby", indexPage())
	mux.HandleFunc("GET /lobby/{id}", indexPage())
	mux.HandleFunc("GET /room/{id}", indexPage())
}

func indexPage() http.HandlerFunc {
	return renderPage("index.html", "text/html")
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
