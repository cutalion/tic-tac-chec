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
}

var (
	ErrOutOfBounds    = errors.New("cell is out of bounds")
	ErrOccupied       = errors.New("cell is already occupied")
	ErrIllegalMove    = errors.New("illegal move")
	ErrNotOnBoard     = errors.New("piece is not on the board")
	ErrNotYourTurn    = errors.New("it is not your turn")
	ErrGameOver       = errors.New("game is over")
	ErrNotImplemented = errors.New("not implemented")
)

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

	if g.checkGameOver() {
		g.Status = GameOver
		g.Winner = &g.Turn
	} else {
		g.nextTurn()
	}

	return nil
}

func (g *Game) Piece(p Piece) *Piece {
	return g.Pieces.Get(p.Color, p.Kind)
}

// --- Private helpers ---

func (g *Game) movePiece(piece *Piece, cell Cell) error {
	moves, err := g.pieceMoves(piece)
	if err != nil {
		return err
	}

	if !slices.Contains(moves, cell) {
		return fmt.Errorf("%v cannot move to %v: %w", piece, cell, ErrIllegalMove)
	}

	return g.Board.Move(piece, cell)
}

func (g *Game) placePiece(piece *Piece, cell Cell) error {
	p := g.Board.At(cell)

	if p != nil {
		return fmt.Errorf("cell %v is already taken by %v: %w", cell, p, ErrOccupied)
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
