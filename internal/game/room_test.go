package game

import (
	"testing"
	"time"

	"tic-tac-chec/engine"

	"github.com/stretchr/testify/require"
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

func TestReactionsWorks(t *testing.T) {
	room, commands := setupRoom()
	defer close(commands[0])

	go room.Run()

	// drain initial game state messages
	<-room.Players[0].Updates
	<-room.Players[1].Updates

	commands[0] <- ReactionCommand{PlayerID: room.Players[0].ID, Reaction: "😂"}
	event1, ok := <-room.Players[0].Updates
	if !ok {
		t.Fatalf("expected white to receive reaction event, but none was received")
	}
	if _, ok := event1.(ReactionEvent); !ok {
		t.Fatalf("expected white to receive ReactionEvent, but got: %v", event1)
	}

	if event1.(ReactionEvent).Reaction != "😂" {
		t.Fatalf("expected white to receive reaction '😂', but got: %s", event1.(ReactionEvent).Reaction)
	}

	event2, ok := <-room.Players[1].Updates
	if !ok {
		t.Fatalf("expected black to receive reaction event, but none was received")
	}
	if _, ok := event2.(ReactionEvent); !ok {
		t.Fatalf("expected black to receive ReactionEvent, but got: %v", event2)
	}
	if event2.(ReactionEvent).Reaction != "😂" {
		t.Fatalf("expected black to receive reaction '😂', but got: %s", event2.(ReactionEvent).Reaction)
	}
}

func TestRoom_SubscribeReceivesMoveAppliedThenStateUpdate(t *testing.T) {
	room, commands := setupRoom()
	defer close(commands[0])

	subscriber := make(chan RoomEvent, 10)
	cancel := room.Subscribe(subscriber)
	defer cancel()

	go room.Run()

	<-subscriber // drain GameStarted message

	commands[0] <- MoveCommand{
		Piece: engine.WhiteBishop,
		To:    engine.Cell{Row: 0, Col: 0},
	}
	event1, ok := <-subscriber
	if !ok {
		t.Fatalf("expected move event to be received, but none was received")
	}
	if _, ok := event1.(MoveApplied); !ok {
		t.Fatalf("expected MoveApplied, but got: %v", event1)
	}

	event2, ok := <-subscriber
	if !ok {
		t.Fatalf("expected state update to be received after move, but none was received")
	}
	if _, ok := event2.(StateUpdate); !ok {
		t.Fatalf("expected StateUpdate, but got: %v", event2)
	}
}

func TestRoom_CancelStopsDelivery(t *testing.T) {
	room, commands := setupRoom()
	defer close(commands[0])

	subscriber := make(chan RoomEvent, 10)
	cancel := room.Subscribe(subscriber)
	cancel()

	go room.Run()

	commands[0] <- MoveCommand{
		Piece: engine.WhiteBishop,
		To:    engine.Cell{Row: 0, Col: 0},
	}
	select {
	case <-subscriber:
		t.Fatalf("expected no more events after cancel, but got one")
	default:
	}
}

func TestRoom_MultipleSubscribers(t *testing.T) {
	room, commands := setupRoom()
	defer close(commands[0])

	subscriber1 := make(chan RoomEvent, 10)
	cancel1 := room.Subscribe(subscriber1)
	defer cancel1()

	subscriber2 := make(chan RoomEvent, 10)
	cancel2 := room.Subscribe(subscriber2)
	defer cancel2()

	go room.Run()

	<-subscriber1 // drain GameStarted message
	<-subscriber2

	commands[0] <- MoveCommand{
		Piece: engine.WhiteBishop,
		To:    engine.Cell{Row: 0, Col: 0},
	}

	event1, ok := <-subscriber1
	if !ok {
		t.Fatalf("expected move event to be received, but none was received")
	}
	if _, ok := event1.(MoveApplied); !ok {
		t.Fatalf("expected MoveApplied, but got: %v", event1)
	}

	event2, ok := <-subscriber2
	if !ok {
		t.Fatalf("expected move event to be received, but none was received")
	}
	if _, ok := event2.(MoveApplied); !ok {
		t.Fatalf("expected MoveApplied, but got: %v", event2)
	}
}

func TestRoom_SlowSubscriberDoesntStallOthers(t *testing.T) {
	room, commands := setupRoom()
	defer close(commands[0])

	slow := make(chan RoomEvent, 1)
	cancelSlow := room.Subscribe(slow)
	defer cancelSlow()

	fast := make(chan RoomEvent, 16)
	cancelFast := room.Subscribe(fast)
	defer cancelFast()

	go room.Run()

	commands[0] <- MoveCommand{Piece: engine.WhiteBishop, To: engine.Cell{Row: 0, Col: 0}}
	commands[1] <- MoveCommand{Piece: engine.BlackBishop, To: engine.Cell{Row: 3, Col: 3}}

	if _, ok := <-fast; !ok {
		t.Fatalf("expected move event to be received, but none was received")
	}
	if _, ok := <-fast; !ok {
		t.Fatalf("expected move event to be received, but none was received")
	}
	if _, ok := <-fast; !ok {
		t.Fatalf("expected move event to be received, but none was received")
	}
	if _, ok := <-fast; !ok {
		t.Fatalf("expected move event to be received, but none was received")
	}
}

func TestRoom_QuitClosesSubscribers(t *testing.T) {
	room, commands := setupRoom()
	defer close(commands[0])

	sub := make(chan RoomEvent, 16)
	cancel := room.Subscribe(sub)
	defer cancel()

	go room.Run()

	room.Quit <- struct{}{}

	for {
		_, ok := <-sub
		if !ok {
			return
		}
	}
}

func TestRoom_RematchWithSubscriber(t *testing.T) {
	room, commands := setupRoom()
	defer close(commands[0])
	defer close(commands[1])
	defer close(room.Quit)

	sub := make(chan RoomEvent, 1000)
	cancel := room.Subscribe(sub)
	defer cancel()

	go room.Run()

	firstEvent := <-sub
	gs1, ok := firstEvent.(GameStarted)
	require.True(t, ok, "first game event should be GameStarted, got %T", firstEvent)
	require.Equal(t, uint(1), gs1.GameNumber)
	initialWhite := gs1.WhitePlayer

	commands[0] <- RematchCommand{PlayerID: room.Players[0].ID}
	commands[1] <- RematchCommand{PlayerID: room.Players[1].ID}

	var gs2 GameStarted
	for {
		ev := <-sub
		if gs, ok := ev.(GameStarted); ok {
			gs2 = gs
			break
		}
	}

	require.Equal(t, uint(2), gs2.GameNumber)
	require.NotEqual(t, initialWhite, gs2.WhitePlayer, "colors should be swapped after rematch")
}
