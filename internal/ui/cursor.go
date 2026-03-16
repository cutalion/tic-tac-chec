package ui

import (
	"tic-tac-chec/engine"
)

// Represents the cursor position, which can be on a panel or a board cell.
// Only one of PanelIndex or BoardCursor will be set at a time.
type Cursor struct {
	PanelIndex  *int
	BoardCursor *engine.Cell
}

func NewCursor() *Cursor {
	return &Cursor{
		PanelIndex:  new(int),
		BoardCursor: nil,
	}
}

func (c *Cursor) onBoard() bool {
	return c.BoardCursor != nil
}

func (c *Cursor) onPanel() bool {
	return c.PanelIndex != nil
}

func (c *Cursor) enterBoard(row, col int) {
	c.BoardCursor = &engine.Cell{Row: row, Col: col}
	c.PanelIndex = nil
}

func (c *Cursor) enterPanel(index int) {
	c.PanelIndex = &index
	c.BoardCursor = nil
}

func (c *Cursor) moveHorizontally(delta int) {
	if c.onBoard() {
		c.BoardCursor.Col += delta
	} else if c.onPanel() {
		*c.PanelIndex += delta
	}
}

func (c *Cursor) moveVertically(delta int) {
	if c.onBoard() {
		c.BoardCursor.Row += delta
	}
}

func (c *Cursor) col() int {
	if c.onBoard() {
		return c.BoardCursor.Col
	} else if c.onPanel() {
		return *c.PanelIndex
	}

	panic("illegal state: cursor not on board or panel")
}
