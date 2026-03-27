package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

var clients = NewClientService()
var assetVersion = resolveAssetVersion()
var analyticsConfig = resolveAnalyticsConfig()

type AnalyticsConfig struct {
	Enabled     bool
	PostHogKey  string
	PostHogHost string
}

func main() {
	app := NewApp(clients)

	mux := http.NewServeMux()
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

	// api
	mux.HandleFunc("POST /api/clients", app.CreateClient)
	mux.HandleFunc("POST /api/lobbies", app.CreateLobby)
	mux.HandleFunc("GET /api/me", app.Me)

	// ws
	mux.HandleFunc("GET /ws/lobby", app.DefaultLobby)
	mux.HandleFunc("GET /ws/lobby/{id}", app.Lobby)
	mux.HandleFunc("GET /ws/room/{id}", app.Room)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("Listening on :" + port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func resolveAssetVersion() string {
	if version := os.Getenv("ASSET_VERSION"); version != "" {
		return version
	}

	return strconv.FormatInt(time.Now().UTC().Unix(), 10)
}

func resolveAnalyticsConfig() AnalyticsConfig {
	enabled := os.Getenv("ANALYTICS_ENABLED") == "true"
	key := os.Getenv("POSTHOG_KEY")
	host := os.Getenv("POSTHOG_HOST")

	if !enabled || key == "" || host == "" {
		return AnalyticsConfig{}
	}

	return AnalyticsConfig{
		Enabled:     enabled,
		PostHogKey:  key,
		PostHogHost: host,
	}
}
