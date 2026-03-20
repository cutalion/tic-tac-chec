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
	rematch := [2]chan ui.RematchRequest{
		make(chan ui.RematchRequest),
		make(chan ui.RematchRequest),
	}

	whitePlayer := NewPlayer(moves[0], rematch[0])
	blackPlayer := NewPlayer(moves[1], rematch[1])

	room := NewRoom(whitePlayer, blackPlayer)

	return room, moves
}

func TestRoom(t *testing.T) {
	defer goleak.VerifyNone(t)

	room, moves := setupRoom()
	defer close(moves[0])
	defer close(moves[1])
	defer close(room.Quit)

	go room.Run()

	// drain initial game state messages
	<-room.Players[0].Updates
	<-room.Players[1].Updates

	moves[0] <- ui.MoveRequest{Piece: engine.WhiteBishop, Cell: engine.Cell{Row: 0, Col: 0}}
	msg0 := <-room.Players[0].Updates

	state, ok := msg0.(ui.GameStateMsg)
	if !ok {
		t.Fatalf("expected GameStateMsg, got: %T", msg0)
	}

	piece := state.Game.Board.At(engine.Cell{Row: 0, Col: 0})
	if *piece != engine.WhiteBishop {
		t.Fatalf("expected WhiteBishop at {0, 0}, got: %v", piece)
	}

	<-room.Players[1].Updates
	moves[1] <- ui.MoveRequest{Piece: engine.BlackBishop, Cell: engine.Cell{Row: 0, Col: 0}}
	msg1 := <-room.Players[1].Updates

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

	rematch := [2]chan ui.RematchRequest{
		make(chan ui.RematchRequest),
		make(chan ui.RematchRequest),
	}

	whitePlayer := NewPlayer(moves[0], rematch[0])
	blackPlayer := NewPlayer(moves[1], rematch[1])

	room := NewRoom(whitePlayer, blackPlayer)
	defer close(room.Quit)

	go room.Run()

	// drain initial game state messages
	<-room.Players[0].Updates
	<-room.Players[1].Updates

	moves[0] <- ui.MoveRequest{Piece: engine.WhiteBishop, Cell: engine.Cell{Row: 0, Col: 0}}

	// expecting black to receive the message, not drop it
	select {
	case <-room.Players[1].Updates:
		// black received a message, expected behavior
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("expected incoming[1] (black) to receive a message, but none was received")
	}

	close(moves[0])
	close(moves[1])
}

func TestItCheckCurrentMoversColor(t *testing.T) {
	room, moves := setupRoom()
	defer close(moves[0])
	defer close(moves[1])

	go room.Run()

	// drain initial game state messages
	<-room.Players[0].Updates
	<-room.Players[1].Updates

	// white cannot move black bishop
	moves[0] <- ui.MoveRequest{Piece: engine.BlackBishop, Cell: engine.Cell{Row: 0, Col: 0}}
	msg0 := <-room.Players[0].Updates

	err, ok := msg0.(ui.ErrorMsg)
	if !ok {
		t.Fatalf("expected ErrorMsg, got: %T", msg0)
	}

	if err.Err != ErrInvalidMove {
		t.Fatalf("expected error message: %s, got: %s", ErrInvalidMove, err.Err)
	}
}

func TestRoomSurvivesPlayerDisconnect(t *testing.T) {
	room, moves := setupRoom()
	defer close(moves[1])

	go room.Run()

	// drain initial game state messages
	<-room.Players[0].Updates
	<-room.Players[1].Updates

	// white disconnects
	close(moves[0])

	// black should still be able to receive messages
	select {
	case msg := <-room.Players[1].Updates:
		if (msg != ui.OpponentAwayMsg{}) {
			t.Fatalf("expected incoming[1] (black) to receive OpponentAwayMsg, but got: %v", msg)
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("expected incoming[1] (black) to receive a message, but none was received")
	}
}

func TestReconnectToRoom(t *testing.T) {
	room, moves := setupRoom()
	defer close(moves[0])

	go room.Run()

	// drain initial game state messages
	<-room.Players[0].Updates
	<-room.Players[1].Updates

	// black disconnects
	close(moves[1])

	msg1 := <-room.Players[0].Updates
	if (msg1 != ui.OpponentAwayMsg{}) {
		t.Fatalf("expected white to receive OpponentAwayMsg, but got: %v", msg1)
	}

	newMoves := make(chan ui.MoveRequest, 1)
	defer close(newMoves)
	newRematch := make(chan ui.RematchRequest, 1)
	defer close(newRematch)

	newBlack := NewPlayer(newMoves, newRematch)
	newBlack.Color = engine.Black
	room.Reconnect <- newBlack

	msg2 := <-room.Players[0].Updates
	if (msg2 != ui.OpponentReconnectedMsg{}) {
		t.Fatalf("expected white to receive OpponentReconnectedMsg after reconnect, but got: %v", msg2)
	}

	msg3 := <-newBlack.Updates
	_, ok := msg3.(ui.GameStateMsg)
	if !ok {
		t.Fatalf("expected black to receive GameStateMsg after reconnect, but got: %v", msg3)
	}
}
