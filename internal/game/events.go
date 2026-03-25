package game

import (
	"tic-tac-chec/engine"
)

type Event any

type SnapshotEvent struct {
	RoomID RoomID
	Game   engine.Game
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
