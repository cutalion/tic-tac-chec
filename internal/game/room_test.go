package game

import (
	"testing"
	"time"

	"tic-tac-chec/engine"
	"tic-tac-chec/internal/ui"

	"go.uber.org/goleak"
)

func setupRoom() (Room, [2]chan ui.MoveRequest) {
	// create bi-directional channels, so we can manually read/write in tests
	// readonly would be impractical
	moves := [2]chan ui.MoveRequest{
		make(chan ui.MoveRequest),
		make(chan ui.MoveRequest),
	}

	whitePlayer := NewPlayer(moves[0])
	blackPlayer := NewPlayer(moves[1])

	room := NewRoom(whitePlayer, blackPlayer)

	return room, moves
}

func TestRoom(t *testing.T) {
	defer goleak.VerifyNone(t)

	room, moves := setupRoom()
	defer close(moves[0])
	defer close(moves[1])

	go room.Run()

	moves[0] <- ui.MoveRequest{Piece: engine.WhiteBishop, Cell: engine.Cell{Row: 0, Col: 0}}
	msg0 := <-room.Players[0].Incoming

	state, ok := msg0.(ui.GameStateMsg)
	if !ok {
		t.Fatalf("expected GameStateMsg, got: %T", msg0)
	}

	piece := state.Game.Board.At(engine.Cell{Row: 0, Col: 0})
	if *piece != engine.WhiteBishop {
		t.Fatalf("expected WhiteBishop at {0, 0}, got: %v", piece)
	}

	<-room.Players[1].Incoming
	moves[1] <- ui.MoveRequest{Piece: engine.BlackBishop, Cell: engine.Cell{Row: 0, Col: 0}}
	msg1 := <-room.Players[1].Incoming

	_, ok = msg1.(ui.ErrorMsg)
	if !ok {
		t.Fatalf("expected ErrorMsg, got: %T", msg1)
	}
}

func TestBufferedChannelDeliversMessage(t *testing.T) {
	defer goleak.VerifyNone(t)
	moves := [2]chan ui.MoveRequest{
		make(chan ui.MoveRequest),
		make(chan ui.MoveRequest),
	}

	whitePlayer := NewPlayer(moves[0])
	blackPlayer := NewPlayer(moves[1])

	room := NewRoom(whitePlayer, blackPlayer)

	go room.Run()

	moves[0] <- ui.MoveRequest{Piece: engine.WhiteBishop, Cell: engine.Cell{Row: 0, Col: 0}}

	// expecting black to receive the message, not drop it
	select {
	case <-room.Players[1].Incoming:
		// black received a message, expected behavior
	case <-time.After(time.Second):
		t.Fatalf("expected incoming[1] (black) to receive a message, but none was received")
	}

	close(moves[0])
	close(moves[1])
}
