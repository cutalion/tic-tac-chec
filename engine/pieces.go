package engine

import "fmt"

type (
	Color     int
	PieceKind int
)

const (
	Pawn PieceKind = iota
	Rook
	Bishop
	Knight
	PieceKindCount
)

const (
	White Color = iota
	Black
	ColorCount
)

const PieceCount = int(PieceKindCount) * int(ColorCount)

type Piece struct {
	Color Color
	Kind  PieceKind
}

var (
	WhitePawn   = Piece{Color: White, Kind: Pawn}
	WhiteRook   = Piece{Color: White, Kind: Rook}
	WhiteBishop = Piece{Color: White, Kind: Bishop}
	WhiteKnight = Piece{Color: White, Kind: Knight}

	BlackPawn   = Piece{Color: Black, Kind: Pawn}
	BlackRook   = Piece{Color: Black, Kind: Rook}
	BlackBishop = Piece{Color: Black, Kind: Bishop}
	BlackKnight = Piece{Color: Black, Kind: Knight}
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

func (p *Piece) String() string {
	if p == nil {
		return "."
	}
	return fmt.Sprintf("[%v %v]", p.Color, p.Kind)
}

type Pieces [ColorCount][PieceKindCount]Piece

func NewPieces() Pieces {
	pieces := Pieces{}
	for color := range ColorCount {
		for kind := range PieceKindCount {
			pieces[color][kind] = Piece{Kind: kind, Color: color}
		}
	}

	return pieces
}

func (p *Pieces) Get(color Color, kind PieceKind) *Piece {
	return &p[color][kind]
}
