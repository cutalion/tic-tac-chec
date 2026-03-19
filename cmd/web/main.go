package main

import (
	"encoding/json"
	"errors"
	"fmt"
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
	ready  chan Participant
}

type jsonMsg struct {
	Type  string `json:"type"`
	Piece string `json:"piece"`
	Cell  string `json:"cell"`
	Token string `json:"token"`
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
	Token string `json:"token"`
}

var lobby = make(chan playerConn)
var participants = NewParticipants()

var (
	ErrUnsupportedMessageType = errors.New("unsupported message type")
)

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

	ready := make(chan Participant, 1)
	moves := make(chan ui.MoveRequest)
	player := game.NewPlayer(moves)
	done := make(chan struct{})

	var closeOnce sync.Once
	closeMoves := func() { closeOnce.Do(func() { close(moves) }) }
	defer closeMoves()

	for {
		if r.Context().Err() != nil {
			return
		}

		err := awaitJoinOrReconnect(ws, r, player, ready)

		switch {
		case errors.Is(err, ErrUnsupportedMessageType):
			continue // unsupported message type, skip and wait for next message
		case err != nil:
			log.Println(err) // we cannot handle this error, return from handleWebSocket
			return
		}

		break
	}

	// read loop
	go func() {
		defer closeMoves()

		for {
			msgType, data, err := ws.Read(r.Context())
			if err != nil {
				log.Println(err)
				return
			}

			log.Printf("Received message of type %s: %s", msgType, data)
			if msgType != websocket.MessageText {
				log.Println("Not a text message. Skipping")
				continue
			}

			var msg jsonMsg
			if err := json.Unmarshal(data, &msg); err != nil {
				log.Println(err)
				continue
			}

			switch msg.Type {
			case "move":
				cell, err := parse.Square(msg.Cell)
				if err != nil {
					log.Println(err)
					continue
				}

				piece, err := parse.Piece(msg.Piece)
				if err != nil {
					log.Println(err)
					continue
				}

				select {
				case moves <- ui.MoveRequest{Piece: piece, Cell: cell}:
				case <-done:
					return
				}

			default:
				log.Println("Unknown message type:", msg.Type)
				continue
			}
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
		whiteParticipantToken := participants.register(&room, engine.White)
		blackParticipantToken := participants.register(&room, engine.Black)

		go room.Run()

		white.ready <- Participant{color: engine.White, token: whiteParticipantToken}
		black.ready <- Participant{color: engine.Black, token: blackParticipantToken}
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

func newMsgPaired(color engine.Color, token string) msgPaired {
	return msgPaired{Type: "paired", Color: wire.ColorToString(color), Token: token}
}

func awaitJoinOrReconnect(ws *websocket.Conn, r *http.Request, player game.Player, ready chan Participant) error {
	msgType, data, err := ws.Read(r.Context())
	if err != nil {
		return err
	}

	log.Printf("Received message of type %s: %s", msgType, data)
	if msgType != websocket.MessageText {
		return ErrUnsupportedMessageType
	}

	var msg jsonMsg
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Println(err)
		return err
	}

	switch msg.Type {
	case "join":
		// send the player to the lobby
		// the lobby will assign a color and send it back through the "ready" channel
		// when the second player joins
		go func() {
			lobby <- playerConn{player: player, ready: ready}
		}()

		// lobby assigns color and sends it to the "ready" channel
		// we read the color, assign it to the player, and send "paired" response to the client
		select {
		case participant := <-ready:
			paired, err := json.Marshal(newMsgPaired(participant.color, participant.token))
			if err != nil {
				return err
			}

			err = ws.Write(r.Context(), websocket.MessageText, paired)
			if err != nil {
				return err
			}
		case <-r.Context().Done():
			return r.Context().Err() // exit from handleWebSocket
		}

		return nil
	case "reconnect":
		// todo
	default:
		log.Println("unknown message type")
	}

	return fmt.Errorf("unknown message type")
}
