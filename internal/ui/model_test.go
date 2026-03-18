package ui

import (
	"testing"
	"tic-tac-chec/engine"

	"go.uber.org/goleak"
)

func TestExecuteMoveOnline_ReturnsResponse(t *testing.T) {
	defer goleak.VerifyNone(t)

	moves := make(chan MoveRequest, 1)
	defer close(moves)
	updates := make(chan any)

	model := InitialModel()
	model.Mode = ModeOnline
	model.Moves = moves
	model.Updates = updates

	// fake Room.Run goroutine, in order to process move
	go func() {
		<-moves                    // read move
		updates <- GameStateMsg{} // send some state back
	}()

	piece := engine.WhiteBishop
	cell := engine.Cell{Row: 0, Col: 0}
	_, cmd := model.executeMove(piece, cell)

	res := cmd()
	_, ok := res.(GameStateMsg)
	if !ok {
		t.Errorf("expected GameStateMsg, got %T", res)
	}
}
