//go:build !dev

package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*
var staticFiles embed.FS

func staticHandler() http.Handler {
	sub, _ := fs.Sub(staticFiles, "static")
	return http.FileServer(http.FS(sub))
}

func indexHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := staticFiles.ReadFile("static/index.html")
		if err != nil {
			http.Error(w, "index not found", http.StatusInternalServerError)
			return
		}

		writeTemplatedAsset(w, "text/html; charset=utf-8", raw)
	})
}

func serviceWorkerHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := staticFiles.ReadFile("static/sw.js")
		if err != nil {
			http.Error(w, "service worker not found", http.StatusInternalServerError)
			return
		}

		writeTemplatedAsset(w, "application/javascript; charset=utf-8", raw)
	})
}
