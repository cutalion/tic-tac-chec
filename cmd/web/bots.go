package main

import (
	"context"
	"log"
	"os"

	"tic-tac-chec/cmd/web/store"
	"tic-tac-chec/internal/bot"

	ort "github.com/yalue/onnxruntime_go"
)

type Bot struct {
	Model *bot.Model
	Info  store.Bot
}

// initBots loads a single ONNX model and creates three difficulty levels
// using different MCTS simulation counts: easy (0), medium (250), hard (500).
func initBots(ctx context.Context, db *store.Store) map[string]*Bot {
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

	version := 1
	botRecords, err := db.Bots().LoadBots(ctx, version)
	if err != nil {
		log.Printf("Failed to load bots: %v - bot disabled", err)
		return nil
	}

	bots := make(map[string]*Bot)
	for _, br := range botRecords {
		b, err := bot.New(br.ModelPath, br.Mcts_Sims)
		if err != nil {
			log.Printf("Failed to create bot %s: %v - bot disabled", br.Difficulty, err)
			return nil
		}

		bots[br.Difficulty] = &Bot{Model: b, Info: br}
	}

	return bots
}
