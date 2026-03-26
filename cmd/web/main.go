package main

import (
	"log"
	"net/http"
	"os"
)

var clients = NewClientService()

func main() {
	app := NewApp(clients)

	mux := http.NewServeMux()
	mux.Handle("GET /", indexHandler())
	mux.Handle("GET /lobby", indexHandler())
	mux.Handle("GET /lobby/{id}", indexHandler())
	mux.Handle("GET /room/{id}", indexHandler())
	mux.Handle("GET /app.js", staticHandler())
	mux.Handle("GET /style.css", staticHandler())

	// api
	mux.HandleFunc("POST /api/clients", app.CreateClient)
	mux.HandleFunc("POST /api/lobbies", app.CreateLobby)
	mux.HandleFunc("GET /api/me", app.Me)

	// ws
	mux.HandleFunc("GET /ws/lobby", app.DefaultLobby)
	mux.HandleFunc("GET /ws/lobby/{id}", app.Lobby)
	mux.HandleFunc("GET /ws/room/{id}", app.Room)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("Listening on :" + port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
