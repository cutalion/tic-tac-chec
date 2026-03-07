package engine

import (
	"fmt"
	"strings"
)

func (b *Board) At(pos Cell) *Piece {
	return b[pos.Row][pos.Col]
}

func (b *Board) Find(piece *Piece) (Cell, bool) {
	if piece == nil {
		return Cell{}, false
	}

	for row := range BoardSize {
		for col := range BoardSize {
			pos := Cell{row, col}
			boardPiece := b.At(pos)

			if boardPiece != nil && *boardPiece == *piece {
				return pos, true
			}
		}
	}

	return Cell{}, false
}

// Move relocates a piece to the target cell. If the target is occupied,
// the occupant is silently removed from the board — this implements
// shogi-style capture: since Board holds pointers into Game.Pieces,
// a captured piece remains in Pieces and becomes "in hand" (Board.Find
// returns false for it). This invariant requires that every piece on the
// board is a pointer obtained from Game.Pieces, never a fresh &Piece{}.
func (b *Board) move(piece *Piece, to Cell) error {
	if !to.Valid() {
		return ErrOutOfBounds
	}

	from, found := b.Find(piece)
	if !found {
		return ErrNotOnBoard
	}

	b[from.Row][from.Col] = nil
	b[to.Row][to.Col] = piece
	return nil
}

func (b *Board) Clear() {
	for i := range BoardSize {
		for j := range BoardSize {
			b[i][j] = nil
		}
	}
}

func (b *Board) Lines() []Line {
	var lines []Line

	// rows
	for r := range BoardSize {
		lines = append(lines, b[r][:])
	}

	// columns
	for col := range BoardSize {
		column := Line{}
		for row := range BoardSize {
			column = append(column, b[row][col])
		}

		lines = append(lines, column)
	}

	// diagonals
	diagonal := Line{}
	antiDiagonal := Line{}
	for i := range BoardSize {
		diagonal = append(diagonal, b[i][i])
		antiDiagonal = append(antiDiagonal, b[i][BoardSize-i-1])
	}
	lines = append(lines, diagonal)
	lines = append(lines, antiDiagonal)

	return lines
}

func (c Cell) Valid() bool {
	return c.Row >= 0 && c.Row < BoardSize && c.Col >= 0 && c.Col < BoardSize
}

func (c Cell) String() string {
	return fmt.Sprintf("(%d, %d)", c.Row, c.Col)
}

func (b Board) String() string {
	var out strings.Builder

	out.WriteString("\n")
	out.WriteString("  ")
	for n := range BoardSize {
		fmt.Fprintf(&out, "  %-6v ", n)
	}
	for i := range BoardSize {
		out.WriteString("\n")
		out.WriteString("  ")
		out.WriteString(strings.Repeat("-", (6+3)*BoardSize+1))
		out.WriteString("\n")
		fmt.Fprintf(&out, "%d ", i)
		for j := range BoardSize {
			fmt.Fprintf(&out, "| %-6v ", b[i][j])
		}
		out.WriteString("|")
	}
	out.WriteString("\n")
	out.WriteString("  ")
	out.WriteString(strings.Repeat("-", (6+3)*BoardSize+1))

	return out.String()
}
