package game

import (
	"testing"
	"time"

	"tic-tac-chec/engine"

	"go.uber.org/goleak"
)

func setupRoom() (*Room, [2]chan Command) {
	// create bi-directional channels, so we can manually read/write in tests
	// readonly would be impractical
	commands := [2]chan Command{
		make(chan Command),
		make(chan Command),
	}

	whitePlayer := NewPlayer(commands[0])
	blackPlayer := NewPlayer(commands[1])

	room := NewRoom(whitePlayer, blackPlayer)

	return room, commands
}

func TestRoom(t *testing.T) {
	defer goleak.VerifyNone(t)

	room, commands := setupRoom()
	defer close(commands[0])
	defer close(commands[1])
	defer close(room.Quit)

	go room.Run()

	// drain initial game state messages
	<-room.Players[0].Updates
	<-room.Players[1].Updates

	commands[0] <- MoveCommand{Piece: engine.WhiteBishop, To: engine.Cell{Row: 0, Col: 0}}
	msg0 := <-room.Players[0].Updates

	state, ok := msg0.(SnapshotEvent)
	if !ok {
		t.Fatalf("expected GameStateMsg, got: %T", msg0)
	}

	piece := state.Game.Board.At(engine.Cell{Row: 0, Col: 0})
	if *piece != engine.WhiteBishop {
		t.Fatalf("expected WhiteBishop at {0, 0}, got: %v", piece)
	}

	<-room.Players[1].Updates
	commands[1] <- MoveCommand{Piece: engine.BlackBishop, To: engine.Cell{Row: 0, Col: 0}}
	msg1 := <-room.Players[1].Updates

	_, ok = msg1.(ErrorEvent)
	if !ok {
		t.Fatalf("expected ErrorMsg, got: %T", msg1)
	}
}

func TestRoomRunSetsPlayerColors(t *testing.T) {
	defer goleak.VerifyNone(t)
	room, commands := setupRoom()
	defer close(commands[0])
	defer close(commands[1])
	defer close(room.Quit)

	go room.Run()

	if room.Players[0].Color == room.Players[1].Color {
		t.Fatalf("expected players to have different colors, got: %v and %v", room.Players[0].Color, room.Players[1].Color)
	}
}

func TestBufferedChannelDeliversMessage(t *testing.T) {
	defer goleak.VerifyNone(t)
	commands := [2]chan Command{
		make(chan Command),
		make(chan Command),
	}

	whitePlayer := NewPlayer(commands[0])
	blackPlayer := NewPlayer(commands[1])

	room := NewRoom(whitePlayer, blackPlayer)
	defer close(room.Quit)

	go room.Run()

	// drain initial game state messages
	<-room.Players[0].Updates
	<-room.Players[1].Updates

	commands[0] <- MoveCommand{Piece: engine.WhiteBishop, To: engine.Cell{Row: 0, Col: 0}}

	// expecting black to receive the message, not drop it
	select {
	case <-room.Players[1].Updates:
		// black received a message, expected behavior
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("expected incoming[1] (black) to receive a message, but none was received")
	}

	close(commands[0])
	close(commands[1])
}

func TestItCheckCurrentMoversColor(t *testing.T) {
	room, commands := setupRoom()
	defer close(commands[0])
	defer close(commands[1])

	go room.Run()

	// drain initial game state messages
	<-room.Players[0].Updates
	<-room.Players[1].Updates

	// white cannot move black bishop
	commands[0] <- MoveCommand{Piece: engine.BlackBishop, To: engine.Cell{Row: 0, Col: 0}}
	msg0 := <-room.Players[0].Updates

	err, ok := msg0.(ErrorEvent)
	if !ok {
		t.Fatalf("expected ErrorMsg, got: %T", msg0)
	}

	if err.Error != ErrInvalidMove {
		t.Fatalf("expected error message: %s, got: %s", ErrInvalidMove, err.Error)
	}
}

func TestRoomSurvivesPlayerDisconnect(t *testing.T) {
	room, commands := setupRoom()
	defer close(commands[1])

	go room.Run()

	// drain initial game state messages
	<-room.Players[0].Updates
	<-room.Players[1].Updates

	// white disconnects
	close(commands[0])

	// black should still be able to receive messages
	select {
	case msg := <-room.Players[1].Updates:
		if msg != (OpponentAwayEvent{PlayerID: room.Players[0].ID}) {
			t.Fatalf("expected incoming[1] (black) to receive OpponentAwayEvent, but got: %v", msg)
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("expected incoming[1] (black) to receive a message, but none was received after 50ms")
	}
}

func TestReconnectToRoom(t *testing.T) {
	room, commands := setupRoom()
	defer close(commands[0])

	go room.Run()

	// drain initial game state messages
	<-room.Players[0].Updates
	<-room.Players[1].Updates

	// black disconnects
	close(commands[1])

	msg1 := <-room.Players[0].Updates
	if (msg1 != OpponentAwayEvent{PlayerID: room.Players[1].ID}) {
		t.Fatalf("expected white to receive OpponentAwayEvent, but got: %v", msg1)
	}

	newCommands := make(chan Command, 1)
	newUpdates := make(chan Event, 1)
	blackID := room.Players[1].ID
	defer close(newCommands)

	room.Reconnect <- ReconnectInfo{PlayerID: blackID, Commands: newCommands, Updates: newUpdates}

	msg2 := <-room.Players[0].Updates
	if (msg2 != OpponentReconnectedEvent{PlayerID: blackID}) {
		t.Fatalf("expected white to receive OpponentReconnectedEvent after reconnect, but got: %v", msg2)
	}

	msg3 := <-newUpdates
	_, ok := msg3.(SnapshotEvent)
	if !ok {
		t.Fatalf("expected black to receive SnapshotEvent after reconnect, but got: %v", msg3)
	}
}
