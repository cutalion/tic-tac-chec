package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"tic-tac-chec/engine"
	"tic-tac-chec/internal/game"
	"tic-tac-chec/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/muesli/termenv"
	"github.com/charmbracelet/wish/bubbletea"
)

var (
	lobby = make(chan playerConn)
)

type playerConn struct {
	player game.Player
	ready  chan engine.Color
}

func main() {
	// Force color support — the server process has no TTY, but SSH clients do.
	// Without this, lipgloss detects no TTY and strips all ANSI colors.
	lipgloss.SetColorProfile(termenv.ANSI256)

	port := os.Getenv("PORT")
	if port == "" {
		port = "2222"
	}

	s, err := wish.NewServer(
		wish.WithAddress(":"+port),
		wish.WithHostKeyPath(".ssh/host_key"),
		wish.WithMiddleware(
			bubbletea.Middleware(teaHandler),
		),
	)
	if err != nil {
		log.Fatal(err)
		return
	}
	log.Println("Listening on " + port)

	go lobbyLoop(lobby)

	go func() {
		log.Fatal(s.ListenAndServe())
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	// wait for Ctrl+C
	<-sig
	log.Println("Shutting down...")
	s.Shutdown(context.Background())
}

func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	moves := make(chan ui.MoveRequest)
	incoming := make(chan tea.Msg)

	player := game.Player{
		Moves:    moves,
		Incoming: incoming,
	}

	ready := make(chan engine.Color)
	lobby <- playerConn{player: player, ready: ready}
	color := <-ready

	model := ui.InitialModel()
	model.MyColor = color
	model.Mode = ui.ModeOnline
	model.Moves = moves
	model.Incoming = incoming

	return model, nil
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

		room := game.Room{
			Players: [2]game.Player{white.player, black.player},
			Game:    engine.NewGame(),
		}

		go room.Run()

		white.ready <- white.player.Color
		black.ready <- black.player.Color
	}
}
