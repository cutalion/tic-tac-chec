package ui

import "tic-tac-chec/engine"

// MoveRequest is sent from the UI to the Room via the Moves channel.
type MoveRequest struct {
	Piece engine.Piece
	Cell  engine.Cell
}

// GameStateMsg is received from the Room after a move is applied.
type GameStateMsg struct {
	Game engine.Game
}

// ErrorMsg is received from the Room when a move is invalid.
type ErrorMsg struct {
	Err error
}

// OpponentDisconnectedMsg is received when the other player leaves.
type OpponentDisconnectedMsg struct{}

// OpponentReconnectedMsg is received when the other player reconnects.
type OpponentReconnectedMsg struct{}

// OpponentAwayMsg is received when the other player is no longer connected.
type OpponentAwayMsg struct{}

// PairedMsg is delivered when the lobby pairs this player with an opponent.
type PairedMsg struct {
	Color engine.Color
}
