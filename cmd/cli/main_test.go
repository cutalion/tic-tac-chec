package main

import (
	"io"
	"os"
	"testing"
)

func TestGameIsPlayable(t *testing.T) {
	f, err := os.CreateTemp("", "tic-tac-chec-test-*.json")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	path := f.Name()
	app := NewApp(io.Discard, io.Discard)
	_, err = app.Start(path)
	if err != nil {
		t.Fatal(err)
	}

	err = app.Move(path, "wp", "a1")
	if err != nil {
		t.Fatal(err)
	}

	err = app.Move(path, "br", "b1")
	if err != nil {
		t.Fatal(err)
	}
}
