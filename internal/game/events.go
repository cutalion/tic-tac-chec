package game

import (
	"tic-tac-chec/engine"
	"time"
)

type Event any

type RoomEvent any

type GameStarted struct {
	RoomID      RoomID
	GameID      GameID
	Game        engine.Game
	GameNumber  uint
	WhitePlayer PlayerID
	BlackPlayer PlayerID
	StartedAt   time.Time
}

func NewGameStarted(
	roomID RoomID,
	gameID GameID,
	game engine.Game,
	gameNumber uint,
	whitePlayer PlayerID,
	blackPlayer PlayerID,
	startedAt time.Time,
) GameStarted {
	return GameStarted{
		RoomID:      roomID,
		GameID:      gameID,
		Game:        game,
		GameNumber:  gameNumber,
		WhitePlayer: whitePlayer,
		BlackPlayer: blackPlayer,
		StartedAt:   startedAt,
	}
}

type StateUpdate struct {
	RoomID     RoomID
	GameID     GameID
	Game       engine.Game
	GameNumber uint
	UpdatedAt  time.Time
}

func NewStateUpdate(
	roomID RoomID,
	gameID GameID,
	game engine.Game,
	gameNumber uint,
	updatedAt time.Time,
) StateUpdate {
	return StateUpdate{
		RoomID:     roomID,
		GameID:     gameID,
		Game:       game,
		GameNumber: gameNumber,
		UpdatedAt:  updatedAt,
	}
}

type MoveApplied struct {
	RoomID     RoomID
	By         PlayerID
	Piece      engine.Piece
	To         engine.Cell
	Seq        uint
	GameNumber uint
	At         time.Time
}

func NewMoveApplied(
	roomID RoomID,
	by PlayerID,
	piece engine.Piece,
	to engine.Cell,
	seq uint,
	gameNumber uint,
	at time.Time,
) MoveApplied {
	return MoveApplied{
		RoomID:     roomID,
		By:         by,
		Piece:      piece,
		To:         to,
		Seq:        seq,
		GameNumber: gameNumber,
		At:         at,
	}
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
