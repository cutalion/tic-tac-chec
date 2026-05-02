package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"tic-tac-chec/internal/game"
	"tic-tac-chec/internal/parse"
	"tic-tac-chec/internal/web/room"

	"github.com/coder/websocket"
)

func ServeRoom(ctx context.Context, ws *websocket.Conn, room *game.Room, participant room.Participant) {
	defer ws.Close(websocket.StatusNormalClosure, "bye")

	commands := make(chan game.Command, 1)
	events := make(chan game.Event, 16)
	defer close(commands)
	// do not close events, it will be closed by the room

	room.Reconnect <- game.ReconnectInfo{
		PlayerID: participant.PlayerID,
		Commands: commands,
		Updates:  events,
	}

	if err := sendMessage(ctx, ws, RoomJoinedMessage{
		Type:   "roomJoined",
		RoomID: room.ID,
		Color:  colorName(room.PlayerColor(participant.PlayerID)),
	}); err != nil {
		slog.Error("room.send_joined_failed", "err", err)
		return
	}

	go func() {
		for event := range events {
			msg, ok := roomEventMessage(event)
			if !ok {
				slog.Warn("room.unknown_event", "event", fmt.Sprintf("%#v", event))
				continue
			}

			err := sendMessage(ctx, ws, msg)
			if err != nil {
				return
			}
		}
	}()

	for {
		msgType, msg, err := ws.Read(ctx)
		if err != nil {
			slog.Error("ws.read_failed", "err", err)
			return
		}

		if msgType != websocket.MessageText {
			continue
		}

		var envelope InboundMessage
		if err := json.Unmarshal(msg, &envelope); err != nil {
			sendMessage(ctx, ws, ErrorMessage{Type: "error", Error: err.Error()})
			continue
		}

		switch envelope.Type {
		case "move":
			var move InboundMoveMessage
			if err := json.Unmarshal(msg, &move); err != nil {
				sendMessage(ctx, ws, ErrorMessage{Type: "error", Error: err.Error()})
				continue
			}

			piece, err := parse.Piece(move.Piece)
			if err != nil {
				sendMessage(ctx, ws, ErrorMessage{Type: "error", Error: err.Error()})
				continue
			}

			target := move.To
			if target == "" {
				target = move.Cell
			}

			to, err := parse.Square(target)
			if err != nil {
				sendMessage(ctx, ws, ErrorMessage{Type: "error", Error: err.Error()})
				continue
			}

			commands <- game.MoveCommand{Piece: piece, To: to}
		case "rematch":
			commands <- game.RematchCommand{PlayerID: participant.PlayerID}
		case "reaction":
			var reaction InboundReactionMessage
			if err := json.Unmarshal(msg, &reaction); err != nil {
				sendMessage(ctx, ws, ErrorMessage{Type: "error", Error: err.Error()})
				continue
			}
			commands <- game.ReactionCommand{PlayerID: participant.PlayerID, Reaction: reaction.Reaction}

		default:
			slog.Warn("ws.invalid_command", "type", envelope.Type)
			sendMessage(ctx, ws, ErrorMessage{Type: "error", Error: "invalid command"})
			continue
		}
	}
}
