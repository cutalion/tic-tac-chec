package main

import (
	"encoding/json"
	"tic-tac-chec/engine"
)

type GameState struct {
	Board  engine.Board `json:"board"`
	Turn   Turn         `json:"turn"`
	Status GameStatus   `json:"status"`
	Winner *Turn        `json:"winner"`
}

type Turn engine.Color
type GameStatus engine.GameStatus

func (t Turn) MarshalJSON() ([]byte, error) {
	switch (engine.Color)(t) {
	case engine.White:
		return json.Marshal("white")
	case engine.Black:
		return json.Marshal("black")
	default:
		panic("unknown color")
	}
}

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
