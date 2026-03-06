package game

import (
	"tic-tac-chec/engine"
	"tic-tac-chec/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

type Player struct {
	Color    engine.Color
	Moves    <-chan ui.MoveRequest
	Incoming chan<- tea.Msg
}

type Room struct {
	Players [2]Player
	Game    *engine.Game
}

func (r *Room) Run() {
	white := r.Players[0]
	black := r.Players[1]

	for {
		select {
		case move, ok := <-white.Moves:
			if !ok {
				black.Incoming <- tea.Msg(ui.OpponentDisconnectedMsg{})
				return
			}

			if stop := r.handleMove(white, move); stop {
				return
			}
		case move, ok := <-black.Moves:
			if !ok {
				white.Incoming <- tea.Msg(ui.OpponentDisconnectedMsg{})
				return
			}

			if stop := r.handleMove(black, move); stop {
				return
			}
		}
	}
}

// handleMove processes a move from mover. Returns true if the game should stop.
func (r *Room) handleMove(mover Player, move ui.MoveRequest) (stop bool) {
	err := r.Game.Move(move.Piece, move.Cell)
	if err != nil {
		mover.Incoming <- tea.Msg(ui.ErrorMsg{Err: err})
		return false
	}

	for _, player := range r.Players {
		player.Incoming <- tea.Msg(ui.GameStateMsg{Game: *r.Game})
	}

	return r.Game.Status == engine.GameOver
}
