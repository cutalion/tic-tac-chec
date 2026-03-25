package ui

import (
	"testing"
	"tic-tac-chec/engine"
	"tic-tac-chec/internal/game"

	"go.uber.org/goleak"
)

func TestExecuteMoveOnline_ReturnsResponse(t *testing.T) {
	defer goleak.VerifyNone(t)

	commands := make(chan game.Command, 1)
	defer close(commands)
	updates := make(chan game.Event)

	model := InitialModel()
	model.Mode = ModeOnline
	model.Commands = commands
	model.Updates = updates

	// fake Room.Run goroutine, in order to process move
	go func() {
		<-commands                                             // read command (moves/rematch)
		updates <- game.SnapshotEvent{Game: *engine.NewGame()} // send some state back
	}()

	piece := engine.WhiteBishop
	cell := engine.Cell{Row: 0, Col: 0}
	_, cmd := model.executeMove(piece, cell)

	res := cmd()
	_, ok := res.(game.SnapshotEvent)
	if !ok {
		t.Errorf("expected game.SnapshotEvent, got %T", res)
	}
}
