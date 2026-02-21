package engine

import (
	"fmt"
	"strings"
)

const BoardSize = 4

type Board [BoardSize][BoardSize]*Piece
type Line []*Piece

type Cell struct {
	Row int
	Col int
}

func (b *Board) At(pos Cell) *Piece {
	return b[pos.Row][pos.Col]
}

func (b *Board) Find(piece *Piece) (Cell, bool) {
	for row := range BoardSize {
		for col := range BoardSize {
			pos := Cell{row, col}

			if b.At(pos) == piece {
				return pos, true
			}
		}
	}

	return Cell{}, false
}

func (b *Board) Move(piece *Piece, to Cell) error {
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
