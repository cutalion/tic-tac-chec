package main

import (
	"os"

	"github.com/alecthomas/kong"
)

var cli struct {
	Game string `help:"Path to the current state of the game" short:"g" optional:""`
	Start     StartCmd `cmd:"" help:"Start a new game"`
	Move      MoveCmd  `cmd:"" help:"Move a piece"`
}

type MoveCmd struct {
	Piece  string `arg:"" help:"Piece to move: WR, WN, WK, WB, WP, BR, BP, BN, BK, BB"`
	Square string `arg:"" help:"Square to move to: a1, b2, c3, d4"`
}

type StartCmd struct{}

func main() {
	app := NewApp(os.Stdout, os.Stderr)
	ctx := kong.Parse(&cli,
		kong.Name("tic-tac-chec"),
		kong.Description("A chess-themed tic-tac-toe game on a 4x4 board."),
		kong.UsageOnError(),
	)

	var err error
	switch ctx.Command() {
	case "start":
		_, err = app.Start(cli.Game)
	case "move <piece> <square>":
		err = app.Move(cli.Game, cli.Move.Piece, cli.Move.Square)
	default:
		ctx.Fatalf("unknown command: %s", ctx.Command())
	}

	ctx.FatalIfErrorf(err)
}

func (MoveCmd) Help() string {
	return `White: WP=Pawn  WR=Rook  WN/WK=Knight  WB=Bishop

Black: BP=Pawn  BR=Rook  BN/BK=Knight  BB=Bishop

Square: a1 .. d4`
}
