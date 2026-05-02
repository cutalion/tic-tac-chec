package app

import (
	"context"
	"net/http"
	"tic-tac-chec/internal/game"
	"tic-tac-chec/internal/web/api"
	"tic-tac-chec/internal/web/bots"
	"tic-tac-chec/internal/web/clients"
	"tic-tac-chec/internal/web/config"
	"tic-tac-chec/internal/web/lobby"
	store "tic-tac-chec/internal/web/persistence/sqlite"
	"tic-tac-chec/internal/web/room"
	"tic-tac-chec/internal/web/router"
	"tic-tac-chec/internal/web/server"
)

type App struct {
	db            *store.Store
	clients       clients.ClientService
	lobbyRegistry lobby.Registry
	roomRegistry  room.Registry
	bots          bots.Bots
	config        config.Config
	api           *api.API
}

func NewApp(ctx context.Context, db *store.Store, cfg config.Config) *App {
	bb := bots.Init(ctx, db, *cfg.Bots)
	spawnBot := func(botID string) (game.Player, bool) {
		for _, bot := range bb {
			if bot.Info.ID == botID {
				return bot.Model.RunPlayer(bot.Info.PlayerID), true
			}
		}
		return game.Player{}, false
	}

	roomRegistry := room.NewRegistry(db.Games(), db.Players(), spawnBot)
	lobbyRegistry := lobby.NewRegistry(roomRegistry, db.Games())
	clients := clients.NewService(db.Users())
	apy := api.NewAPI(clients, lobbyRegistry, roomRegistry, bb, db, cfg.Server.AllowedOrigins)

	app := &App{
		db:            db,
		config:        cfg,
		clients:       clients,
		lobbyRegistry: lobbyRegistry,
		roomRegistry:  roomRegistry,
		bots:          bb,
		api:           apy,
	}
	app.restoreActiveGames(ctx)
	return app
}

func (app *App) Run(ctx context.Context) error {
	r := router.Router(app.api, app.config)
	return server.Run(ctx, app.config.Server.Port, r)
}

func (app *App) Router() http.Handler {
	r := router.Router(app.api, app.config)
	return r
}

func (app *App) Clients() clients.ClientService {
	return app.clients
}

func (app *App) LobbyRegistry() lobby.Registry {
	return app.lobbyRegistry
}

func (app *App) RoomRegistry() room.Registry {
	return app.roomRegistry
}
