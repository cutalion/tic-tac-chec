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
