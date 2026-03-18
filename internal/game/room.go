package game

import (
	"errors"
	"tic-tac-chec/engine"
	"tic-tac-chec/internal/ui"
)

const (
	Connected    = "connected"
	Disconnected = "disconnected"
)

type Player struct {
	Color           engine.Color
	Moves           <-chan ui.MoveRequest
	Updates         chan any
	ConnectionState string
}

type Room struct {
	Players   [2]Player
	Game      *engine.Game
	Quit      chan struct{}
	Reconnect chan Player
}

var (
	ErrInvalidMove = errors.New("invalid move")
)

func NewPlayer(moves <-chan ui.MoveRequest) Player {
	return Player{
		Moves: moves,
		// Room writes, UI reads. Bidirectional on the struct so both sides
		// can access it. Buffered(1) to absorb timing gaps between UI Cmd dispatches.
		Updates:         make(chan any, 1),
		ConnectionState: Connected,
	}
}

func NewRoom(white, black Player) Room {
	return Room{
		Game:      engine.NewGame(),
		Players:   [2]Player{white, black},
		Quit:      make(chan struct{}),
		Reconnect: make(chan Player),
	}
}

func (r *Room) Run() {
	defer func() {
		for _, player := range r.Players {
			if player.Updates != nil {
				close(player.Updates)
			}
		}
	}()

	white := &r.Players[0]
	black := &r.Players[1]

	for _, player := range r.Players {
		player.Updates <- ui.GameStateMsg{Game: *r.Game}
	}

	for {
		select {
		case move, ok := <-white.Moves:
			if !ok {
				white.disconnect()
				sendUpdateTo(*black, ui.OpponentAwayMsg{})
				continue
			}

			if stop := r.handleMove(*white, move); stop {
				return
			}
		case move, ok := <-black.Moves:
			if !ok {
				black.disconnect()
				sendUpdateTo(*white, ui.OpponentAwayMsg{})
				continue
			}

			if stop := r.handleMove(*black, move); stop {
				return
			}

		case player, ok := <-r.Reconnect:
			if !ok {
				continue
			}

			if player.Color == engine.White {
				white.reconnect(player.Moves, player.Updates)
				sendUpdateTo(*black, ui.OpponentReconnectedMsg{})
			} else {
				black.reconnect(player.Moves, player.Updates)
				sendUpdateTo(*white, ui.OpponentReconnectedMsg{})
			}

			sendUpdateTo(player, ui.GameStateMsg{Game: *r.Game})

		case <-r.Quit:
			// quit signal received, exit the loop
			return
		}
	}
}

// handleMove processes a move from mover. Returns true if the game should stop.
func (r *Room) handleMove(mover Player, move ui.MoveRequest) (stop bool) {
	if mover.Color != move.Piece.Color {
		sendUpdateTo(mover, ui.ErrorMsg{Err: ErrInvalidMove})
		return false
	}

	err := r.Game.Move(move.Piece, move.Cell)
	if err != nil {
		sendUpdateTo(mover, ui.ErrorMsg{Err: err})
		return false
	}

	for _, player := range r.Players {
		sendUpdateTo(player, ui.GameStateMsg{Game: *r.Game})
	}

	return r.Game.Status == engine.GameOver
}

func sendUpdateTo(player Player, msg any) {
	if player.Updates == nil {
		return
	}

	// non-blocking send - if nobody listens, the message is dropped
	// otherwise it would block the sender until the message is consumed
	select {
	case player.Updates <- msg:
	default: // skip if nobody listens
	}
}

func (p *Player) disconnect() {
	p.ConnectionState = Disconnected
	p.Moves = nil
	p.Updates = nil
}

func (p *Player) reconnect(moves <-chan ui.MoveRequest, updates chan any) {
	if p.Updates != nil {
		close(p.Updates)
	}

	p.ConnectionState = Connected
	p.Moves = moves
	p.Updates = updates
}
