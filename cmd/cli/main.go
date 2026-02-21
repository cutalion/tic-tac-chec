package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"tic-tac-chec/engine"
)

func main() {
	gameState := flag.String("game-state", "", "Path to the current state of the game (json file)")
	flag.Parse()
	args := flag.Args()

	if len(args) == 0 {
		printUsage()
		return
	}

	switch args[0] {
	case "start":
		start(gameState)
	case "move":
		move(gameState, args[1:])
	default:
		fmt.Println("Invalid command")
		printUsage()
		return
	}
}

func start(gameState *string) {
	fmt.Println("Starting game...")
	if *gameState == "" {
		fmt.Println("No game state file provided, creating new...")
		path, err := createGameStateFile()
		if err != nil {
			fmt.Println("Error creating game state:", err)
			return
		}

		gameState = &path
		fmt.Println("Game state file created:", gameState)
	}

	game := engine.NewGame()
	err := writeGameState(game, *gameState)
	if err != nil {
		fmt.Println("Error writing game state:", err)
		return
	}

	fmt.Println("Game started, make your move")
	fmt.Printf("Example: tic-tac-chec --game-state=%v, move wp a3", *gameState)

	f, err := os.Open(*gameState)
	if err != nil {
		fmt.Println("Error opening game state file:", err)
		return
	}

	contents, err := io.ReadAll(f)
	if err != nil {
		fmt.Println("Error reading game state file:", err)
		return
	}

	fmt.Println()
	fmt.Println(string(contents))
}

func writeGameState(game *engine.Game, path string) error {
	state := &GameState{
		Board:  game.Board,
		Turn:   Turn(game.Turn),
		Status: GameStatus(game.Status),
		Winner: (*Turn)(game.Winner),
	}

	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	err = os.WriteFile(path, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

func createGameStateFile() (string, error) {
	f, err := os.CreateTemp("", "tic-tac-chec-game-state-*.json")
	if err != nil {
		return "", err
	}
	f.Close()

	return f.Name(), nil
}

func move(gameState *string, args []string) {
	fmt.Println("Moving piece...:", args)
}

func stop(gameState *string) {
	fmt.Println("Stopping game...")
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  tic-tac-chec start")
	fmt.Println("  tic-tac-chec move [PIECE] [SQUARE]")
	fmt.Println("  tic-tac-chec stop")
	fmt.Println("PIECE:")
	fmt.Println("  WR, wr - White Rook")
	fmt.Println("  WN, wn, WK, wk - White Knight")
	fmt.Println("  WB, wb - White Bishop")
	fmt.Println("  WQ, wq - White Queen")
	fmt.Println("  WK, wk - White King")
	fmt.Println("  BP, bp - Black Pawn")
	fmt.Println("  BN, bn, BK, bk - Black Knight")
	fmt.Println("  BB, bb - Black Bishop")
	fmt.Println("  BQ, bq - Black Queen")
	fmt.Println("  BK, bk - Black King")
	fmt.Println("SQUARE:")
	fmt.Println("  a1 .. d4")
}
