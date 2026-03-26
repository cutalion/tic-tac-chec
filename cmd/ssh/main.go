package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"tic-tac-chec/internal/game"
	"tic-tac-chec/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/muesli/termenv"
)

var (
	lobby = make(chan playerConn)
)

type playerConn struct {
	player game.Player
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
		hostKeyOption(),
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
	commands := make(chan game.Command)

	// when player quits/disconnects, close its channel
	// so that other player/session can catch it
	// and send ui.OpponentDisconnectedMsg
	go func(s ssh.Session) {
		<-s.Context().Done()
		close(commands)
	}(s)

	player := game.NewPlayer(commands)

	model := ui.InitialModel()
	model.Mode = ui.ModeOnline
	model.Phase = ui.PhaseWaiting
	model.Commands = commands
	model.Updates = player.Updates

	go func() {
		lobby <- playerConn{player: player}
	}()

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

		room := game.NewRoom(white.player, black.player)
		go room.Run()
	}
}

func hostKeyOption() ssh.Option {
	if pem := os.Getenv("HOST_KEY_PEM"); pem != "" {
		return wish.WithHostKeyPEM([]byte(pem))
	}

	return wish.WithHostKeyPath(".ssh/host_key")
}
