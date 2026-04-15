package main

import (
	"log"
	"net/http"
	"os"
	"tic-tac-chec/internal/bot"

	ort "github.com/yalue/onnxruntime_go"
)

var clients = NewClientService()
var analyticsConfig = resolveAnalyticsConfig()

type AnalyticsConfig struct {
	Enabled     bool
	PostHogKey  string
	PostHogHost string
}

func main() {
	bots := initBots()
	allowedOrigins := parseAllowedOrigins()

	app := NewApp(clients, bots, allowedOrigins)

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

// initBots loads a single ONNX model and creates three difficulty levels
// using different MCTS simulation counts: easy (0), medium (250), hard (500).
func initBots() map[string]*bot.Bot {
	ortLibPath := os.Getenv("ORT_LIB_PATH")
	if ortLibPath == "" {
		log.Println("ORT_LIB_PATH not set, bot disabled")
		return nil
	}

	ort.SetSharedLibraryPath(ortLibPath)
	if err := ort.InitializeEnvironment(); err != nil {
		log.Printf("Failed to initialize ONNX Runtime: %v - bot disabled", err)
		return nil
	}

	modelPath := os.Getenv("BOT_MODEL_PATH")
	if modelPath == "" {
		modelPath = "bot/models/bot.onnx"
	}

	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		log.Printf("Bot model not found: %s - bot disabled", modelPath)
		return nil
	}

	easy, err := bot.New(modelPath, "easy")
	if err != nil {
		log.Printf("Failed to create easy bot: %v - bot disabled", err)
		return nil
	}
	medium, err := bot.New(modelPath, "medium")
	if err != nil {
		log.Printf("Failed to create medium bot: %v - bot disabled", err)
		return nil
	}
	hard, err := bot.New(modelPath, "hard")
	if err != nil {
		log.Printf("Failed to create hard bot: %v - bot disabled", err)
		return nil
	}

	bots := map[string]*bot.Bot{
		"easy":   easy,
		"medium": medium,
		"hard":   hard,
	}
	return bots
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
