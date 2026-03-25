//go:build !dev

package main

import (
	"embed"
	"io"
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
		file, err := staticFiles.Open("static/index.html")
		if err != nil {
			http.Error(w, "index not found", http.StatusInternalServerError)
			return
		}
		defer file.Close()

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.Copy(w, file)
	})
}
