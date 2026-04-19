package game

import (
	"tic-tac-chec/engine"
	"time"
)

type Event any

type RoomEvent any

type StateUpdate struct {
	RoomID    RoomID
	Game      engine.Game
	Match     uint
	UpdatedAt time.Time
}

type MoveApplied struct {
	RoomID RoomID
	By     PlayerID
	Piece  engine.Piece
	To     engine.Cell
	Seq    uint
	Match  uint
	At     time.Time
}

type SnapshotEvent struct {
	RoomID RoomID
	Game   engine.Game
}

type GameStartedEvent struct {
	RoomID      RoomID
	Game        engine.Game
	PlayerColor engine.Color
}

type ErrorEvent struct {
	Error error
}

type OpponentStatusEvent struct {
	PlayerID PlayerID
	Status   string
}

type OpponentDisconnectedEvent struct {
	PlayerID PlayerID
}

type OpponentReconnectedEvent struct {
	PlayerID PlayerID
}

type OpponentAwayEvent struct {
	PlayerID PlayerID
}

type PairedEvent struct {
	PlayerID PlayerID
	Color    engine.Color
}

type RematchRequestedEvent struct {
	PlayerID PlayerID
}

type ReactionEvent struct {
	PlayerID PlayerID
	Reaction string
}
