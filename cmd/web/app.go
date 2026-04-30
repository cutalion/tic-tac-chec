package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"tic-tac-chec/cmd/web/store"
	"tic-tac-chec/engine"
	"tic-tac-chec/internal/game"
	"tic-tac-chec/internal/parse"

	"github.com/coder/websocket"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
)

type App struct {
	db             *store.Store
	clients        ClientService
	lobbyRegistry  LobbyRegistry
	roomRegistry   RoomRegistry
	bots           map[string]*Bot
	allowedOrigins []string
}

func NewApp(db *store.Store, allowedOrigins []string) *App {
	bots := initBots(context.Background(), db)
	spawnBot := func(botID string) (game.Player, bool) {
		for _, bot := range bots {
			if bot.Info.ID == botID {
				return bot.Model.RunPlayer(bot.Info.PlayerID), true
			}
		}
		return game.Player{}, false
	}

	roomRegistry := NewRoomRegistry(db.Games(), db.Players(), spawnBot)
	lobbyRegistry := NewLobbyRegistry(roomRegistry, db.Games())
	clients := NewClientService(db.Users())

	app := &App{
		db:             db,
		clients:        clients,
		lobbyRegistry:  lobbyRegistry,
		roomRegistry:   roomRegistry,
		bots:           bots,
		allowedOrigins: allowedOrigins,
	}
	app.restoreActiveGames(context.Background())
	return app
}

