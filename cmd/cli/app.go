package main

import (
	"fmt"
	"io"
	"strings"
	"tic-tac-chec/engine"
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
	a.printGame(game)

	return gameState, nil
}

func (a *App) Move(gameState string, pieceArg string, squareArg string) error {
	game, err := restoreGame(gameState)
	if err != nil {
		return err
	}

	piece, err := parsePiece(pieceArg)
	if err != nil {
		return err
	}

	square, err := parseSquare(squareArg)
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
	a.printGame(game)

	return nil
}

func (a *App) printGame(game *engine.Game) {
	fmt.Fprintln(a.out, "  a  b  c  d")
	for row := range engine.BoardSize {
		label := engine.BoardSize - row
		fmt.Fprintf(a.out, "%d ", label)
		for col := range engine.BoardSize {
			piece := game.Board[row][col]
			if piece == nil {
				fmt.Fprint(a.out, ".  ")
			} else {
				fmt.Fprintf(a.out, "%s%s ", piece.Color, piece.Kind)
			}
		}
		fmt.Fprintln(a.out, label)
	}
	fmt.Fprintln(a.out, "  a  b  c  d")
	fmt.Fprintf(a.out, "White hand: %s\n", handString(game, engine.White))
	fmt.Fprintf(a.out, "Black hand: %s\n", handString(game, engine.Black))

	if game.Status == engine.GameOver {
		fmt.Fprintf(a.out, "Game over. Winner: %s\n", game.Winner)
	} else {
		fmt.Fprintf(a.out, "Next turn: %s\n", game.Turn)
	}
}

var allKinds = []engine.PieceKind{engine.Pawn, engine.Rook, engine.Bishop, engine.Knight}

func handString(game *engine.Game, color engine.Color) string {
	var pieces []string
	for _, kind := range allKinds {
		piece := game.Pieces.Get(color, kind)
		if _, onBoard := game.Board.Find(piece); !onBoard {
			pieces = append(pieces, piece.Color.String()+piece.Kind.String())
		}
	}
	if len(pieces) == 0 {
		return "(none)"
	}
	return strings.Join(pieces, " ")
}

func parsePiece(piece string) (engine.Piece, error) {
	switch strings.ToUpper(piece) {
	case "WR":
		return engine.WhiteRook, nil
	case "WN", "WK":
		return engine.WhiteKnight, nil
	case "WB":
		return engine.WhiteBishop, nil
	case "WP":
		return engine.WhitePawn, nil
	case "BR":
		return engine.BlackRook, nil
	case "BP":
		return engine.BlackPawn, nil
	case "BN", "BK":
		return engine.BlackKnight, nil
	case "BB":
		return engine.BlackBishop, nil
	default:
		return engine.Piece{}, fmt.Errorf("invalid piece: %s", piece)
	}
}

func parseSquare(square string) (engine.Cell, error) {
	if len(square) != 2 {
		return engine.Cell{}, fmt.Errorf("invalid square: %s", square)
	}

	file := square[0]
	rank := square[1]

	if file < 'a' || file > 'd' || rank < '1' || rank > '4' {
		return engine.Cell{}, fmt.Errorf("invalid square: %s", square)
	}

	col := int(file - 'a')
	row := int('4' - rank)

	return engine.Cell{Row: row, Col: col}, nil
}
