package engine

import (
	"errors"
	"fmt"
	"slices"
)

// --- Core types ---

type (
	Color     int
	PieceKind int
)

const (
	White Color = iota
	Black
	ColorCount
)

func (c Color) String() string {
	switch c {
	case White:
		return "W"
	case Black:
		return "B"
	}

	panic("unknown color")
}

const (
	Pawn PieceKind = iota
	Rook
	Bishop
	Knight
	PieceKindCount
)

func (k PieceKind) String() string {
	switch k {
	case Pawn:
		return "P"
	case Rook:
		return "R"
	case Bishop:
		return "B"
	case Knight:
		return "N" // K is used for King in chess. We don't have King, but anyway
	}

	panic("unknown piece kind")
}

const PieceCount = int(PieceKindCount) * int(ColorCount)

type Piece struct {
	Color Color
	Kind  PieceKind
}

type Pieces [ColorCount][PieceKindCount]Piece

// --- Board ---

const BoardSize = 4

type Board [BoardSize][BoardSize]*Piece
type Line []*Piece

type Cell struct {
	Row int
	Col int
}

type BoardSide int

const (
	BlackSide BoardSide = 0 // 0 row at the top
	WhiteSide BoardSide = BoardSize - 1
)

// --- Game ---

type GameStatus int

const (
	GameStarted GameStatus = iota
	GameOver
)

type PawnDirection int

const (
	ToBlackSide PawnDirection = -1
	ToWhiteSide PawnDirection = 1
)

type PawnDirections [ColorCount]PawnDirection

type Game struct {
	Board          Board
	Pieces         Pieces
	Turn           Color
	PawnDirections PawnDirections
	Status         GameStatus
	Winner         *Color
	MoveCount      uint
}

var (
	ErrOutOfBounds = errors.New("cell is out of bounds")
	ErrNotOnBoard  = errors.New("piece is not on the board")
	ErrNotYourTurn = errors.New("it is not your turn")
	ErrGameOver    = errors.New("game is over")
)

type IllegalMoveError struct {
	Piece  Piece
	Target Cell
}

func (e *IllegalMoveError) Error() string {
	return fmt.Sprintf("%v can't move there — illegal move", e.Piece.FriendlyName())
}

type OccupiedError struct {
	OccupiedBy Piece
	Target     Cell
}

func (e *OccupiedError) Error() string {
	return fmt.Sprintf("can't place here — occupied by %v", e.OccupiedBy.FriendlyName())
}

func NewGame() *Game {
	return &Game{
		Turn:   White,
		Pieces: NewPieces(),
		PawnDirections: PawnDirections{
			White: ToBlackSide, // white moves up initially
			Black: ToWhiteSide, // black moves down initially
		},
		Status: GameStarted,
		Winner: nil,
	}
}

func (g *Game) Move(selected Piece, cell Cell) error {
	if g.Status == GameOver {
		return ErrGameOver
	}
	if !cell.Valid() {
		return ErrOutOfBounds
	}

	if selected.Color != g.Turn {
		return ErrNotYourTurn
	}

	piece := g.Piece(selected)
	_, onBoard := g.Board.Find(piece)

	var err error
	if onBoard {
		err = g.movePiece(piece, cell)
	} else {
		err = g.placePiece(piece, cell)
	}

	if err != nil {
		return err
	}

	g.MoveCount++

	if g.checkGameOver() {
		g.Status = GameOver

		// if Turn is ever changed after this point,
		// Winner will still point to the correct value
		winner := g.Turn
		g.Winner = &winner
	} else {
		g.nextTurn()
	}

	return nil
}

func (g *Game) Piece(p Piece) *Piece {
	return g.Pieces.Get(p.Color, p.Kind)
}

func (g *Game) PieceOnBoard(piece Piece) bool {
	_, onBoard := g.Board.Find(g.Piece(piece))
	return onBoard
}

func (g *Game) PieceInHand(piece Piece) bool {
	return !g.PieceOnBoard(piece)
}

// --- Private helpers ---

func (g *Game) movePiece(piece *Piece, cell Cell) error {
	moves, err := g.pieceMoves(piece)
	if err != nil {
		return err
	}

	if !slices.Contains(moves, cell) {
		return &IllegalMoveError{Piece: *piece, Target: cell}
	}

	return g.Board.move(piece, cell)
}

func (g *Game) placePiece(piece *Piece, cell Cell) error {
	p := g.Board.At(cell)

	if p != nil {
		return &OccupiedError{Target: cell, OccupiedBy: *p}
	}

	g.Board[cell.Row][cell.Col] = piece

	return nil
}

func (g *Game) nextTurn() {
	if g.Turn == White {
		g.Turn = Black
	} else {
		g.Turn = White
	}

	g.maybeTurnPawnDirection(g.Piece(WhitePawn))
	g.maybeTurnPawnDirection(g.Piece(BlackPawn))
}

func (g *Game) checkGameOver() bool {
	if g.Status == GameOver {
		return true
	}

	for _, lines := range g.Board.Lines() {
		win := true

		for _, cell := range lines {
			if cell == nil || cell.Color != g.Turn {
				win = false
				break
			}
		}

		if win {
			return true
		}
	}

	return false
}

func (g *Game) maybeTurnPawnDirection(pawn *Piece) {
	pos, onBoard := g.Board.Find(pawn)
	if onBoard {
		if g.PawnDirections[pawn.Color] == ToBlackSide && pos.Row == int(BlackSide) {
			g.PawnDirections[pawn.Color] = ToWhiteSide
		}

		if g.PawnDirections[pawn.Color] == ToWhiteSide && pos.Row == int(WhiteSide) {
			g.PawnDirections[pawn.Color] = ToBlackSide
		}
	} else {
		if pawn.Color == White {
			g.PawnDirections[White] = ToBlackSide
		} else {
			g.PawnDirections[Black] = ToWhiteSide
		}
	}
}

func (g *Game) Clone() *Game {
	clone := NewGame()
	clone.Turn = g.Turn
	clone.PawnDirections = g.PawnDirections
	clone.Status = g.Status

	if g.Winner != nil {
		winner := *g.Winner
		clone.Winner = &winner
	}

	// game board keeps pointers to pieces
	// put new pieces on the same places
	for row := range g.Board {
		for col := range g.Board[row] {
			piece := g.Board[row][col]
			if piece != nil {
				clone.Board[row][col] = clone.Pieces.Get(piece.Color, piece.Kind)
			}
		}
	}

	return clone
}
