package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

	app := NewApp(clients, bots)

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
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

// initBots loads all ONNX models from the models directory.
// Files named bot_<difficulty>.onnx become available as difficulty levels.
// A plain bot.onnx is loaded as the default ("hard") bot.
func initBots() map[string]*bot.Bot {
	modelsDir := os.Getenv("BOT_MODELS_DIR")
	if modelsDir == "" {
		modelsDir = "bot/models"
	}

	ortLibPath := os.Getenv("ORT_LIB_PATH")
	if ortLibPath == "" {
		log.Println("ORT_LIB_PATH not set, bot disabled")
		return nil
	}

	ort.SetSharedLibraryPath(ortLibPath)
	if err := ort.InitializeEnvironment(); err != nil {
		log.Printf("Failed to init ONNX Runtime: %v — bot disabled", err)
		return nil
	}

	entries, err := os.ReadDir(modelsDir)
	if err != nil {
		log.Printf("Failed to read models dir %s: %v — bot disabled", modelsDir, err)
		return nil
	}

	bots := make(map[string]*bot.Bot)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".onnx") {
			continue
		}

		modelPath := filepath.Join(modelsDir, name)
		b, err := bot.New(modelPath, 100)
		if err != nil {
			log.Printf("Failed to load %s: %v — skipping", modelPath, err)
			continue
		}

		// bot_easy.onnx → "easy", bot_medium.onnx → "medium", bot.onnx → "hard"
		difficulty := "hard"
		if strings.HasPrefix(name, "bot_") {
			difficulty = strings.TrimSuffix(strings.TrimPrefix(name, "bot_"), ".onnx")
		}

		bots[difficulty] = b
		log.Printf("Bot loaded: %s (%s)", difficulty, modelPath)
	}

	if len(bots) == 0 {
		log.Println("No bot models found — bot disabled")
		return nil
	}

	return bots
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
