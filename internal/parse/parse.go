package parse

import (
	"fmt"
	"strings"
	"tic-tac-chec/engine"
)

func Piece(piece string) (engine.Piece, error) {
	switch strings.ToUpper(piece) {
	case "WR":
		return engine.WhiteRook, nil
	case "WN", "WK":
		return engine.WhiteKnight, nil
	case "WB":
		return engine.WhiteBishop, nil
	case "WP":
		return engine.WhitePawn, nil
	case "BR":
		return engine.BlackRook, nil
	case "BP":
		return engine.BlackPawn, nil
	case "BN", "BK":
		return engine.BlackKnight, nil
	case "BB":
		return engine.BlackBishop, nil
	default:
		return engine.Piece{}, fmt.Errorf("invalid piece: %s", piece)
	}
}

func Square(square string) (engine.Cell, error) {
	if len(square) != 2 {
		return engine.Cell{}, fmt.Errorf("invalid square: %s", square)
	}

	file := square[0]
	rank := square[1]

	if file < 'a' || file > 'd' || rank < '1' || rank > '4' {
		return engine.Cell{}, fmt.Errorf("invalid square: %s", square)
	}

	col := int(file - 'a')
	row := int('4' - rank)

	return engine.Cell{Row: row, Col: col}, nil
}
