//go:build dev

package router

import (
	"net/http"
	"os"
	"tic-tac-chec/internal/web/assets"
	"tic-tac-chec/internal/web/config"
)

func registerStaticRoutes(mux *http.ServeMux, cfg config.Config) {
	mux.Handle("GET /{$}", indexHandler(cfg))
	mux.Handle("GET /rules", indexHandler(cfg))
	mux.Handle("GET /lobby", indexHandler(cfg))
	mux.Handle("GET /lobby/{id}", indexHandler(cfg))
	mux.Handle("GET /room/{id}", indexHandler(cfg))

	mux.Handle("GET /app.js", staticHandler())
	mux.Handle("GET /style.css", staticHandler())
	mux.Handle("GET /manifest.json", staticHandler())
	mux.Handle("GET /sw.js", serviceWorkerHandler(cfg))
	mux.Handle("GET /icon-192-v2.png", staticHandler())
	mux.Handle("GET /icon-512-v2.png", staticHandler())
	mux.Handle("GET /icon-512-maskable-v2.png", staticHandler())
	mux.Handle("GET /apple-touch-icon-v2.png", staticHandler())
	mux.Handle("GET /favicon.ico", staticHandler())
	mux.Handle("GET /llms.txt", staticHandler())
	mux.Handle("GET /ttc_logo.png", staticHandler())
	mux.Handle("GET /fonts/", staticHandler())
	mux.Handle("GET /sounds/", staticHandler())
	mux.Handle("GET /docs/", http.StripPrefix("/docs/", http.FileServer(http.Dir("internal/web/app/docs"))))

	mux.Handle("GET /", notFoundHandler())
}

func staticHandler() http.Handler {
	fs := http.FileServer(http.Dir("internal/web/static"))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Dev builds: never cache static assets, so edits are always picked up.
		w.Header().Set("Cache-Control", "no-store")
		fs.ServeHTTP(w, r)
	})
}

func indexHandler(cfg config.Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := os.ReadFile("internal/web/router/pages/index.html")
		if err != nil {
			http.Error(w, "index not found", http.StatusInternalServerError)
			return
		}

		assets.WriteTemplatedAsset(w, "text/html; charset=utf-8", raw, *cfg.Analytics)
	})
}

func notFoundHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := os.ReadFile("internal/web/router/pages/notfound.html")
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(raw)
	})
}

func serviceWorkerHandler(cfg config.Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := os.ReadFile("internal/web/static/sw.js")
		if err != nil {
			http.Error(w, "service worker not found", http.StatusInternalServerError)
			return
		}

		assets.WriteTemplatedAsset(w, "application/javascript; charset=utf-8", raw, *cfg.Analytics)
	})
}
