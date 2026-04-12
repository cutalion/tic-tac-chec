package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"tic-tac-chec/engine"
	"tic-tac-chec/internal/bot"
	"tic-tac-chec/internal/game"
	"tic-tac-chec/internal/parse"

	"github.com/coder/websocket"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
)

type App struct {
	clients       ClientService
	lobbyRegistry LobbyRegistry
	roomRegistry  RoomRegistry
	bots          map[string]*bot.Bot
}

func NewApp(clients ClientService, bots map[string]*bot.Bot) *App {
	roomRegistry := NewRoomRegistry()
	lobbyRegistry := NewLobbyRegistry(roomRegistry)

	return &App{
		clients:       clients,
		lobbyRegistry: lobbyRegistry,
		roomRegistry:  roomRegistry,
		bots:          bots,
	}
}

func (a *App) CreateClient(w http.ResponseWriter, r *http.Request) {
	client := a.clients.Create()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(clientResponse{Token: string(client.ID)})
}

func (a *App) Me(w http.ResponseWriter, r *http.Request) {
	client, err := a.authenticate(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(clientResponse{Token: string(client.ID)})
}

func (a *App) CreateLobby(w http.ResponseWriter, r *http.Request) {
	lobby := a.lobbyRegistry.Create()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(lobbyResponse{ID: string(lobby.ID)})
}

func (a *App) Lobby(w http.ResponseWriter, r *http.Request) {
	lobbyID := r.PathValue("id")
	if lobbyID == "" {
		http.Error(w, "lobbyId is required", http.StatusBadRequest)
		return
	}

	client, err := a.authenticate(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	lobby := a.lobbyRegistry.Find(LobbyID(lobbyID))

	if lobby == nil {
		http.Error(w, "lobby not found or you are not a participant", http.StatusNotFound)
		return
	}

	a.serveLobby(w, r, lobby, client.ID)
}

func (a *App) DefaultLobby(w http.ResponseWriter, r *http.Request) {
	client, err := a.authenticate(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	a.serveLobby(w, r, a.lobbyRegistry.DefaultLobby(), client.ID)
}

func (a *App) BotGame(w http.ResponseWriter, r *http.Request) {
	if len(a.bots) == 0 {
		http.Error(w, "bot is not available", http.StatusServiceUnavailable)
		return
	}

	client, err := a.authenticate(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Pick difficulty from query param, default to best available
	difficulty := r.URL.Query().Get("difficulty")
	gameBot := a.pickBot(difficulty)
	if gameBot == nil {
		http.Error(w, "bot is not available", http.StatusServiceUnavailable)
		return
	}

	// Create human player with a commands channel
	humanCommands := make(chan game.Command)
	humanPlayer := game.NewPlayer(humanCommands)

	// Create bot player
	botPlayer := gameBot.RunPlayer()

	// Use a synthetic client ID for the bot
	botClientID := ClientID("bot")

	entry := a.roomRegistry.CreateWithPlayers(humanPlayer, botPlayer, [2]ClientID{client.ID, botClientID})
	go entry.Room.Run()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(struct {
		RoomID game.RoomID `json:"roomId"`
	}{RoomID: entry.Room.ID})
}

// pickBot returns the bot for the requested difficulty.
// Falls back to: requested → "hard" → "medium" → "easy" → any available.
func (a *App) pickBot(difficulty string) *bot.Bot {
	if b, ok := a.bots[difficulty]; ok {
		return b
	}
	for _, name := range []string{"hard", "medium", "easy"} {
		if b, ok := a.bots[name]; ok {
			return b
		}
	}
	// Return any available bot
	for _, b := range a.bots {
		return b
	}
	return nil
}

func (a *App) Room(w http.ResponseWriter, r *http.Request) {
	roomID := game.RoomID(r.PathValue("id"))
	if roomID == "" {
		http.Error(w, "roomId is required", http.StatusBadRequest)
		return
	}

	client, err := a.authenticate(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	roomEntry, exists := a.roomRegistry.Lookup(roomID)
	if !exists {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}

	participant, ok := roomEntry.participantByClientID(client.ID)
	if !ok {
		http.Error(w, "you are not a participant of this room", http.StatusForbidden)
		return
	}

	ws, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer ws.Close(websocket.StatusNormalClosure, "bye")

	commands := make(chan game.Command, 1)
	events := make(chan game.Event, 16)
	defer close(commands)
	// do not close events, it will be closed by the room

	room := roomEntry.Room
	room.Reconnect <- game.ReconnectInfo{
		PlayerID: participant.PlayerID,
		Commands: commands,
		Updates:  events,
	}

	if err := a.sendMessage(ws, roomJoinedMessage{
		Type:   "roomJoined",
		RoomID: room.ID,
		Color:  colorName(room.PlayerColor(participant.PlayerID)),
	}); err != nil {
		log.Println("error sending room joined message:", err)
		return
	}

	go func() {
		for event := range events {
			msg, ok := roomEventMessage(event)
			if !ok {
				log.Printf("unknown room event: %#v", event)
				continue
			}

			err := a.sendMessage(ws, msg)
			if err != nil {
				return
			}
		}
	}()

	for {
		msgType, msg, err := ws.Read(context.Background())
		if err != nil {
			log.Println("error reading websocket message:", err)
			return
		}

		if msgType != websocket.MessageText {
			continue
		}

		var envelope inboundMessage
		if err := json.Unmarshal(msg, &envelope); err != nil {
			a.sendMessage(ws, errorMessage{Type: "error", Error: err.Error()})
			continue
		}

		switch envelope.Type {
		case "move":
			var move inboundMoveMessage
			if err := json.Unmarshal(msg, &move); err != nil {
				a.sendMessage(ws, errorMessage{Type: "error", Error: err.Error()})
				continue
			}

			piece, err := parse.Piece(move.Piece)
			if err != nil {
				a.sendMessage(ws, errorMessage{Type: "error", Error: err.Error()})
				continue
			}

			target := move.To
			if target == "" {
				target = move.Cell
			}

			to, err := parse.Square(target)
			if err != nil {
				a.sendMessage(ws, errorMessage{Type: "error", Error: err.Error()})
				continue
			}

			commands <- game.MoveCommand{Piece: piece, To: to}
		case "rematch":
			commands <- game.RematchCommand{PlayerID: participant.PlayerID}
		case "reaction":
			var reaction inboundReactionMessage
			if err := json.Unmarshal(msg, &reaction); err != nil {
				a.sendMessage(ws, errorMessage{Type: "error", Error: err.Error()})
				continue
			}
			commands <- game.ReactionCommand{PlayerID: participant.PlayerID, Reaction: reaction.Reaction}

		default:
			log.Printf("invalid command: %s", envelope.Type)
			a.sendMessage(ws, errorMessage{Type: "error", Error: "invalid command"})
			continue
		}
	}
}

// ------------------------------------------------------------

func (a *App) authenticate(r *http.Request) (*Client, error) {
	token := r.URL.Query().Get("token")
	if token == "" {
		header := r.Header.Get("Authorization")
		if header == "" {
			return nil, ErrUnauthorized
		}

		split := strings.Split(header, " ")
		if len(split) != 2 {
			return nil, ErrUnauthorized
		}

		if split[0] != "Bearer" {
			return nil, ErrUnauthorized
		}

		token = split[1]
	}

	if token == "" {
		return nil, ErrUnauthorized
	}

	client, exists := a.clients.Lookup(ClientID(token))
	if !exists {
		return nil, ErrUnauthorized
	}

	return client, nil
}

func (a *App) serveLobby(w http.ResponseWriter, r *http.Request, lobby *lobby, clientID ClientID) {
	ws, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer ws.Close(websocket.StatusNormalClosure, "we're closing. bye!")

	results, err := lobby.Join(clientID)
	if err != nil {
		ws.Close(websocket.StatusPolicyViolation, err.Error())
		return
	}
	defer lobby.Leave(clientID)

	if err := a.sendMessage(ws, lobbyWaitMessage{Type: "waiting"}); err != nil {
		return
	}

	select {
	case result, ok := <-results:
		if !ok {
			return
		}

		log.Println("received pairing result", result)
		roomEntry := result.RoomEntry
		msg := lobbyPairedMessage{
			Type:   "paired",
			RoomID: roomEntry.Room.ID,
		}

		log.Println("sending lobby paired message", msg)
		if err := a.sendMessage(ws, msg); err != nil {
			log.Println("error sending lobby paired message:", err)
			return
		}
	case <-r.Context().Done():
		return
	}
}

func (a *App) sendMessage(ws *websocket.Conn, msg any) error {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	err = ws.Write(context.Background(), websocket.MessageText, msgBytes)
	if err != nil {
		ws.Close(websocket.StatusInternalError, err.Error())
	}

	return err
}

type clientResponse struct {
	Token string `json:"token"`
}

type lobbyResponse struct {
	ID string `json:"id"`
}

type lobbyWaitMessage struct {
	Type string `json:"type"`
}

type lobbyPairedMessage struct {
	Type   string      `json:"type"`
	RoomID game.RoomID `json:"roomId"`
}

type roomJoinedMessage struct {
	Type   string      `json:"type"`
	RoomID game.RoomID `json:"roomId"`
	Color  string      `json:"color"`
}

type errorMessage struct {
	Type  string `json:"type"`
	Error string `json:"error"`
}

type inboundMessage struct {
	Type string `json:"type"`
}

type inboundMoveMessage struct {
	inboundMessage
	Piece string `json:"piece"`
	To    string `json:"to"`
	Cell  string `json:"cell"`
}

type inboundReactionMessage struct {
	inboundMessage
	Reaction string `json:"reaction"`
}

type outboundReactionMessage struct {
	Type     string `json:"type"`
	Reaction string `json:"reaction"`
	Player   string `json:"player"`
	RoomID   string `json:"roomId"`
}

type gameStateMessage struct {
	Type  string           `json:"type"`
	State gameStatePayload `json:"state"`
}

type gameStatePayload struct {
	Board          [engine.BoardSize][engine.BoardSize]*piecePayload `json:"board"`
	Turn           string                                            `json:"turn"`
	Status         string                                            `json:"status"`
	Winner         *string                                           `json:"winner"`
	PawnDirections pawnDirectionsPayload                             `json:"pawnDirections"`
}

type pawnDirectionsPayload struct {
	White string `json:"white"`
	Black string `json:"black"`
}

type piecePayload struct {
	Color string `json:"color"`
	Kind  string `json:"kind"`
}

type pairedMessage struct {
	Type  string `json:"type"`
	Color string `json:"color"`
}

func roomEventMessage(event game.Event) (any, bool) {
	switch event := event.(type) {
	case game.SnapshotEvent:
		return gameStateMessage{
			Type:  "gameState",
			State: gameStatePayloadFrom(event.Game),
		}, true
	case game.ErrorEvent:
		errText := "unknown error"
		if event.Error != nil {
			errText = event.Error.Error()
		}
		return errorMessage{Type: "error", Error: errText}, true
	case game.OpponentAwayEvent:
		return struct {
			Type string `json:"type"`
		}{Type: "opponentAway"}, true
	case game.OpponentDisconnectedEvent:
		return struct {
			Type string `json:"type"`
		}{Type: "opponentDisconnected"}, true
	case game.OpponentReconnectedEvent:
		return struct {
			Type string `json:"type"`
		}{Type: "opponentReconnected"}, true
	case game.PairedEvent:
		return pairedMessage{
			Type:  "rematchStarted",
			Color: colorName(event.Color),
		}, true
	case game.RematchRequestedEvent:
		return struct {
			Type string `json:"type"`
		}{Type: "rematchRequested"}, true
	case game.ReactionEvent:
		return outboundReactionMessage{
			Type:     "reaction",
			Reaction: event.Reaction,
		}, true
	default:
		return nil, false
	}
}

func gameStatePayloadFrom(g engine.Game) gameStatePayload {
	payload := gameStatePayload{
		Turn:   colorName(g.Turn),
		Status: gameStatusName(g.Status),
		Winner: nil,
		PawnDirections: pawnDirectionsPayload{
			White: pawnDirectionName(g.PawnDirections[engine.White]),
			Black: pawnDirectionName(g.PawnDirections[engine.Black]),
		},
	}

	if g.Winner != nil {
		winner := colorName(*g.Winner)
		payload.Winner = &winner
	}

	for row := range g.Board {
		for col := range g.Board[row] {
			piece := g.Board[row][col]
			if piece == nil {
				continue
			}

			payload.Board[row][col] = &piecePayload{
				Color: colorName(piece.Color),
				Kind:  pieceKindName(piece.Kind),
			}
		}
	}

	return payload
}

func pawnDirectionName(direction engine.PawnDirection) string {
	switch direction {
	case engine.ToBlackSide:
		return "toBlackSide"
	case engine.ToWhiteSide:
		return "toWhiteSide"
	default:
		return ""
	}
}

func colorName(color engine.Color) string {
	switch color {
	case engine.White:
		return "white"
	case engine.Black:
		return "black"
	default:
		return ""
	}
}

func pieceKindName(kind engine.PieceKind) string {
	switch kind {
	case engine.Pawn:
		return "pawn"
	case engine.Rook:
		return "rook"
	case engine.Bishop:
		return "bishop"
	case engine.Knight:
		return "knight"
	default:
		return ""
	}
}

func gameStatusName(status engine.GameStatus) string {
	switch status {
	case engine.GameStarted:
		return "started"
	case engine.GameOver:
		return "over"
	default:
		return ""
	}
}
