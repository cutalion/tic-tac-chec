package wire

import (
	"encoding/json"
	"fmt"
	"tic-tac-chec/engine"
)

type GameState struct {
	Board          Board          `json:"board"`
	Turn           Turn           `json:"turn"`
	Status         GameStatus     `json:"status"`
	Winner         *Turn          `json:"winner"`
	PawnDirections PawnDirections `json:"pawnDirections"`
	MoveCount      uint           `json:"moveCount"`
}

type Board engine.Board

func (b Board) MarshalJSON() ([]byte, error) {
	type pieceJSON struct {
		Color string `json:"color"`
		Kind  string `json:"kind"`
	}

	res := [engine.BoardSize][engine.BoardSize]*pieceJSON{}

	for i, row := range b {
		for j, piece := range row {
			if piece == nil {
				res[i][j] = nil
			} else {
				res[i][j] = &pieceJSON{
					Color: ColorToString(piece.Color),
					Kind:  kindToString(piece.Kind),
				}
			}
		}
	}

	return json.Marshal(res)
}

func (b *Board) UnmarshalJSON(data []byte) error {
	type pieceJSON struct {
		Color string `json:"color"`
		Kind  string `json:"kind"`
	}

	var wire [engine.BoardSize][engine.BoardSize]*pieceJSON
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	for i, row := range wire {
		for j, piece := range row {
			if piece == nil {
				b[i][j] = nil
			} else {
				color, err := strToColor(piece.Color)
				if err != nil {
					return err
				}
				kind, err := strToKind(piece.Kind)
				if err != nil {
					return err
				}
				b[i][j] = &engine.Piece{Color: color, Kind: kind}
			}
		}
	}

	return nil
}

type Turn engine.Color

func (t Turn) MarshalJSON() ([]byte, error) {
	return json.Marshal(ColorToString(engine.Color(t)))
}

func (t *Turn) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	color, err := strToColor(str)
	if err != nil {
		return err
	}
	*t = Turn(color)
	return nil
}

type GameStatus engine.GameStatus

func (gs GameStatus) MarshalJSON() ([]byte, error) {
	switch (engine.GameStatus)(gs) {
	case engine.GameStarted:
		return json.Marshal("started")
	case engine.GameOver:
		return json.Marshal("over")
	default:
		panic("unknown game status")
	}
}

func (gs *GameStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	switch str {
	case "started":
		*gs = GameStatus(engine.GameStarted)
	case "over":
		*gs = GameStatus(engine.GameOver)
	default:
		return fmt.Errorf("unknown game status: %s", str)
	}
	return nil
}

type PawnDirections engine.PawnDirections

func (pd PawnDirections) MarshalJSON() ([]byte, error) {
	type intermediate struct {
		White string `json:"white"`
		Black string `json:"black"`
	}

	return json.Marshal(intermediate{
		White: pawnDirectionToString(pd[engine.White]),
		Black: pawnDirectionToString(pd[engine.Black]),
	})
}

func (pd *PawnDirections) UnmarshalJSON(data []byte) error {
	var intermediate struct {
		White string `json:"white"`
		Black string `json:"black"`
	}
	if err := json.Unmarshal(data, &intermediate); err != nil {
		return err
	}

	white, err := pawnDirectionFromString(intermediate.White)
	if err != nil {
		return err
	}
	black, err := pawnDirectionFromString(intermediate.Black)
	if err != nil {
		return err
	}
	pd[engine.White] = white
	pd[engine.Black] = black

	return nil
}

const (
	toWhiteSideStr = "toWhiteSide"
	toBlackSideStr = "toBlackSide"
)

func pawnDirectionToString(direction engine.PawnDirection) string {
	switch direction {
	case engine.ToWhiteSide:
		return toWhiteSideStr
	case engine.ToBlackSide:
		return toBlackSideStr
	default:
		panic("unknown direction")
	}
}

func pawnDirectionFromString(str string) (engine.PawnDirection, error) {
	switch str {
	case toWhiteSideStr:
		return engine.ToWhiteSide, nil
	case toBlackSideStr:
		return engine.ToBlackSide, nil
	default:
		return engine.PawnDirection(0), fmt.Errorf("unknown direction: %s", str) // 0 is not a valid direction; callers must check err
	}
}

func ColorToString(color engine.Color) string {
	switch color {
	case engine.White:
		return "white"
	case engine.Black:
		return "black"
	default:
		panic("unknown color")
	}
}

func strToColor(color string) (engine.Color, error) {
	switch color {
	case "white":
		return engine.White, nil
	case "black":
		return engine.Black, nil
	default:
		return engine.Color(0), fmt.Errorf("unknown color: %s", color)
	}
}

func kindToString(kind engine.PieceKind) string {
	switch kind {
	case engine.Pawn:
		return "pawn"
	case engine.Knight:
		return "knight"
	case engine.Bishop:
		return "bishop"
	case engine.Rook:
		return "rook"
	default:
		panic("unknown kind")
	}
}

func strToKind(kind string) (engine.PieceKind, error) {
	switch kind {
	case "pawn":
		return engine.Pawn, nil
	case "rook":
		return engine.Rook, nil
	case "bishop":
		return engine.Bishop, nil
	case "knight":
		return engine.Knight, nil
	default:
		return engine.PieceKind(0), fmt.Errorf("unknown kind: %s", kind)
	}
}

func GameFromState(state *GameState) *engine.Game {
	game := engine.NewGame()
	for i, row := range state.Board {
		for j, piece := range row {
			if piece == nil {
				continue
			}
			game.Board[i][j] = game.Pieces.Get(piece.Color, piece.Kind)
		}
	}
	game.Turn = engine.Color(state.Turn)
	game.Status = engine.GameStatus(state.Status)
	game.Winner = (*engine.Color)(state.Winner)
	game.PawnDirections = engine.PawnDirections(state.PawnDirections)
	game.MoveCount = state.MoveCount

	return game
}

func ToGameState(game *engine.Game) *GameState {
	return &GameState{
		Board:          Board(game.Board),
		Turn:           Turn(game.Turn),
		Status:         GameStatus(game.Status),
		Winner:         (*Turn)(game.Winner),
		PawnDirections: PawnDirections(game.PawnDirections),
		MoveCount:      game.MoveCount,
	}
}
