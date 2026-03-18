package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"tic-tac-chec/engine"
	"tic-tac-chec/internal/game"
	"tic-tac-chec/internal/parse"
	"tic-tac-chec/internal/ui"
	"tic-tac-chec/internal/wire"

	"github.com/coder/websocket"
)

type playerConn struct {
	player game.Player
	ready  chan engine.Color
}

type msgError struct {
	Type  string `json:"type"`
	Error string `json:"error"`
}

type msgOpponentDisconnected struct {
	Type string `json:"type"`
}

type msgGameState struct {
	Type  string         `json:"type"`
	State wire.GameState `json:"state"`
}

type msgPaired struct {
	Type  string `json:"type"`
	Color string `json:"color"`
}

var lobby = make(chan playerConn)

func main() {
	mux := http.NewServeMux()
	mux.Handle("GET /", staticHandler())
	mux.HandleFunc("GET /ws", handleWebSocket)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	go lobbyLoop(lobby)

	log.Println("Listening on :" + port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	ws, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer ws.Close(websocket.StatusInternalError, "")

	moves := make(chan ui.MoveRequest)
	ready := make(chan engine.Color, 1)
	player := game.NewPlayer(moves)

	var closeOnce sync.Once
	closeMoves := func() { closeOnce.Do(func() { close(moves) }) }
	defer closeMoves()

	go func() {
		lobby <- playerConn{player: player, ready: ready}
	}()

	select {
	case color := <-ready:
		paired, err := json.Marshal(newMsgPaired(color))
		if err != nil {
			log.Println(err)
			return
		}
		err = ws.Write(r.Context(), websocket.MessageText, paired)
		if err != nil {
			log.Println(err)
			return
		}
	case <-r.Context().Done():
		return
	}

	done := make(chan struct{})

	// read loop
	go func() {
		defer closeMoves()

		for {
			msgType, data, err := ws.Read(r.Context())
			if err != nil {
				log.Println(err)
				return
			}

			var command struct {
				Type  string `json:"type"`
				Piece string `json:"piece"`
				Cell  string `json:"cell"`
			}
			switch msgType {
			case websocket.MessageText:
				if err := json.Unmarshal(data, &command); err != nil {
					log.Println(err)
					continue
				}

				cell, err := parse.Square(command.Cell)
				if err != nil {
					log.Println(err)
					continue
				}

				piece, err := parse.Piece(command.Piece)
				if err != nil {
					log.Println(err)
					continue
				}

				select {
				case moves <- ui.MoveRequest{Piece: piece, Cell: cell}:
				case <-done:
					return
				}
			}

			log.Printf("Received message of type %s: %s", msgType, data)
		}
	}()

	// write loop
	for {
		msg, ok := <-player.Updates
		if !ok {
			close(done) // signal the read loop to exit
			ws.Close(websocket.StatusNormalClosure, "")
			return
		}

		log.Printf("Updates message %v of type %T", msg, msg)

		switch msg := msg.(type) {
		case ui.GameStateMsg:
			state := wire.ToGameState(&msg.Game)
			data, err := json.Marshal(newMsgGameState(*state))
			if err != nil {
				log.Println(err)
				return
			}
			err = ws.Write(r.Context(), websocket.MessageText, data)
			if err != nil {
				log.Println(err)
				return
			}
		case ui.ErrorMsg:
			data, err := json.Marshal(newMsgError(msg.Err))
			if err != nil {
				log.Println(err)
				return
			}
			err = ws.Write(r.Context(), websocket.MessageText, data)
			if err != nil {
				log.Println(err)
				return
			}
		case ui.OpponentDisconnectedMsg:
			data, err := json.Marshal(newMsgOpponentDisconnected())
			if err != nil {
				log.Println(err)
				return
			}
			err = ws.Write(r.Context(), websocket.MessageText, data)
			if err != nil {
				log.Println(err)
				return
			}
		}
	}
}

func lobbyLoop(lobby chan playerConn) {
	for {
		white, ok := <-lobby
		if !ok {
			return
		}

		black, ok := <-lobby
		if !ok {
			return
		}

		white.player.Color = engine.White
		black.player.Color = engine.Black

		room := game.NewRoom(white.player, black.player)

		go room.Run()

		white.ready <- white.player.Color
		black.ready <- black.player.Color
	}
}

func newMsgGameState(state wire.GameState) msgGameState {
	return msgGameState{Type: "gameState", State: state}
}

func newMsgError(err error) msgError {
	return msgError{Type: "error", Error: err.Error()}
}

func newMsgOpponentDisconnected() msgOpponentDisconnected {
	return msgOpponentDisconnected{Type: "opponentDisconnected"}
}

func newMsgPaired(color engine.Color) msgPaired {
	return msgPaired{Type: "paired", Color: wire.ColorToString(color)}
}
