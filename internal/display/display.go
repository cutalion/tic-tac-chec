package display

import (
	"fmt"
	"io"
	"strings"
	"tic-tac-chec/engine"
)

var allKinds = []engine.PieceKind{engine.Pawn, engine.Rook, engine.Bishop, engine.Knight}

func PrintGame(w io.Writer, game *engine.Game) {
	fmt.Fprintln(w, "  a  b  c  d")
	for row := range engine.BoardSize {
		label := engine.BoardSize - row
		fmt.Fprintf(w, "%d ", label)
		for col := range engine.BoardSize {
			piece := game.Board[row][col]
			if piece == nil {
				fmt.Fprint(w, ".  ")
			} else {
				fmt.Fprintf(w, "%s%s ", piece.Color, piece.Kind)
			}
		}
		fmt.Fprintln(w, label)
	}
	fmt.Fprintln(w, "  a  b  c  d")
	fmt.Fprintf(w, "White hand: %s\n", HandString(game, engine.White))
	fmt.Fprintf(w, "Black hand: %s\n", HandString(game, engine.Black))

	if game.Status == engine.GameOver {
		if game.Winner != nil {
			fmt.Fprintf(w, "Game over. Winner: %s\n", *game.Winner)
		} else {
			fmt.Fprintln(w, "Game over. Draw.")
		}
	} else {
		fmt.Fprintf(w, "Next turn: %s\n", game.Turn)
	}
}

func HandString(game *engine.Game, color engine.Color) string {
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
