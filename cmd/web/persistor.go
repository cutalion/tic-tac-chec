package main

import (
	"context"
	"encoding/json"
	"log"
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
			log.Printf("[PERSISTOR]: game started. Room: %s, White: %s, Black: %s\n", e.RoomID, e.WhitePlayer, e.BlackPlayer)
			game := store.NewGame(string(e.GameID), string(e.RoomID), string(e.WhitePlayer), string(e.BlackPlayer))

			stateJSON, err := json.Marshal(e.Game)
			if err != nil {
				log.Printf("[PERSISTOR]: failed to marshal game state: %v\n", err)
				continue
			}

			game.State = stateJSON
			err = games.Upsert(ctx, game)
			if err != nil {
				log.Printf("[PERSISTOR]: failed to create game: %v\n", err)
				continue
			}

		case game.StateUpdate:
			jsonState, err := json.Marshal(e.Game)
			if err != nil {
				log.Printf("[PERSISTOR]: failed to marshal game state: %v\n", err)
				continue
			}

			if e.Game.Status == engine.GameOver {
				winner := winnerStr(e.Game.Winner)
				err := games.Finish(ctx, string(e.GameID), winner, jsonState, time.Now())
				if err != nil {
					log.Printf("[PERSISTOR]: failed to finish game: %v\n", err)
				}
			} else {
				err := games.UpdateState(ctx, string(e.GameID), jsonState)
				if err != nil {
					log.Printf("[PERSISTOR]: failed to get game: %v\n", err)
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
