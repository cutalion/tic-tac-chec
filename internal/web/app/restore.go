package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"tic-tac-chec/engine"
	"tic-tac-chec/internal/game"
	"tic-tac-chec/internal/web/clients"
	store "tic-tac-chec/internal/web/persistence/sqlite"
	"tic-tac-chec/internal/web/persistor"
	"tic-tac-chec/internal/web/room"
)

func (a *App) buildRoomFromGame(ctx context.Context, g store.Game) (room.Entry, error) {
	whitePlayer, err := a.db.Players().Get(ctx, g.WhitePlayerID)
	if err != nil {
		return room.Entry{}, room.ErrRoomNotFound
	}
	blackPlayer, err := a.db.Players().Get(ctx, g.BlackPlayerID)
	if err != nil {
		return room.Entry{}, room.ErrRoomNotFound
	}

	gamePlayerWhite, clientWhite, err := a.playerFor(whitePlayer)
	if err != nil {
		return room.Entry{}, room.ErrRoomNotFound
	}
	gamePlayerBlack, clientBlack, err := a.playerFor(blackPlayer)
	if err != nil {
		return room.Entry{}, room.ErrRoomNotFound
	}

	var gameState engine.Game
	err = json.Unmarshal(g.State, &gameState)
	if err != nil {
		return room.Entry{}, room.ErrRoomNotFound
	}

	r := game.NewRoom(gamePlayerWhite, gamePlayerBlack)
	r.ID = game.RoomID(g.RoomID)
	r.GameID = game.GameID(g.ID)
	r.Game = &gameState

	entry := room.Entry{
		Room: r,
		Participants: [2]room.Participant{
			{ClientID: clientWhite, PlayerID: gamePlayerWhite.ID},
			{ClientID: clientBlack, PlayerID: gamePlayerBlack.ID},
		},
	}
	return entry, nil
}

func (a *App) playerFor(p store.Player) (game.Player, clients.ClientID, error) {
	switch {
	case p.BotID != nil:
		player, ok := a.spawnBot(*p.BotID)
		if !ok {
			return game.Player{}, "", fmt.Errorf("bot %s is not available", *p.BotID)
		}

		return player, clients.BotClientID, nil
	case p.UserID != nil:
		player := game.Player{
			ID:              game.PlayerID(p.ID),
			ConnectionState: game.Disconnected,
			// disconnected, will establish channels and set color on reconnect
			Commands: nil,
			Updates:  nil,
			Color:    engine.Color(0),
		}

		return player, clients.ClientID(*p.UserID), nil
	default:
		return game.Player{}, "", fmt.Errorf("player neither bot nor user")
	}
}

func (a *App) spawnBot(botID string) (game.Player, bool) {
	for _, bot := range a.bots {
		if bot.Info.ID == botID {
			return bot.Model.RunPlayer(bot.Info.PlayerID), true
		}
	}
	return game.Player{}, false
}

func (a *App) restoreActiveGames(ctx context.Context) {
	games, err := a.db.Games().LoadActive(ctx)
	if err != nil {
		return
	}

	for _, g := range games {
		roomEntry, err := a.buildRoomFromGame(ctx, g)
		if err != nil {
			slog.Warn("restore.skip_game", "game_id", g.ID, "err", err)
			continue
		}

		a.roomRegistry.Add(roomEntry)

		persistor.Run(a.db.Games(), roomEntry.Room)
		go roomEntry.Room.Run()
	}
	slog.Info("restore.complete", "count", len(games))
}
