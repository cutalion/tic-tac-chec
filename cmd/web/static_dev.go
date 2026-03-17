//go:build dev

package main

import "net/http"

func staticHandler() http.Handler {
	return http.FileServer(http.Dir("cmd/web/static"))
}
