package ui

import "tic-tac-chec/engine"

// Mode determines whether the game is local or online.
type Mode int

const (
	ModeLocal Mode = iota
	ModeOnline
)

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
