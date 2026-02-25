package main

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"

	"tic-tac-chec/engine"
)

func TestGameStateRoundTrip(t *testing.T) {
	game := engine.NewGame()
	game.Move(engine.WhitePawn, engine.Cell{Row: 0, Col: 0})
	game.Move(engine.BlackBishop, engine.Cell{Row: 3, Col: 1})
	game.Move(engine.WhiteKnight, engine.Cell{Row: 2, Col: 2})

	file, err := os.CreateTemp("", "*.json")
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	defer os.Remove(file.Name())

	err = writeGameState(game, file.Name())
	if err != nil {
		t.Fatal(err)
	}

	restoredGame, err := restoreGame(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	diff := cmp.Diff(restoredGame, game)

	if diff != "" {
		t.Errorf("restored game does not match original: %s", diff)
	}
}
