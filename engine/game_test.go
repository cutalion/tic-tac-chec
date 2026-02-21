package engine

import (
	"testing"
)

func TestNewGame(t *testing.T) {
	g := NewGame()
	if g.Board != [BoardSize][BoardSize]*Piece{} {
		t.Error("New game board is not empty")
	}

	if g.Turn != White {
		t.Error("New game first turn must be White")
	}
}

func TestFirstMove(t *testing.T) {
	g := NewGame()

	err := g.Move(WhiteRook, Cell{0, 0})

	expectNoError(t, err)
	expectBoard(t, g.Board, Board{
		{g.Piece(WhiteRook), nil, nil, nil},
		{nil, nil, nil, nil},
		{nil, nil, nil, nil},
		{nil, nil, nil, nil},
	})

	t.Log(g.Board)

	if g.Turn != Black {
		t.Error("expected turn to switch")
	}
}

func TestBounds(t *testing.T) {
	tests := []Cell{
		{0, -1}, {-1, 0}, {-1, -1},
		{4, 3}, {3, 4}, {4, 4},
	}

	for _, cell := range tests {
		t.Run(cell.String(), func(t *testing.T) {
			g := NewGame()
			err := g.Move(WhiteRook, cell)
			expectError(t, err, ErrOutOfBounds)
		})
	}
}

func TestPlacingAtOccupiedCell(t *testing.T) {
	g := NewGame()

	g.Board[0][0] = g.Piece(BlackPawn)

	err := g.Move(WhiteRook, Cell{0, 0})
	expectError(t, err, ErrOccupied)
}

func TestValidRookMovement(t *testing.T) {
	validMovesFrom11 := []Cell{
		{0, 1}, {2, 1}, {3, 1},
		{1, 0}, {1, 2}, {1, 3},
	}

	for _, cell := range validMovesFrom11 {
		t.Run(cell.String(), func(t *testing.T) {
			g := NewGame()
			g.Board[1][1] = g.Piece(WhiteRook)

			err := g.Move(WhiteRook, cell)
			expectNoError(t, err)

			expected := Board{}
			expected[cell.Row][cell.Col] = g.Piece(WhiteRook)

			expectBoard(t, g.Board, expected)
		})
	}
}

func TestInvalidRookMovement(t *testing.T) {
	invalidMoves := []struct {
		name string
		from Cell
		to   Cell
	}{
		{"diagonal", Cell{1, 1}, Cell{2, 2}},
		{"invalid", Cell{1, 1}, Cell{2, 3}},
	}

	for _, tt := range invalidMoves {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGame()
			g.Board[tt.from.Row][tt.from.Col] = g.Piece(WhiteRook)

			err := g.Move(WhiteRook, tt.to)
			expectError(t, err, ErrIllegalMove)
		})
	}
}

func TestRookJumpsNotAllowed(t *testing.T) {
	tests := []struct {
		name  string
		from  Cell
		to    Cell
		taken Cell
	}{
		{"horizontal", Cell{1, 0}, Cell{1, 3}, Cell{1, 2}},
		{"horizontal backward", Cell{1, 3}, Cell{1, 0}, Cell{1, 2}},
		{"vertical", Cell{0, 1}, Cell{3, 1}, Cell{2, 1}},
		{"vertical backward", Cell{3, 1}, Cell{0, 1}, Cell{2, 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGame()
			g.Board[tt.from.Row][tt.from.Col] = g.Piece(WhiteRook)
			g.Board[tt.taken.Row][tt.taken.Col] = g.Piece(WhitePawn)

			err := g.Move(WhiteRook, tt.to)
			expectError(t, err, ErrIllegalMove)
		})
	}
}

func TestPawnSwitchesDirection(t *testing.T) {
	g := NewGame()
	g.Board[0][0] = g.Piece(WhitePawn)
	g.Board[3][1] = g.Piece(BlackPawn)

	g.nextTurn()

	if g.PawnDirections[White] != 1 {
		t.Errorf("expected white pawn direction to be 1, got %d", g.PawnDirections[White])
	}
	if g.PawnDirections[Black] != -1 {
		t.Errorf("expected black pawn direction to be -1, got %d", g.PawnDirections[Black])
	}
}

func TestPawnDirectionResetsWhenPawnIsCaptured(t *testing.T) {
	g := NewGame()

	g.Board[0][0] = g.Piece(WhitePawn) // on the black edge
	g.Board[3][1] = g.Piece(BlackPawn) // on the white edge
	g.nextTurn()

	expectEqual(t, g.PawnDirections[White], ToWhiteSide) // turned back, to white side
	expectEqual(t, g.PawnDirections[Black], ToBlackSide) // turned back, to black side

	// both captured (not on the board)
	g.Board[0][0] = nil
	g.Board[3][1] = nil
	g.nextTurn()

	// should reset to initial direction
	expectEqual(t, g.PawnDirections[White], ToBlackSide)
	expectEqual(t, g.PawnDirections[Black], ToWhiteSide)
}

func TestPawnDirectionDoesNotChangeWhenPawnIsNotOnTheEdge(t *testing.T) {
	g := NewGame()

	g.Board[1][0] = g.Piece(WhitePawn)
	g.nextTurn()

	expectEqual(t, g.PawnDirections[White], ToBlackSide)
}

func TestGame(t *testing.T) {
	g := NewGame()

	g.Move(WhiteRook, Cell{0, 0})
	g.Move(BlackRook, Cell{0, 3})

	g.Move(WhitePawn, Cell{1, 0})
	g.Move(BlackPawn, Cell{1, 3})

	g.Move(WhiteBishop, Cell{2, 0})
	g.Move(BlackBishop, Cell{2, 3})

	g.Move(WhiteKnight, Cell{3, 0})

	err := g.Move(BlackKnight, Cell{3, 3})
	expectError(t, err, ErrGameOver)

	expectEqual(t, g.Status, GameOver)
	expectEqual(t, *g.Winner, White)
}
