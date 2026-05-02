package api

import (
	"encoding/json"
	"net/http"
	"tic-tac-chec/internal/game"
	"tic-tac-chec/internal/web/bots"
	"tic-tac-chec/internal/web/clients"
	"tic-tac-chec/internal/web/lobby"
	"tic-tac-chec/internal/web/persistor"
	"tic-tac-chec/internal/web/ws"

	"github.com/coder/websocket"
)

func (a *API) CreateClient(w http.ResponseWriter, r *http.Request) {
	client, err := a.clients.Create(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(clientResponse{Token: string(client.ID)})
}

func (a *API) Me(w http.ResponseWriter, r *http.Request) {
	client, err := a.authenticate(r)
	if err != nil {
		a.handleAuthError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(clientResponse{Token: string(client.ID)})
}

func (a *API) CreateLobby(w http.ResponseWriter, r *http.Request) {
	lobby := a.lobbyRegistry.Create()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(lobbyResponse{ID: string(lobby.ID)})
}

func (a *API) BotGame(w http.ResponseWriter, r *http.Request) {
	if len(a.bots) == 0 {
		msg := "bot is not available"
		if bots.UnavailableReason != "" {
			msg += ": " + bots.UnavailableReason
		}
		http.Error(w, msg, http.StatusServiceUnavailable)
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

	entry := a.roomRegistry.CreateWithPlayers(
		humanPlayer, botPlayer, [2]clients.ClientID{client.ID, clients.BotClientID},
	)

	persistor.Run(a.db.Games(), entry.Room)
	go entry.Room.Run()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(struct {
		RoomID game.RoomID `json:"roomId"`
	}{RoomID: entry.Room.ID})
}

func (a *API) Lobby(w http.ResponseWriter, r *http.Request) {
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

	l := a.lobbyRegistry.Find(lobby.LobbyID(lobbyID))

	if l == nil {
		http.Error(w, "lobby not found or you are not a participant", http.StatusNotFound)
		return
	}

	sock, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: a.allowedOrigins,
	})
	if err != nil {
		return
	}

	ws.ServeLobby(r.Context(), sock, l, *client)
}

func (a *API) DefaultLobby(w http.ResponseWriter, r *http.Request) {
	client, err := a.authenticate(r)
	if err != nil {
		a.handleAuthError(w, err)
		return
	}

	sock, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: a.allowedOrigins,
	})
	if err != nil {
		return
	}

	ws.ServeLobby(r.Context(), sock, a.lobbyRegistry.DefaultLobby(), *client)
}

func (a *API) Room(w http.ResponseWriter, r *http.Request) {
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

	participant, ok := roomEntry.ParticipantByClientID(client.ID)
	if !ok {
		http.Error(w, "you are not a participant of this room", http.StatusForbidden)
		return
	}

	sock, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: a.allowedOrigins,
	})
	if err != nil {
		return
	}

	ws.ServeRoom(r.Context(), sock, roomEntry.Room, participant)
}

// pickBot returns the bot for the requested difficulty.
// Falls back to: requested → "hard" → "medium" → "easy" → any available.
func (a *API) pickBot(difficulty string) *bots.Bot {
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
