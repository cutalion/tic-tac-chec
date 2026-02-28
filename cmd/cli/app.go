package main

import (
	"fmt"
	"io"
	"tic-tac-chec/engine"
	"tic-tac-chec/internal/display"
	"tic-tac-chec/internal/parse"
)

type App struct {
	out io.Writer
	err io.Writer
}

func NewApp(out io.Writer, err io.Writer) *App {
	return &App{out: out, err: err}
}

func (a *App) Start(gameState string) (string, error) {
	if gameState == "" {
		path, err := createGameStateFile()
		if err != nil {
			return "", err
		}

		gameState = path
	}

	game := engine.NewGame()
	err := writeGameState(game, gameState)
	if err != nil {
		return "", err
	}

	fmt.Fprintln(a.out, "Game started, make your move")
	fmt.Fprintf(a.out, "Example: tic-tac-chec --game=%v move wp a3\n", gameState)
	display.PrintGame(a.out, game)

	return gameState, nil
}

func (a *App) Move(gameState string, pieceArg string, squareArg string) error {
	game, err := restoreGame(gameState)
	if err != nil {
		return err
	}

	piece, err := parse.Piece(pieceArg)
	if err != nil {
		return err
	}

	square, err := parse.Square(squareArg)
	if err != nil {
		return err
	}

	if err = game.Move(piece, square); err != nil {
		return err
	}

	if err = writeGameState(game, gameState); err != nil {
		return err
	}

	fmt.Fprintln(a.out, "Piece moved successfully")
	display.PrintGame(a.out, game)

	return nil
}
