package main

import (
	"log"
	"net/http"
	"os"
)

var clients = NewClientService()
var analyticsConfig = resolveAnalyticsConfig()

type AnalyticsConfig struct {
	Enabled     bool
	PostHogKey  string
	PostHogHost string
}

func main() {
	app := NewApp(clients)

	mux := http.NewServeMux()
	registerStaticRoutes(mux)

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
