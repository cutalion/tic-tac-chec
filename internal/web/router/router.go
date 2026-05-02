package router

import (
	"net/http"
	"tic-tac-chec/internal/web/api"
	"tic-tac-chec/internal/web/config"
)

func Router(a *api.API, cfg config.Config) http.Handler {
	mux := http.NewServeMux()
	registerStaticRoutes(mux, cfg)

	// api
	mux.HandleFunc("POST /api/clients", a.CreateClient)
	mux.HandleFunc("POST /api/lobbies", a.CreateLobby)
	mux.HandleFunc("POST /api/bot-game", a.BotGame)
	mux.HandleFunc("GET /api/me", a.Me)

	// ws
	mux.HandleFunc("GET /ws/lobby", a.DefaultLobby)
	mux.HandleFunc("GET /ws/lobby/{id}", a.Lobby)
	mux.HandleFunc("GET /ws/room/{id}", a.Room)

	handler := corsMiddleware(mux, cfg.Server.AllowedOrigins)
	return handler
}
