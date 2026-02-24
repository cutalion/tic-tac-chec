package engine

import "fmt"

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

func (p *Piece) String() string {
	if p == nil {
		return "."
	}
	return fmt.Sprintf("[%v %v]", p.Color, p.Kind)
}

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
