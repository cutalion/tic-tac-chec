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
	ready  chan readyMsg
}

type readyMsg struct {
	token string
	color engine.Color
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

type msgOpponentAway struct {
	Type string `json:"type"`
}

type msgOpponentReconnected struct {
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

type msgTokenExpired struct {
	Type string `json:"type"`
}

type msgRematchRequested struct {
	Type string `json:"type"`
}

type msgRematchStarted struct {
	Type  string `json:"type"`
	Color string `json:"color"`
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

	ready := make(chan readyMsg, 1)
	moves := make(chan ui.MoveRequest)
	rematch := make(chan ui.RematchRequest)
	player := game.NewPlayer(moves, rematch)
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

			case "rematch":
				select {
				case rematch <- ui.RematchRequest{}:
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
		update, ok := <-player.Updates
		if !ok {
			close(done) // signal the read loop to exit
			ws.Close(websocket.StatusNormalClosure, "")
			return
		}

		log.Printf("Received update of type %T", update)

		var msg any
		switch update := update.(type) {
		case ui.GameStateMsg:
			state := wire.ToGameState(&update.Game)
			msg = newMsgGameState(*state)
		case ui.ErrorMsg:
			msg = newMsgError(update.Err)
		case ui.OpponentDisconnectedMsg:
			msg = newMsgOpponentDisconnected()
		case ui.OpponentAwayMsg:
			msg = newMsgOpponentAway()
		case ui.OpponentReconnectedMsg:
			msg = newMsgOpponentReconnected()
		case ui.RematchRequestedMsg:
			msg = newMsgRematchRequested()
		case ui.PairedMsg:
			msg = newMsgRematchStarted(update.Color)
		default:
			log.Println("Unknown update type:", update)
			continue
		}

		if msg == nil {
			continue
		}

		err := sendJSON(ws, r, msg)
		if err != nil {
			log.Println(err)
			return
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
		whiteParticipantToken := participants.register(&room, 0)
		blackParticipantToken := participants.register(&room, 1)

		log.Printf("Room created: white=%s, black=%s\n", whiteParticipantToken, blackParticipantToken)
		go room.Run()

		white.ready <- readyMsg{color: engine.White, token: whiteParticipantToken}
		black.ready <- readyMsg{color: engine.Black, token: blackParticipantToken}
	}
}

func awaitJoinOrReconnect(ws *websocket.Conn, r *http.Request, player game.Player, ready chan readyMsg) error {
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
		case msg := <-ready:
			return sendJSON(ws, r, newMsgPaired(msg.color, msg.token))
		case <-r.Context().Done():
			return r.Context().Err() // exit from handleWebSocket
		}
	case "reconnect":
		token := msg.Token
		participant, exists := participants.lookup(token)
		if !exists {
			return sendJSON(ws, r, newMsgTokenExpired())
		}

		color := participant.room.Players[participant.playerIdx].Color
		player.Color = color

		select {
		case participant.room.Reconnect <- player:
		case <-r.Context().Done():
			return r.Context().Err()
		}

		return sendJSON(ws, r, newMsgPaired(color, token))
	default:
		log.Println("unknown message type")
	}

	return fmt.Errorf("unknown message type")
}

func sendJSON(ws *websocket.Conn, r *http.Request, msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return ws.Write(r.Context(), websocket.MessageText, data)
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

func newMsgOpponentAway() msgOpponentAway {
	return msgOpponentAway{Type: "opponentAway"}
}

func newMsgOpponentReconnected() msgOpponentReconnected {
	return msgOpponentReconnected{Type: "opponentReconnected"}
}

func newMsgPaired(color engine.Color, token string) msgPaired {
	return msgPaired{Type: "paired", Color: wire.ColorToString(color), Token: token}
}

func newMsgTokenExpired() msgTokenExpired {
	return msgTokenExpired{Type: "tokenExpired"}
}

func newMsgRematchRequested() msgRematchRequested {
	return msgRematchRequested{Type: "rematchRequested"}
}

func newMsgRematchStarted(color engine.Color) msgRematchStarted {
	return msgRematchStarted{Type: "rematchStarted", Color: wire.ColorToString(color)}
}
