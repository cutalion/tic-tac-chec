package engine

import (
	"testing"
)

func TestRookMoves(t *testing.T) {
	g := NewGame()
	rook := g.Piece(WhiteRook)
	g.Board[0][0] = rook

	moves, err := g.rookMoves(rook)
	expectNoError(t, err)

	expected := []Cell{
		{0, 1}, {0, 2}, {0, 3},
		{1, 0},
		{2, 0},
		{3, 0},
	}

	expectCells(t, moves, expected)
}

func TestRookMovesWithObstacle(t *testing.T) {
	g := NewGame()
	rook := g.Piece(WhiteRook)
	g.Board[0][0] = rook
	g.Board[0][2] = g.Piece(WhitePawn)

	moves, err := g.rookMoves(rook)
	expectNoError(t, err)

	expected := []Cell{
		{0, 1},
		{1, 0},
		{2, 0},
		{3, 0},
	}

	expectCells(t, moves, expected)
}

func TestRookMovesWithCapture(t *testing.T) {
	g := NewGame()
	rook := g.Piece(WhiteRook)
	g.Board[0][0] = rook
	g.Board[0][2] = g.Piece(BlackPawn)

	moves, err := g.rookMoves(rook)
	expectNoError(t, err)

	expected := []Cell{
		{0, 1}, {0, 2},
		{1, 0},
		{2, 0},
		{3, 0},
	}

	expectCells(t, moves, expected)
}

func TestBishopMoves(t *testing.T) {
	g := NewGame()
	bishop := g.Piece(WhiteBishop)
	g.Board[2][2] = bishop

	moves, err := g.bishopMoves(bishop)
	expectNoError(t, err)

	expected := []Cell{
		{1, 3},
		{3, 3},
		{3, 1},
		{1, 1},
		{0, 0},
	}

	expectCells(t, moves, expected)
}

func TestBishopMovesWithObstacle(t *testing.T) {
	g := NewGame()
	bishop := g.Piece(WhiteBishop)
	g.Board[2][2] = bishop
	g.Board[1][1] = g.Piece(WhitePawn)

	moves, err := g.bishopMoves(bishop)
	expectNoError(t, err)

	expected := []Cell{
		{1, 3},
		{3, 3},
		{3, 1},
	}

	expectCells(t, moves, expected)
}

func TestBishopMovesWithCapture(t *testing.T) {
	g := NewGame()
	bishop := g.Piece(WhiteBishop)
	g.Board[2][2] = bishop
	g.Board[1][1] = g.Piece(BlackPawn)

	moves, err := g.bishopMoves(bishop)
	expectNoError(t, err)

	expected := []Cell{
		{1, 3},
		{3, 3},
		{3, 1},
		{1, 1},
	}

	expectCells(t, moves, expected)
}

func TestKnightMoves(t *testing.T) {
	g := NewGame()
	knight := g.Piece(WhiteKnight)
	g.Board[1][1] = knight

	moves, err := g.knightMoves(knight)
	expectNoError(t, err)

	expected := []Cell{
		{0, 3},
		{2, 3},
		{3, 2},
		{3, 0},
	}

	expectCells(t, moves, expected)
}

func TestKnightMovesWithObstacle(t *testing.T) {
	g := NewGame()
	knight := g.Piece(WhiteKnight)
	g.Board[1][1] = knight
	g.Board[3][0] = g.Piece(WhitePawn)

	moves, err := g.knightMoves(knight)
	expectNoError(t, err)

	expected := []Cell{
		{0, 3},
		{2, 3},
		{3, 2},
	}

	expectCells(t, moves, expected)
}

func TestKnightMovesWithCapture(t *testing.T) {
	g := NewGame()
	knight := g.Piece(WhiteKnight)
	g.Board[1][1] = knight
	g.Board[3][0] = g.Piece(BlackPawn)

	moves, err := g.knightMoves(knight)
	expectNoError(t, err)

	expected := []Cell{
		{0, 3},
		{2, 3},
		{3, 2},
		{3, 0},
	}

	expectCells(t, moves, expected)
}

func TestPawnMoves(t *testing.T) {
	g := NewGame()
	pawn := g.Piece(WhitePawn)
	g.Board[2][2] = pawn
	g.PawnDirections[White] = ToBlackSide

	moves, err := g.pawnMoves(pawn)
	expectNoError(t, err)

	expected := []Cell{
		{1, 2},
	}

	expectCells(t, moves, expected)
}

func TestPawnMovesWithObstacle(t *testing.T) {
	g := NewGame()
	pawn := g.Piece(WhitePawn)
	g.Board[2][2] = pawn
	g.Board[1][2] = g.Piece(WhiteRook)
	g.PawnDirections[White] = ToBlackSide

	moves, err := g.pawnMoves(pawn)
	expectNoError(t, err)

	var expected []Cell
	expectCells(t, moves, expected)
}

func TestPawnMovesWithCapture(t *testing.T) {
	g := NewGame()
	pawn := g.Piece(WhitePawn)
	g.Board[2][2] = pawn
	g.Board[1][1] = g.Piece(BlackPawn)
	g.Board[1][3] = g.Piece(BlackKnight)
	g.PawnDirections[White] = ToBlackSide

	moves, err := g.pawnMoves(pawn)
	expectNoError(t, err)

	expected := []Cell{
		{1, 2},
		{1, 1},
		{1, 3},
	}

	expectCells(t, moves, expected)
}

func TestPawnMovesWithCaptureAtTheEdge(t *testing.T) {
	g := NewGame()
	pawn := g.Piece(WhitePawn)
	g.Board[2][0] = pawn
	g.Board[1][1] = g.Piece(BlackPawn)
	g.PawnDirections[White] = ToBlackSide

	moves, err := g.pawnMoves(pawn)
	expectNoError(t, err)

	expected := []Cell{
		{1, 0},
		{1, 1},
	}

	expectCells(t, moves, expected)
}
