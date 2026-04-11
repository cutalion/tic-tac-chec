package game

import "tic-tac-chec/engine"

type Command any

type AttachCommand struct {
	PlayerID PlayerID
	Updates  chan Event
}

type DetachCommand struct {
	PlayerID PlayerID
}

type MoveCommand struct {
	Piece engine.Piece
	To    engine.Cell
}

type RematchCommand struct {
	PlayerID PlayerID
}

type ReactionCommand struct {
	PlayerID PlayerID
	Reaction string
}
