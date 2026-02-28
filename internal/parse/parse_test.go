package parse

import (
	"testing"
	"tic-tac-chec/engine"
)

func TestSquare(t *testing.T) {
	tests := []struct {
		input    string
		expected engine.Cell
	}{
		{"a1", engine.Cell{Row: 3, Col: 0}},
		{"b2", engine.Cell{Row: 2, Col: 1}},
		{"c3", engine.Cell{Row: 1, Col: 2}},
		{"d4", engine.Cell{Row: 0, Col: 3}},
	}

	for _, test := range tests {
		cell, err := Square(test.input)
		if err != nil {
			t.Errorf("Square(%q) returned error: %v", test.input, err)
		}
		if cell != test.expected {
			t.Errorf("Square(%q) = %v, want %v", test.input, cell, test.expected)
		}
	}
}
