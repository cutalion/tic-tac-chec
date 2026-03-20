package game

import (
	"errors"
	"log"
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
	Rematch         <-chan ui.RematchRequest
	ConnectionState string
}

type Room struct {
	Players               [2]Player
	Game                  *engine.Game
	Quit                  chan struct{}
	Reconnect             chan Player
	WhiteRematchRequested bool
	BlackRematchRequested bool
}

var (
	ErrInvalidMove = errors.New("invalid move")
)

func NewPlayer(moves <-chan ui.MoveRequest, rematch <-chan ui.RematchRequest) Player {
	return Player{
		Moves:   moves,
		Rematch: rematch,
		// Room writes, UI reads. Bidirectional on the struct so both sides
		// can access it. Buffered(1) to absorb timing gaps between UI Cmd dispatches.
		Updates:         make(chan any, 1),
		ConnectionState: Connected,
	}
}

func NewRoom(white, black Player) Room {
	return Room{
		Game:                  engine.NewGame(),
		Players:               [2]Player{white, black},
		Quit:                  make(chan struct{}),
		Reconnect:             make(chan Player),
		WhiteRematchRequested: false,
		BlackRematchRequested: false,
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

	for _, player := range r.Players {
		player.Updates <- ui.GameStateMsg{Game: *r.Game}
	}

	for {
		if r.WhiteRematchRequested && r.BlackRematchRequested {
			r.startRematch()
		}

		select {
		case move, ok := <-r.white().Moves:
			if !ok {
				r.white().disconnect()
				sendUpdateTo(*r.black(), ui.OpponentAwayMsg{})
				continue
			}

			r.handleMove(*r.white(), move)

		case move, ok := <-r.black().Moves:
			if !ok {
				r.black().disconnect()
				sendUpdateTo(*r.white(), ui.OpponentAwayMsg{})
				continue
			}

			r.handleMove(*r.black(), move)

		case <-r.white().Rematch:
			r.WhiteRematchRequested = true
			if !r.BlackRematchRequested {
				sendUpdateTo(*r.black(), ui.RematchRequestedMsg{})
			}
		case <-r.black().Rematch:
			r.BlackRematchRequested = true
			if !r.WhiteRematchRequested {
				sendUpdateTo(*r.white(), ui.RematchRequestedMsg{})
			}
		case player, ok := <-r.Reconnect:
			if !ok {
				continue
			}

			if player.Color == engine.White {
				r.white().reconnect(player.Moves, player.Rematch, player.Updates)
				sendUpdateTo(*r.black(), ui.OpponentReconnectedMsg{})
			} else {
				r.black().reconnect(player.Moves, player.Rematch, player.Updates)
				sendUpdateTo(*r.white(), ui.OpponentReconnectedMsg{})
			}

			sendUpdateTo(player, ui.GameStateMsg{Game: *r.Game})

		case <-r.Quit:
			// quit signal received, exit the loop
			return
		}
	}
}

func (r *Room) handleMove(mover Player, move ui.MoveRequest) {
	if mover.Color != move.Piece.Color {
		sendUpdateTo(mover, ui.ErrorMsg{Err: ErrInvalidMove})
		return
	}

	err := r.Game.Move(move.Piece, move.Cell)
	if err != nil {
		sendUpdateTo(mover, ui.ErrorMsg{Err: err})
		return
	}

	for _, player := range r.Players {
		sendUpdateTo(player, ui.GameStateMsg{Game: *r.Game})
	}
}

func (r *Room) startRematch() {
	r.Game = engine.NewGame()
	r.WhiteRematchRequested = false
	r.BlackRematchRequested = false

	// swap colors, keep players at their original indices (matching participant tokens)
	r.Players[0].Color, r.Players[1].Color = r.Players[1].Color, r.Players[0].Color

	for _, player := range r.Players {
		sendUpdateTo(player, ui.PairedMsg{Color: player.Color})
		sendUpdateTo(player, ui.GameStateMsg{Game: *r.Game})
	}
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
		log.Printf("dropping message: %v", msg)
	}
}

func (p *Player) disconnect() {
	p.ConnectionState = Disconnected
	p.Moves = nil
	p.Rematch = nil
	p.Updates = nil
}

func (p *Player) reconnect(moves <-chan ui.MoveRequest, rematch <-chan ui.RematchRequest, updates chan any) {
	if p.Updates != nil {
		close(p.Updates)
	}

	p.ConnectionState = Connected
	p.Moves = moves
	p.Rematch = rematch
	p.Updates = updates
}

func (r *Room) white() *Player {
	if r.Players[0].Color == engine.White {
		return &r.Players[0]
	}
	return &r.Players[1]
}

func (r *Room) black() *Player {
	if r.Players[0].Color == engine.Black {
		return &r.Players[0]
	}
	return &r.Players[1]
}