func (a *App) CreateClient(w http.ResponseWriter, r *http.Request) {
	client, err := a.clients.Create(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(clientResponse{Token: string(client.ID)})
}

func (a *App) Me(w http.ResponseWriter, r *http.Request) {
	client, err := a.authenticate(r)
	if err != nil {
		a.handleAuthError(w, err)
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
		a.handleAuthError(w, err)
		return
	}

	lobby := a.lobbyRegistry.Find(LobbyID(lobbyID))

	if lobby == nil {
		http.Error(w, "lobby not found or you are not a participant", http.StatusNotFound)
		return
	}

	a.serveLobby(w, r, lobby, *client)
}

func (a *App) DefaultLobby(w http.ResponseWriter, r *http.Request) {
	client, err := a.authenticate(r)
	if err != nil {
		a.handleAuthError(w, err)
		return
	}

	a.serveLobby(w, r, a.lobbyRegistry.DefaultLobby(), *client)
}

func (a *App) BotGame(w http.ResponseWriter, r *http.Request) {
	if len(a.bots) == 0 {
		http.Error(w, "bot is not available", http.StatusServiceUnavailable)
		return
	}

	client, err := a.authenticate(r)
	if err != nil {
		a.handleAuthError(w, err)
		return
	}

	// Pick difficulty from query param, default to best available
	difficulty := r.URL.Query().Get("difficulty")
	bot := a.pickBot(difficulty)
	if bot == nil {
		http.Error(w, "bot is not available", http.StatusServiceUnavailable)
		return
	}

	// Create human player with a commands channel
	humanCommands := make(chan game.Command)
	humanPlayer := game.NewPlayerWithID(humanCommands, client.PlayerID)

	botPlayer := bot.Model.RunPlayer(bot.Info.PlayerID)

	entry := a.roomRegistry.CreateWithPlayers(humanPlayer, botPlayer, [2]ClientID{client.ID, BotClientID})

	runPersistor(a.db.Games(), entry.Room)
	go entry.Room.Run()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(struct {
		RoomID game.RoomID `json:"roomId"`
	}{RoomID: entry.Room.ID})
}

// pickBot returns the bot for the requested difficulty.
// Falls back to: requested → "hard" → "medium" → "easy" → any available.
func (a *App) pickBot(difficulty string) *Bot {
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
		a.handleAuthError(w, err)
		return
	}

	roomEntry, found := a.roomRegistry.Lookup(roomID)
	if !found {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}

	participant, ok := roomEntry.participantByClientID(client.ID)
	if !ok {
		http.Error(w, "you are not a participant of this room", http.StatusForbidden)
		return
	}

	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: a.allowedOrigins,
	})
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
		slog.Error("room.send_joined_failed", "err", err)
		return
	}

	go func() {
		for event := range events {
			msg, ok := roomEventMessage(event)
			if !ok {
				slog.Warn("room.unknown_event", "event", fmt.Sprintf("%#v", event))
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
			slog.Error("ws.read_failed", "err", err)
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
			slog.Warn("ws.invalid_command", "type", envelope.Type)
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

	client, err := a.clients.Lookup(r.Context(), ClientID(token))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}

	return client, nil
}

func (a *App) serveLobby(w http.ResponseWriter, r *http.Request, lobby *lobby, client Client) {
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: a.allowedOrigins,
	})
	if err != nil {
		return
	}
	defer ws.Close(websocket.StatusNormalClosure, "we're closing. bye!")

	results, err := lobby.Join(client)
	if err != nil {
		ws.Close(websocket.StatusPolicyViolation, err.Error())
		return
	}
	defer lobby.Leave(client.ID)

	if err := a.sendMessage(ws, lobbyWaitMessage{Type: "waiting"}); err != nil {
		return
	}

	select {
	case result, ok := <-results:
		if !ok {
			return
		}

		slog.Info("lobby.pairing_received", "room_id", result.RoomEntry.Room.ID)
		roomEntry := result.RoomEntry
		msg := lobbyPairedMessage{
			Type:   "paired",
			RoomID: roomEntry.Room.ID,
		}

		slog.Info("lobby.paired_send", "room_id", msg.RoomID)
		if err := a.sendMessage(ws, msg); err != nil {
			slog.Error("lobby.paired_send_failed", "err", err)
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

func (a *App) handleAuthError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrUnauthorized) {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	http.Error(w, "internal server error", http.StatusInternalServerError)
}

func (a *App) buildRoomFromGame(ctx context.Context, g store.Game) (RoomEntry, error) {
	whitePlayer, err := a.db.Players().Get(ctx, g.WhitePlayerID)
	if err != nil {
		return RoomEntry{}, ErrRoomNotFound
	}
	blackPlayer, err := a.db.Players().Get(ctx, g.BlackPlayerID)
	if err != nil {
		return RoomEntry{}, ErrRoomNotFound
	}

	gamePlayerWhite, clientWhite, err := a.playerFor(whitePlayer)
	if err != nil {
		return RoomEntry{}, ErrRoomNotFound
	}
	gamePlayerBlack, clientBlack, err := a.playerFor(blackPlayer)
	if err != nil {
		return RoomEntry{}, ErrRoomNotFound
	}

	var gameState engine.Game
	err = json.Unmarshal(g.State, &gameState)
	if err != nil {
		return RoomEntry{}, ErrRoomNotFound
	}

	room := game.NewRoom(gamePlayerWhite, gamePlayerBlack)
	room.ID = game.RoomID(g.RoomID)
	room.GameID = game.GameID(g.ID)
	room.Game = &gameState

	entry := RoomEntry{
		Room: room,
		Participants: [2]Participant{
			{ClientID: clientWhite, PlayerID: gamePlayerWhite.ID},
			{ClientID: clientBlack, PlayerID: gamePlayerBlack.ID},
		},
	}
	return entry, nil
}

func (a *App) playerFor(p store.Player) (game.Player, ClientID, error) {
	switch {
	case p.BotID != nil:
		player, ok := a.spawnBot(*p.BotID)
		if !ok {
			return game.Player{}, "", fmt.Errorf("bot %s is not available", *p.BotID)
		}

		return player, BotClientID, nil
	case p.UserID != nil:
		player := game.Player{
			ID:              game.PlayerID(p.ID),
			ConnectionState: game.Disconnected,
			// disconnected, will establish channels and set color on reconnect
			Commands: nil,
			Updates:  nil,
			Color:    engine.Color(0),
		}

		return player, ClientID(*p.UserID), nil
	default:
		return game.Player{}, "", fmt.Errorf("player neither bot nor user")
	}
}

func (a *App) spawnBot(botID string) (game.Player, bool) {
	for _, bot := range a.bots {
		if bot.Info.ID == botID {
			return bot.Model.RunPlayer(bot.Info.PlayerID), true
		}
	}
	return game.Player{}, false
}

func (a *App) restoreActiveGames(ctx context.Context) {
	games, err := a.db.Games().LoadActive(ctx)
	if err != nil {
		return
	}

	for _, g := range games {
		roomEntry, err := a.buildRoomFromGame(ctx, g)
		if err != nil {
			slog.Warn("restore.skip_game", "game_id", g.ID, "err", err)
			continue
		}

		a.roomRegistry.Add(roomEntry)

		runPersistor(a.db.Games(), roomEntry.Room)
		go roomEntry.Room.Run()
	}
	slog.Info("restore.complete", "count", len(games))
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
