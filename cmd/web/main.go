package main

import (
	"errors"
	"log"
	"net/http"
	"os"
)

var (
	ErrUnsupportedMessageType = errors.New("unsupported message type")
)

var clients = NewClientService()

func main() {
	app := NewApp(clients)

	mux := http.NewServeMux()
	mux.Handle("GET /", indexHandler())
	mux.Handle("GET /lobby", indexHandler())
	mux.Handle("GET /room/{id}", indexHandler())
	mux.Handle("GET /app.js", staticHandler())
	mux.Handle("GET /style.css", staticHandler())

	// api
	mux.HandleFunc("POST /api/clients", app.CreateClient)
	mux.HandleFunc("GET /api/me", app.Me)

	// ws
	mux.HandleFunc("GET /ws/lobby", app.Lobby)
	mux.HandleFunc("GET /ws/room/{id}", app.Room)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("Listening on :" + port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
