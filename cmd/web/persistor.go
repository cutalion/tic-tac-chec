package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"tic-tac-chec/cmd/web/store"
	"tic-tac-chec/engine"
	"tic-tac-chec/internal/game"
	"time"
)

func runPersistor(games *store.GameStore, room *game.Room) {
	listener := make(chan game.RoomEvent, 32)
	cancel := room.Subscribe(listener)

	go func() {
		defer cancel()
		defer close(listener)

		recordGames(games, listener)
	}()
}

func recordGames(games *store.GameStore, listener <-chan game.RoomEvent) {
	ctx := context.Background()

	for event := range listener {
		switch e := event.(type) {
		case game.GameStarted:
			slog.Info("persistor.game_started", "room_id", e.RoomID, "white", e.WhitePlayer, "black", e.BlackPlayer)
			game := store.NewGame(string(e.GameID), string(e.RoomID), string(e.WhitePlayer), string(e.BlackPlayer))

			stateJSON, err := json.Marshal(e.Game)
			if err != nil {
				slog.Error("persistor.marshal_failed", "stage", "create", "err", err)
				continue
			}

			game.State = stateJSON
			err = games.Upsert(ctx, game)
			if err != nil {
				slog.Error("persistor.create_failed", "err", err)
				continue
			}

		case game.StateUpdate:
			jsonState, err := json.Marshal(e.Game)
			if err != nil {
				slog.Error("persistor.marshal_failed", "stage", "update", "err", err)
				continue
			}

			if e.Game.Status == engine.GameOver {
				winner := winnerStr(e.Game.Winner)
				err := games.Finish(ctx, string(e.GameID), winner, jsonState, time.Now())
				if err != nil {
					slog.Error("persistor.finish_failed", "err", err)
				}
			} else {
				err := games.UpdateState(ctx, string(e.GameID), jsonState)
				if err != nil {
					slog.Error("persistor.update_failed", "err", err)
				}
			}
		}
	}
}

func winnerStr(winner *engine.Color) string {
	if winner == nil {
		return "draw"
	}
	if *winner == engine.White {
		return "white"
	}
	if *winner == engine.Black {
		return "black"
	}

	return "draw"
}
