package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"

	"tic-tac-chec/engine"
	"tic-tac-chec/internal/display"
	"tic-tac-chec/internal/parse"
)

const (
	msgWelcome            = "Welcome to the Tic-Tac-Chec game!"
	msgGameStarting       = "Game is starting!"
	msgWhite              = "You'll be playing White"
	msgBlack              = "You'll be playing Black"
	msgYourTurn           = "It's your turn"
	msgWaitForYourTurn    = "Wait for your turn"
	msgGameOver           = "Game over"
	msgYouWon             = "You won!"
	msgYouLost            = "You lost."
	msgDraw               = "It's a draw."
	msgPlayerDisconnected = "Other player disconnected"
)

var (
	ErrPlayerDisconnected = errors.New("Player disconnected")
)

type Player struct {
	Conn    net.Conn
	Scanner *bufio.Scanner
}

func lobby(conns <-chan net.Conn) {
	for {
		whiteConn, ok := <-conns
		if !ok {
			log.Println("white is not ok")
			return
		}

		whitePlayer := newPlayer(whiteConn)
		whitePlayer.Println(msgWelcome)
		whitePlayer.Println(msgWhite)

		log.Println("white is ready, waiting for black")

		blackConn, ok := <-conns
		if !ok {
			log.Println("black is not ok")
			whiteConn.Close()
			return
		}

		blackPlayer := newPlayer(blackConn)
		blackPlayer.Println(msgWelcome)
		blackPlayer.Println(msgBlack)

		log.Println("black is ready, starting")

		go startGame(whitePlayer, blackPlayer)
	}
}

func startGame(whitePlayer, blackPlayer Player) {
	defer whitePlayer.Conn.Close()
	defer blackPlayer.Conn.Close()

	game := engine.NewGame()

	whitePlayer.Println(msgGameStarting)
	blackPlayer.Println(msgGameStarting)

	whitePlayer.PrintGame(game)
	blackPlayer.PrintGame(game)

	whitePlayer.Println(msgYourTurn)
	blackPlayer.Println(msgWaitForYourTurn)

	for {
		switch game.Status {
		case engine.GameStarted:
			err := tickGame(game, whitePlayer, blackPlayer)
			if err != nil {
				log.Println(err)
				return
			}
		case engine.GameOver:
			endGame(game, whitePlayer, blackPlayer)
			return
		}
	}
}

func tickGame(game *engine.Game, white, black Player) error {
	var currentPlayer Player
	var nextPlayer Player

	if game.Turn == engine.White {
		currentPlayer = white
		nextPlayer = black
	} else {
		currentPlayer = black
		nextPlayer = white
	}

	if !currentPlayer.Scanner.Scan() {
		log.Println(currentPlayer.Scanner.Err())
		nextPlayer.Println(msgPlayerDisconnected)
		return ErrPlayerDisconnected
	}

	piece, cell, err := readCommand(currentPlayer.Scanner.Text())
	if err != nil {
		currentPlayer.Println(err.Error())
		currentPlayer.Println(msgYourTurn)
		return nil // not an critical error, continue to the next tick
	}

	err = game.Move(piece, cell)
	if err != nil {
		currentPlayer.Println(err.Error())
		currentPlayer.Println(msgYourTurn)
		return nil // not an critical error, continue to the next tick
	}

	currentPlayer.PrintGame(game)
	nextPlayer.PrintGame(game)

	currentPlayer.Println(msgWaitForYourTurn)
	nextPlayer.Println(msgYourTurn)

	return nil
}

func endGame(game *engine.Game, white, black Player) {
	white.Println(msgGameOver)
	black.Println(msgGameOver)

	if game.Winner == nil {
		white.Println(msgDraw)
		black.Println(msgDraw)
		return
	}

	if *game.Winner == engine.White {
		white.Println(msgYouWon)
		black.Println(msgYouLost)
	} else {
		white.Println(msgYouLost)
		black.Println(msgYouWon)
	}
}

func readCommand(line string) (engine.Piece, engine.Cell, error) {
	fields := strings.Fields(line)
	if len(fields) != 2 {
		err := fmt.Errorf("Didn't get you! Type piece and cell separated by space")
		return engine.Piece{}, engine.Cell{}, err
	}

	piece, err := parse.Piece(fields[0])
	if err != nil {
		return engine.Piece{}, engine.Cell{}, fmt.Errorf("Didn't get you! %s", err)
	}

	cell, err := parse.Square(fields[1])
	if err != nil {
		return engine.Piece{}, engine.Cell{}, fmt.Errorf("Didn't get you! %s", err)
	}

	return piece, cell, nil
}

func newPlayer(conn net.Conn) Player {
	return Player{Conn: conn, Scanner: bufio.NewScanner(conn)}
}

func (p Player) Println(msg string) {
	fmt.Fprintln(p.Conn, msg)
}

func (p Player) PrintGame(game *engine.Game) {
	display.PrintGame(p.Conn, game)
}
