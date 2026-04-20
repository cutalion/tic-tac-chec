//go:build dev

package main

import (
	"net/http"
	"os"
)

func registerStaticRoutes(mux *http.ServeMux) {
	mux.Handle("GET /", indexHandler())
	mux.Handle("GET /lobby", indexHandler())
	mux.Handle("GET /lobby/{id}", indexHandler())
	mux.Handle("GET /room/{id}", indexHandler())

	mux.Handle("GET /app.js", staticHandler())
	mux.Handle("GET /style.css", staticHandler())
	mux.Handle("GET /manifest.json", staticHandler())
	mux.Handle("GET /sw.js", serviceWorkerHandler())
	mux.Handle("GET /icon.svg", staticHandler())
	mux.Handle("GET /icon-192.png", staticHandler())
	mux.Handle("GET /icon-512.png", staticHandler())
	mux.Handle("GET /apple-touch-icon.png", staticHandler())
	mux.Handle("GET /favicon.ico", staticHandler())
	mux.Handle("GET /llms.txt", staticHandler())
	mux.Handle("GET /ttc_logo.png", staticHandler())
	mux.Handle("GET /fonts/", staticHandler())
	mux.Handle("GET /docs/", http.StripPrefix("/docs/", http.FileServer(http.Dir("cmd/web/docs"))))
}

func staticHandler() http.Handler {
	fs := http.FileServer(http.Dir("cmd/web/static"))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Dev builds: never cache static assets, so edits are always picked up.
		w.Header().Set("Cache-Control", "no-store")
		fs.ServeHTTP(w, r)
	})
}

func indexHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := os.ReadFile("cmd/web/pages/index.html")
		if err != nil {
			http.Error(w, "index not found", http.StatusInternalServerError)
			return
		}

		writeTemplatedAsset(w, "text/html; charset=utf-8", raw)
	})
}

func serviceWorkerHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := os.ReadFile("cmd/web/static/sw.js")
		if err != nil {
			http.Error(w, "service worker not found", http.StatusInternalServerError)
			return
		}

		writeTemplatedAsset(w, "application/javascript; charset=utf-8", raw)
	})
}
