package ui

import (
	"testing"
	"tic-tac-chec/engine"

	tea "github.com/charmbracelet/bubbletea"
	"go.uber.org/goleak"
)

func TestExecuteMoveOnline_ReturnsResponse(t *testing.T) {
	defer goleak.VerifyNone(t)

	moves := make(chan MoveRequest, 1)
	defer close(moves)
	incoming := make(chan tea.Msg)

	model := InitialModel()
	model.Mode = ModeOnline
	model.Moves = moves
	model.Incoming = incoming

	// fake Room.Run goroutine, in order to process move
	go func() {
		<-moves                    // read move
		incoming <- GameStateMsg{} // send some state back
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
