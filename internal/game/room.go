package game

import (
	"tic-tac-chec/engine"
	"tic-tac-chec/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

type Player struct {
	Color    engine.Color
	Moves    <-chan ui.MoveRequest
	Incoming chan tea.Msg
}

type Room struct {
	Players [2]Player
	Game    *engine.Game
}

func NewPlayer(moves <-chan ui.MoveRequest) Player {
	return Player{
		Moves: moves,
		// Room writes, UI reads. Bidirectional on the struct so both sides
		// can access it. Buffered(1) to absorb timing gaps between UI Cmd dispatches.
		Incoming: make(chan tea.Msg, 1),
	}
}

func NewRoom(white, black Player) Room {
	return Room{
		Game:    engine.NewGame(),
		Players: [2]Player{white, black},
	}
}

func (r *Room) Run() {
	defer close(r.Players[0].Incoming)
	defer close(r.Players[1].Incoming)

	white := r.Players[0]
	black := r.Players[1]

	for {
		select {
		case move, ok := <-white.Moves:
			if !ok {
				select {
				case black.Incoming <- ui.OpponentDisconnectedMsg{}: // sent if black still listening
				default: // skip if nobody listens
				}
				return
			}

			if stop := r.handleMove(white, move); stop {
				return
			}
		case move, ok := <-black.Moves:
			if !ok {
				select {
				case white.Incoming <- ui.OpponentDisconnectedMsg{}:
				default: // skip
				}
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
