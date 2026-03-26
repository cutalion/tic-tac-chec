//go:build dev

package main

import (
	"net/http"
	"os"
)

func staticHandler() http.Handler {
	return http.FileServer(http.Dir("cmd/web/static"))
}

func indexHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := os.ReadFile("cmd/web/static/index.html")
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
