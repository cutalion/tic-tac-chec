package api

import (
	"tic-tac-chec/internal/web/bots"
	"tic-tac-chec/internal/web/clients"
	"tic-tac-chec/internal/web/lobby"
	store "tic-tac-chec/internal/web/persistence/sqlite"
	"tic-tac-chec/internal/web/room"
)

type API struct {
	clients        clients.ClientService
	lobbyRegistry  lobby.Registry
	roomRegistry   room.Registry
	bots           bots.Bots
	db             *store.Store
	allowedOrigins []string
}

func NewAPI(clients clients.ClientService, lobbyRegistry lobby.Registry, roomRegistry room.Registry, bots bots.Bots, db *store.Store, allowedOrigins []string) *API {
	return &API{
		clients:        clients,
		lobbyRegistry:  lobbyRegistry,
		roomRegistry:   roomRegistry,
		bots:           bots,
		db:             db,
		allowedOrigins: allowedOrigins,
	}
}
