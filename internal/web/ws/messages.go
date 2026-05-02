package ws

import (
	"context"
	"encoding/json"
	"tic-tac-chec/engine"
	"tic-tac-chec/internal/game"

	"github.com/coder/websocket"
)

type LobbyWaitMessage struct {
	Type string `json:"type"`
}

type LobbyPairedMessage struct {
	Type   string      `json:"type"`
	RoomID game.RoomID `json:"roomId"`
}

type RoomJoinedMessage struct {
	Type   string      `json:"type"`
	RoomID game.RoomID `json:"roomId"`
	Color  string      `json:"color"`
}

type ErrorMessage struct {
	Type  string `json:"type"`
	Error string `json:"error"`
}

type InboundMessage struct {
	Type string `json:"type"`
}

type InboundMoveMessage struct {
	InboundMessage
	Piece string `json:"piece"`
	To    string `json:"to"`
	Cell  string `json:"cell"`
}

type InboundReactionMessage struct {
	InboundMessage
	Reaction string `json:"reaction"`
}

type OutboundReactionMessage struct {
	Type     string `json:"type"`
	Reaction string `json:"reaction"`
	Player   string `json:"player"`
	RoomID   string `json:"roomId"`
}

type GameStateMessage struct {
	Type  string           `json:"type"`
	State GameStatePayload `json:"state"`
}

type GameStatePayload struct {
	Board          [engine.BoardSize][engine.BoardSize]*PiecePayload `json:"board"`
	Turn           string                                            `json:"turn"`
	Status         string                                            `json:"status"`
	Winner         *string                                           `json:"winner"`
	PawnDirections PawnDirectionsPayload                             `json:"pawnDirections"`
}

type PawnDirectionsPayload struct {
	White string `json:"white"`
	Black string `json:"black"`
}

type PiecePayload struct {
	Color string `json:"color"`
	Kind  string `json:"kind"`
}

type PairedMessage struct {
	Type  string `json:"type"`
	Color string `json:"color"`
}

func sendMessage(ctx context.Context, sock *websocket.Conn, msg any) error {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	err = sock.Write(ctx, websocket.MessageText, msgBytes)
	if err != nil {
		sock.Close(websocket.StatusInternalError, err.Error())
	}

	return err
}
