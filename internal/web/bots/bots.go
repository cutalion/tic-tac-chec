package bots

import (
	"context"
	"log"

	"tic-tac-chec/internal/bot"
	"tic-tac-chec/internal/web/config"
	store "tic-tac-chec/internal/web/persistence/sqlite"

	ort "github.com/yalue/onnxruntime_go"
)

type Bot struct {
	Model *bot.Model
	Info  store.Bot
}

type Bots map[string]*Bot

// UnavailableReason is set when Init returns nil (empty bots map), for HTTP errors.
var UnavailableReason string

// initBots loads a single ONNX model and creates three difficulty levels
// using different MCTS simulation counts: easy (0), medium (250), hard (500).
func Init(ctx context.Context, db *store.Store, cfg config.Bots) Bots {
	UnavailableReason = ""

	if cfg.OrtLibPath == "" {
		UnavailableReason = "ORT_LIB_PATH is not set (path to the ONNX Runtime shared library)"
		log.Println("ORT_LIB_PATH not set, bot disabled")
		return nil
	}

	ort.SetSharedLibraryPath(cfg.OrtLibPath)
	if err := ort.InitializeEnvironment(); err != nil {
		UnavailableReason = "ONNX Runtime init failed: " + err.Error()
		log.Printf("Failed to initialize ONNX Runtime: %v - bot disabled", err)
		return nil
	}

	version := 1
	botRecords, err := db.Bots().LoadBots(ctx, version)
	if err != nil {
		UnavailableReason = "failed to load bot metadata from database: " + err.Error()
		log.Printf("Failed to load bots: %v - bot disabled", err)
		return nil
	}

	bots := make(Bots)
	for _, br := range botRecords {
		b, err := bot.New(br.ModelPath, br.Mcts_Sims)
		if err != nil {
			UnavailableReason = "failed to load bot model " + br.Difficulty + ": " + err.Error()
			log.Printf("Failed to create bot %s: %v - bot disabled", br.Difficulty, err)
			return nil
		}

		bots[br.Difficulty] = &Bot{Model: b, Info: br}
	}

	if len(bots) == 0 {
		UnavailableReason = "no bot rows in the database for version 1 (run migrations and ensure bots/players are seeded)"
		log.Println("no bot rows in database for version 1, bot disabled")
		return nil
	}

	log.Printf("bots ready: %d difficulty level(s) loaded", len(bots))
	return bots
}
