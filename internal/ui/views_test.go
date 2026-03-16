package ui

import (
	"testing"

	"tic-tac-chec/engine"
)

func TestShowLocalCursorAlwaysTrueInLocalMode(t *testing.T) {
	m := InitialModel()
	m.Mode = ModeLocal
	m.Game.Turn = engine.White

	if !showCursor(m) {
		t.Errorf("expected showCursor to be true, got false")
	}

	m.Game.Turn = engine.Black
	if !showCursor(m) {
		t.Errorf("expected showCursor to be true, got false")
	}
}

func TestResetCursorAllPiecesInHand(t *testing.T) {
	m := InitialModel()
	m.Mode = ModeLocal
	m.Game.Turn = engine.White

	m.resetCursor()

	if *m.Cursor.PanelIndex != 0 {
		t.Errorf("expected PanelIndex to be 0, got %v", m.Cursor.PanelIndex)
	}
	if m.Cursor.BoardCursor != nil {
		t.Errorf("expected BoardCursor to be nil, got %v", m.Cursor.BoardCursor)
	}
}

func TestResetCursorSomePiecesOnBoard(t *testing.T) {
	m := InitialModel()
	m.Mode = ModeLocal
	m.Game.Turn = engine.White

	m.Game.Board[0][0] = m.Game.Pieces.Get(engine.White, engine.Pawn)
	m.Game.Board[0][1] = m.Game.Pieces.Get(engine.White, engine.Rook)

	m.resetCursor()
	if expected := 2; *m.Cursor.PanelIndex != expected {
		t.Errorf("expected PanelCursor to be %d, got %d", expected, *m.Cursor.PanelIndex)
	}
	if m.Cursor.BoardCursor != nil {
		t.Errorf("expected BoardCursor to be nil, got %v", m.Cursor.BoardCursor)
	}
}

func TestResetCursorAllPiecesOnBoard(t *testing.T) {
	m := InitialModel()
	m.Mode = ModeLocal
	m.Game.Turn = engine.White

	wp := m.Game.Pieces.Get(engine.White, engine.Pawn)
	wr := m.Game.Pieces.Get(engine.White, engine.Rook)
	wb := m.Game.Pieces.Get(engine.White, engine.Bishop)
	wk := m.Game.Pieces.Get(engine.White, engine.Knight)

	m.Game.Board = engine.Board{
		{nil, nil, nil, nil}, // 0
		{nil, wp, wr, nil},   // 1
		{nil, wb, wk, nil},   // 2
		{nil, nil, nil, nil}, // 3
		//0    1    2    3
	}

	m.resetCursor()
	if m.Cursor.PanelIndex != nil {
		t.Errorf("expected PanelCursor to be nil, got %v", m.Cursor.PanelIndex)
	}

	expected := &engine.Cell{Row: 2, Col: 1}
	if *m.Cursor.BoardCursor != *expected {
		t.Errorf("expected BoardCursor to be %v, got %v", expected, m.Cursor.BoardCursor)
	}
}

func TestResetCursorResetsForCurrentTurn(t *testing.T) {
	// After White moves, Turn flips to Black.
	// resetCursor should look at Black's pieces, not White's.
	m := InitialModel()
	m.Mode = ModeLocal

	wp := m.Game.Pieces.Get(engine.White, engine.Pawn)

	m.Game.Board = engine.Board{
		{nil, nil, nil, nil}, // 0
		{nil, wp, nil, nil},  // 1
		{nil, nil, nil, nil}, // 2
		{nil, nil, nil, nil}, // 3
		//0    1    2    3
	}

	m.Game.Turn = engine.Black
	m.resetCursor()

	if *m.Cursor.PanelIndex != 0 {
		t.Errorf("expected PanelCursor to be 0, got %v", m.Cursor.PanelIndex)
	}

	if m.Cursor.BoardCursor != nil {
		t.Errorf("expected BoardCursor to be nil, got %v", m.Cursor.BoardCursor)
	}
}

func TestResetCursorAllOnBoardFlippedMode(t *testing.T) {
	// Online mode as Black — board is flipped, so bottomRow is 0
	// and scan goes upward (0 → 1 → 2 → 3).
	// Should find the first Black piece scanning from row 0 up.

	m := InitialModel()
	m.Mode = ModeOnline
	m.MyColor = engine.Black
	m.Game.Turn = engine.Black

	bp := m.Game.Pieces.Get(engine.Black, engine.Pawn)
	br := m.Game.Pieces.Get(engine.Black, engine.Rook)
	bb := m.Game.Pieces.Get(engine.Black, engine.Bishop)
	bk := m.Game.Pieces.Get(engine.Black, engine.Knight)

	m.Game.Board = engine.Board{
		{nil, nil, nil, nil}, // 0
		{nil, bp, br, nil},   // 1
		{nil, bb, bk, nil},   // 2
		{nil, nil, nil, nil}, // 3
		//0    1    2    3
	}

	m.resetCursor()

	if m.Cursor.PanelIndex != nil {
		t.Errorf("expected PanelCursor to be nil, got %v", m.Cursor.PanelIndex)
	}

	expected := &engine.Cell{Row: 1, Col: 1}
	if m.Cursor.BoardCursor == nil {
		t.Errorf("expected BoardCursor to be %v, got nil", expected)
	}

	if *m.Cursor.BoardCursor != *expected {
		t.Errorf("expected BoardCursor to be %v, got %v", expected, m.Cursor.BoardCursor)
	}
}

func TestShowLocalCursorFalseInOnlineModeWhenNotMyTurn(t *testing.T) {
	m := InitialModel()
	m.Mode = ModeOnline
	m.MyColor = engine.White
	m.Game.Turn = engine.Black

	if showCursor(m) {
		t.Errorf("expected showCursor to be false, got true")
	}

	m.Game.Turn = engine.White
	if !showCursor(m) {
		t.Errorf("expected showCursor to be true, got false")
	}
}
