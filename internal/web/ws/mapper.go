package ws

import (
	"tic-tac-chec/engine"
	"tic-tac-chec/internal/game"
)

func roomEventMessage(event game.Event) (any, bool) {
	switch event := event.(type) {
	case game.SnapshotEvent:
		return GameStateMessage{
			Type:  "gameState",
			State: gameStatePayloadFrom(event.Game),
		}, true
	case game.ErrorEvent:
		errText := "unknown error"
		if event.Error != nil {
			errText = event.Error.Error()
		}
		return ErrorMessage{Type: "error", Error: errText}, true
	case game.OpponentAwayEvent:
		return struct {
			Type string `json:"type"`
		}{Type: "opponentAway"}, true
	case game.OpponentDisconnectedEvent:
		return struct {
			Type string `json:"type"`
		}{Type: "opponentDisconnected"}, true
	case game.OpponentReconnectedEvent:
		return struct {
			Type string `json:"type"`
		}{Type: "opponentReconnected"}, true
	case game.PairedEvent:
		return PairedMessage{
			Type:  "rematchStarted",
			Color: colorName(event.Color),
		}, true
	case game.RematchRequestedEvent:
		return struct {
			Type string `json:"type"`
		}{Type: "rematchRequested"}, true
	case game.ReactionEvent:
		return OutboundReactionMessage{
			Type:     "reaction",
			Reaction: event.Reaction,
		}, true
	default:
		return nil, false
	}
}

func gameStatePayloadFrom(g engine.Game) GameStatePayload {
	payload := GameStatePayload{
		Turn:   colorName(g.Turn),
		Status: gameStatusName(g.Status),
		Winner: nil,
		PawnDirections: PawnDirectionsPayload{
			White: pawnDirectionName(g.PawnDirections[engine.White]),
			Black: pawnDirectionName(g.PawnDirections[engine.Black]),
		},
	}

	if g.Winner != nil {
		winner := colorName(*g.Winner)
		payload.Winner = &winner
	}

	for row := range g.Board {
		for col := range g.Board[row] {
			piece := g.Board[row][col]
			if piece == nil {
				continue
			}

			payload.Board[row][col] = &PiecePayload{
				Color: colorName(piece.Color),
				Kind:  pieceKindName(piece.Kind),
			}
		}
	}

	return payload
}

func pawnDirectionName(direction engine.PawnDirection) string {
	switch direction {
	case engine.ToBlackSide:
		return "toBlackSide"
	case engine.ToWhiteSide:
		return "toWhiteSide"
	default:
		return ""
	}
}

func colorName(color engine.Color) string {
	switch color {
	case engine.White:
		return "white"
	case engine.Black:
		return "black"
	default:
		return ""
	}
}

func pieceKindName(kind engine.PieceKind) string {
	switch kind {
	case engine.Pawn:
		return "pawn"
	case engine.Rook:
		return "rook"
	case engine.Bishop:
		return "bishop"
	case engine.Knight:
		return "knight"
	default:
		return ""
	}
}

func gameStatusName(status engine.GameStatus) string {
	switch status {
	case engine.GameStarted:
		return "started"
	case engine.GameOver:
		return "over"
	default:
		return ""
	}
}
