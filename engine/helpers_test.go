package engine

import (
	"errors"
	"testing"
)

func expectEqual[T comparable](t *testing.T, actual, expected T) {
	t.Helper()

	if actual != expected {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}

func expectError(t *testing.T, err error, target error) {
	t.Helper()

	if err == nil {
		t.Errorf("Expected error \"%v\", got nil", target)
	} else if !errors.Is(err, target) {
		t.Errorf("Expected error \"%v\", got \"%v\"", target, err)
	}
}

func expectNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func expectBoard(t *testing.T, board Board, expected Board) {
	t.Helper()
	for row := range board {
		for col := range board[row] {
			if board[row][col] != expected[row][col] {
				t.Errorf("expected board[%d][%d] to be %v, got %v", row, col, expected[row][col], board[row][col])
			}
		}
	}
	t.Log(board)
}
