//go:build dev

package main

import "net/http"

func staticHandler() http.Handler {
	return http.FileServer(http.Dir("cmd/web/static"))
}

func indexHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "cmd/web/static/index.html")
	})
}
