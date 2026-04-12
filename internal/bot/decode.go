package bot

import "tic-tac-chec/engine"

// DecodeAction converts an action index (0-319) to a Piece and target Cell.
// Indices 0-63: drop actions (piece_kind * 16 + row * 4 + col)
// Indices 64-319: move actions (64 + src_cell * 16 + dst_cell)
//
// For drop actions, returns the piece to drop and the target cell.
// For move actions, returns a zero Piece (caller uses board lookup) and the target cell.
// The source cell for moves is encoded in the index and returned via src.
func DecodeAction(action int, color engine.Color) (piece engine.Piece, src, dst engine.Cell, isDrop bool) {
	if action < 64 {
		// Drop: piece_kind * 16 + row * 4 + col
		kindIdx := action / 16
		remainder := action % 16
		row := remainder / engine.BoardSize
		col := remainder % engine.BoardSize

		piece = engine.Piece{Color: color, Kind: engine.PieceKind(kindIdx)}
		dst = engine.Cell{Row: row, Col: col}
		isDrop = true
		return
	}

	// Move: 64 + src * 16 + dst
	idx := action - 64
	srcIdx := idx / 16
	dstIdx := idx % 16

	src = engine.Cell{Row: srcIdx / engine.BoardSize, Col: srcIdx % engine.BoardSize}
	dst = engine.Cell{Row: dstIdx / engine.BoardSize, Col: dstIdx % engine.BoardSize}
	isDrop = false
	return
}
