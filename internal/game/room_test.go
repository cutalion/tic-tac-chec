package game

import (
	"testing"

	"tic-tac-chec/engine"
	"tic-tac-chec/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"go.uber.org/goleak"
)

func setupRoom() (Room, [2]chan ui.MoveRequest, [2]chan tea.Msg) {
	// create bi-directional channels, so we can manually read/write in tests
	// readonly would be impractical
	moves := [2]chan ui.MoveRequest{
		make(chan ui.MoveRequest),
		make(chan ui.MoveRequest),
	}

	incomings := [2]chan tea.Msg{
		make(chan tea.Msg, 1), // buffered, so we don't wait in tests
		make(chan tea.Msg, 1),
	}

	room := Room{
		Game: engine.NewGame(),
		Players: [2]Player{
			{Color: engine.White, Moves: moves[0], Incoming: incomings[0]},
			{Color: engine.Black, Moves: moves[1], Incoming: incomings[1]},
		},
	}

	return room, moves, incomings
}

func TestRoom(t *testing.T) {
	defer goleak.VerifyNone(t)

	room, moves, incomings := setupRoom()
	defer close(moves[0])
	defer close(moves[1])

	go room.Run()

	moves[0] <- ui.MoveRequest{Piece: engine.WhiteBishop, Cell: engine.Cell{Row: 0, Col: 0}}
	msg0 := <-incomings[0]

	state, ok := msg0.(ui.GameStateMsg)
	if !ok {
		t.Fatalf("expected GameStateMsg, got: %T", msg0)
	}

	piece := state.Game.Board.At(engine.Cell{Row: 0, Col: 0})
	if *piece != engine.WhiteBishop {
		t.Fatalf("expected WhiteBishop at {0, 0}, got: %v", piece)
	}

	<-incomings[1]
	moves[1] <- ui.MoveRequest{Piece: engine.BlackBishop, Cell: engine.Cell{Row: 0, Col: 0}}
	msg1 := <-incomings[1]

	_, ok = msg1.(ui.ErrorMsg)
	if !ok {
		t.Fatalf("expected ErrorMsg, got: %T", msg1)
	}
}
