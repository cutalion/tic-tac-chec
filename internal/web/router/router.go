package router

import (
	"net/http"
	"tic-tac-chec/internal/web/api"
	"tic-tac-chec/internal/web/config"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func New(a *api.API, cfg config.Config) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestLogger(RedactingLogFormatter))
	r.Use(corsMiddleware(*cfg.Server))

	r.Route("/api", func(r chi.Router) {
		r.Post("/clients", a.CreateClient)
		r.Post("/lobbies", a.CreateLobby)
		r.Post("/bot-game", a.BotGame)
		r.Get("/me", a.Me)
	})

	r.Route("/ws", func(r chi.Router) {
		r.Get("/lobby", a.DefaultLobby)
		r.Get("/lobby/{id}", a.Lobby)
		r.Get("/room/{id}", a.Room)
	})

	registerStaticRoutes(r, cfg)

	return r
}
