package main

import (
	"context"
	"encoding/json"
	"log"
	"reflect"
	"tic-tac-chec/cmd/web/store"
	"tic-tac-chec/engine"
	"tic-tac-chec/internal/game"
	"time"
)

func recordGames(games *store.GameStore, room *game.Room) {
	listener := make(chan game.RoomEvent, 32)
	cancel := room.Subscribe(listener)
	ctx := context.Background()
	defer cancel()
	defer close(listener)

	currentGameID := ""

	for event := range listener {
		typ := reflect.TypeOf(event)
		log.Printf("[PERSISTOR]: %s (%v)\n", typ, event)

		switch e := event.(type) {
		case game.GameStarted:
			game, err := store.NewGame(string(e.RoomID), string(e.WhitePlayer), string(e.BlackPlayer))
			if err != nil {
				log.Printf("[PERSISTOR]: failed to create game: %v\n", err)
				currentGameID = ""
				continue
			}

			game.State, err = json.Marshal(e.Game)
			if err != nil {
				log.Printf("[PERSISTOR]: failed to marshal game state: %v\n", err)
				currentGameID = ""
				continue
			}

			err = games.Create(ctx, game)
			if err != nil {
				log.Printf("[PERSISTOR]: failed to create game: %v\n", err)
				currentGameID = ""
				continue
			}

			currentGameID = game.ID
		case game.StateUpdate:
			if currentGameID == "" {
				continue
			}

			jsonState, err := json.Marshal(e.Game)
			if err != nil {
				log.Printf("[PERSISTOR]: failed to marshal game state: %v\n", err)
				currentGameID = ""
				continue
			}

			if e.Game.Status == engine.GameOver {
				winner := winnerStr(e.Game.Winner)
				err = games.Finish(ctx, currentGameID, winner, jsonState, time.Now())
				if err != nil {
					log.Printf("[PERSISTOR]: failed to finish game: %v\n", err)
				}
				currentGameID = ""
			} else {
				err = games.UpdateState(ctx, currentGameID, jsonState)
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
