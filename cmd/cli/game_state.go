package main

import (
	"encoding/json"
	"os"
	"tic-tac-chec/engine"
	"tic-tac-chec/internal/wire"
)

func createGameStateFile() (string, error) {
	f, err := os.CreateTemp("", "tic-tac-chec-game-state-*.json")
	if err != nil {
		return "", err
	}
	f.Close()

	return f.Name(), nil
}

func restoreGame(path string) (*engine.Game, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	state := &wire.GameState{}
	if err = json.Unmarshal(data, state); err != nil {
		return nil, err
	}

	game := wire.GameFromState(state)
	return game, nil
}

func writeGameState(game *engine.Game, path string) error {
	state := wire.ToGameState(game)

	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
