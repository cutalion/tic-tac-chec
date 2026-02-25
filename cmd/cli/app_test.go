package main

import (
	"testing"
	"tic-tac-chec/engine"
)

func TestParseSquare(t *testing.T) {
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
		cell, err := parseSquare(test.input)
		if err != nil {
			t.Errorf("parseSquare(%q) returned error: %v", test.input, err)
		}
		if cell != test.expected {
			t.Errorf("parseSquare(%q) = %v, want %v", test.input, cell, test.expected)
		}
	}
}
