package ws

import (
	"context"
	"log/slog"
	"tic-tac-chec/internal/web/clients"
	"tic-tac-chec/internal/web/lobby"

	"github.com/coder/websocket"
)

func ServeLobby(ctx context.Context, sock *websocket.Conn, lobby *lobby.Lobby, client clients.Client) {
	defer sock.Close(websocket.StatusNormalClosure, "we're closing. bye!")

	results, err := lobby.Join(client)
	if err != nil {
		sock.Close(websocket.StatusPolicyViolation, err.Error())
		return
	}
	defer lobby.Leave(client.ID)

	if err := sendMessage(ctx, sock, LobbyWaitMessage{Type: "waiting"}); err != nil {
		return
	}

	select {
	case result, ok := <-results:
		if !ok {
			return
		}

		slog.Info("lobby.pairing_received", "room_id", result.RoomEntry.Room.ID)
		roomEntry := result.RoomEntry
		msg := LobbyPairedMessage{
			Type:   "paired",
			RoomID: roomEntry.Room.ID,
		}

		slog.Info("lobby.paired_send", "room_id", msg.RoomID)
		if err := sendMessage(ctx, sock, msg); err != nil {
			slog.Error("lobby.paired_send_failed", "err", err)
			return
		}
	case <-ctx.Done():
		return
	}
}
