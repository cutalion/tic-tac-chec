package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"tic-tac-chec/cmd/web/store"
)

var analyticsConfig = resolveAnalyticsConfig()

type AnalyticsConfig struct {
	Enabled     bool
	PostHogKey  string
	PostHogHost string
}

func main() {
	allowedOrigins := parseAllowedOrigins()
	db, err := store.NewStore(dbPath())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	app := NewApp(db, allowedOrigins)

	mux := http.NewServeMux()
	registerStaticRoutes(mux)

	// api
	mux.HandleFunc("POST /api/clients", app.CreateClient)
	mux.HandleFunc("POST /api/lobbies", app.CreateLobby)
	mux.HandleFunc("POST /api/bot-game", app.BotGame)
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
	log.Fatal(http.ListenAndServe(":"+port, corsMiddleware(mux, app.allowedOrigins)))
}

func corsMiddleware(next http.Handler, allowedOrigins []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(allowedOrigins) == 0 {
			next.ServeHTTP(w, r)
			return
		}

		for _, origin := range allowedOrigins {
			if r.Header.Get("Origin") == origin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				break
			}
		}

		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func parseAllowedOrigins() []string {
	raw := os.Getenv("ALLOWED_ORIGINS")
	if raw == "" {
		return []string{}
	}
	origins := strings.Split(raw, ",")
	for i, o := range origins {
		origins[i] = strings.TrimSpace(o)
	}
	return origins
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

func dbPath() string {
	env := os.Getenv("DB_PATH")
	if env == "" {
		env = "tic-tac-chec.db"
	}
	return env
}
