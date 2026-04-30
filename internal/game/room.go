package game

import (
	"errors"
	"log/slog"
	"sync"
	"tic-tac-chec/engine"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/bridges/otelslog"
)

var logger = otelslog.NewLogger("tic-tac-chec/internal/game")

const (
	Connected    = "connected"
	Disconnected = "disconnected"
)

type RoomID string
type PlayerID string
type GameID string

func (id RoomID) LogValue() slog.Value   { return slog.StringValue(string(id)) }
func (id PlayerID) LogValue() slog.Value { return slog.StringValue(string(id)) }
func (id GameID) LogValue() slog.Value   { return slog.StringValue(string(id)) }

type Player struct {
	ID              PlayerID
	Color           engine.Color
	Commands        <-chan Command
	Updates         chan Event
	ConnectionState string
}

type ReconnectInfo struct {
	PlayerID PlayerID
	Commands <-chan Command
	Updates  chan Event
}

type Room struct {
	ID                    RoomID
	GameID                GameID // current game id
	Players               [2]Player
	Game                  *engine.Game
	Quit                  chan struct{}
	Reconnect             chan ReconnectInfo
	WhiteRematchRequested bool
	BlackRematchRequested bool
	GameNumber            uint
	subscribers           map[chan<- RoomEvent]struct{}
	mu                    sync.RWMutex
}

var (
	ErrInvalidMove = errors.New("invalid move")
)

func NewPlayer(commands <-chan Command) Player {
	return Player{
		ID:       PlayerID(uuid.New().String()),
		Commands: commands,
		// Room writes, UI reads. Bidirectional on the struct so both sides
		// can access it. Buffered(1) to absorb timing gaps between UI Cmd dispatches.
		Updates:         make(chan Event, 1),
		ConnectionState: Connected,
	}
}

func NewPlayerWithID(commands <-chan Command, id string) Player {
	return Player{
		ID:       PlayerID(id),
		Commands: commands,
		// Room writes, UI reads. Bidirectional on the struct so both sides
		// can access it. Buffered(1) to absorb timing gaps between UI Cmd dispatches.
		Updates:         make(chan Event, 1),
		ConnectionState: Connected,
	}
}

func NewRoom(player1, player2 Player) *Room {
	player1.Color, player2.Color = engine.White, engine.Black

	roomId := uuid.Must(uuid.NewV7()).String()
	gameId := uuid.Must(uuid.NewV7()).String()

	room := &Room{
		ID:                    RoomID(roomId),
		GameID:                GameID(gameId),
		Game:                  engine.NewGame(),
		Players:               [2]Player{player1, player2},
		Quit:                  make(chan struct{}),
		Reconnect:             make(chan ReconnectInfo),
		WhiteRematchRequested: false,
		BlackRematchRequested: false,
		GameNumber:            1,
		subscribers:           make(map[chan<- RoomEvent]struct{}),
	}

	return room
}

func (r *Room) Run() {
	defer func() {
		r.close()
	}()

	r.emit(NewGameStarted(r.ID, r.GameID, *r.Game, r.GameNumber, r.Players[0].ID, r.Players[1].ID, time.Now()))

	// Before the game starts, send the paired event to each player.
	// Use sendUpdateTo so restored Players (Updates=nil) don't block Run.
	for _, player := range r.Players {
		sendUpdateTo(player, PairedEvent{PlayerID: player.ID, Color: player.Color})
	}

	for {
		if r.WhiteRematchRequested && r.BlackRematchRequested {
			r.startRematch()
		}

		select {
		case command, ok := <-r.white().Commands:
			if !ok {
				r.disconnect(r.white())
				sendUpdateTo(*r.black(), OpponentAwayEvent{PlayerID: r.white().ID})
				continue
			}

			switch command := command.(type) {
			case MoveCommand:
				r.handleMove(*r.white(), command)
			case RematchCommand:
				r.handleRematch(*r.white())
			case ReactionCommand:
				r.handleReaction(*r.white(), command)
			}

		case command, ok := <-r.black().Commands:
			if !ok {
				r.disconnect(r.black())
				sendUpdateTo(*r.white(), OpponentAwayEvent{PlayerID: r.black().ID})
				continue
			}

			switch command := command.(type) {
			case MoveCommand:
				r.handleMove(*r.black(), command)
			case RematchCommand:
				r.handleRematch(*r.black())
			case ReactionCommand:
				r.handleReaction(*r.black(), command)
			}

		case player, ok := <-r.Reconnect:
			if !ok {
				continue
			}

			if player.PlayerID == r.white().ID {
				r.reconnect(r.white(), player.Commands, player.Updates)
				sendUpdateTo(*r.white(), SnapshotEvent{RoomID: r.ID, Game: *r.Game})
				sendUpdateTo(*r.black(), OpponentReconnectedEvent{PlayerID: r.white().ID})
			} else if player.PlayerID == r.black().ID {
				r.reconnect(r.black(), player.Commands, player.Updates)
				sendUpdateTo(*r.black(), SnapshotEvent{RoomID: r.ID, Game: *r.Game})
				sendUpdateTo(*r.white(), OpponentReconnectedEvent{PlayerID: r.black().ID})
			} else {
				// ignore reconnect for unknown player
				continue
			}

		case <-r.Quit:
			// quit signal received, exit the loop
			return
		}
	}
}

func (r *Room) close() {
	r.emit(NewStateUpdate(r.ID, r.GameID, *r.Game, r.GameNumber, time.Now()))

	r.clearSubs()

	for _, player := range r.Players {
		if player.Updates != nil {
			close(player.Updates)
		}
	}
}

// subscriber must be a buffered channel
func (r *Room) Subscribe(subscriber chan<- RoomEvent) (cancel func()) {
	r.mu.Lock()
	r.subscribers[subscriber] = struct{}{}
	r.mu.Unlock()

	return func() {
		r.unsubscribe(subscriber)
	}
}

func (r *Room) unsubscribe(updates chan<- RoomEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.subscribers, updates)
}

func (r *Room) emit(event RoomEvent) {
	for _, subscriber := range r.subs() {
		select {
		case subscriber <- event:
		default:
			// subscriber is full, skip
		}
	}
}

func (r *Room) subs() []chan<- RoomEvent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	subs := make([]chan<- RoomEvent, 0, len(r.subscribers))
	for subscriber := range r.subscribers {
		subs = append(subs, subscriber)
	}

	return subs
}

func (r *Room) clearSubs() {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, subscriber := range r.subs() {
		close(subscriber)
	}

	clear(r.subscribers)
}

func (r *Room) handleMove(mover Player, move MoveCommand) {
	if mover.Color != move.Piece.Color {
		sendUpdateTo(mover, ErrorEvent{Error: ErrInvalidMove})
		return
	}

	err := r.Game.Move(move.Piece, move.To)
	if err != nil {
		sendUpdateTo(mover, ErrorEvent{Error: err})
		return
	}

	now := time.Now()
	r.emit(NewMoveApplied(r.ID, mover.ID, move.Piece, move.To, r.Game.MoveCount, r.GameNumber, now))
	r.emit(NewStateUpdate(r.ID, r.GameID, *r.Game, r.GameNumber, now))

	for _, player := range r.Players {
		sendUpdateTo(player, SnapshotEvent{RoomID: r.ID, Game: *r.Game})
	}
}

func (r *Room) handleRematch(mover Player) {
	switch mover.Color {
	case engine.White:
		r.WhiteRematchRequested = true
		if !r.BlackRematchRequested {
			sendUpdateTo(*r.black(), RematchRequestedEvent{PlayerID: r.white().ID})
		}
	case engine.Black:
		r.BlackRematchRequested = true
		if !r.WhiteRematchRequested {
			sendUpdateTo(*r.white(), RematchRequestedEvent{PlayerID: r.black().ID})
		}
	default:
		panic("invalid color")
	}
}

func (r *Room) handleReaction(mover Player, reaction ReactionCommand) {
	if !ValidReaction(reaction.Reaction) {
		sendUpdateTo(mover, ErrorEvent{Error: errors.New("invalid reaction")})
		return
	}
	sendUpdateTo(*r.black(), ReactionEvent{Reaction: reaction.Reaction, PlayerID: mover.ID})
	sendUpdateTo(*r.white(), ReactionEvent{Reaction: reaction.Reaction, PlayerID: mover.ID})
}

func (r *Room) startRematch() {
	r.mu.Lock()

	gameId := uuid.Must(uuid.NewV7()).String()
	r.GameID = GameID(gameId)
	r.Game = engine.NewGame()
	r.WhiteRematchRequested = false
	r.BlackRematchRequested = false
	r.GameNumber++

	// swap colors
	r.Players[0].Color, r.Players[1].Color = r.Players[1].Color, r.Players[0].Color

	gameSnapshot := *r.Game
	gameNumber := r.GameNumber
	players := r.Players
	whiteID := r.white().ID
	blackID := r.black().ID

	r.mu.Unlock()

	now := time.Now().UTC()
	r.emit(NewGameStarted(r.ID, r.GameID, gameSnapshot, gameNumber, whiteID, blackID, now))
	r.emit(NewStateUpdate(r.ID, r.GameID, gameSnapshot, gameNumber, now))

	for _, p := range players {
		sendUpdateTo(p, PairedEvent{PlayerID: p.ID, Color: p.Color})
		sendUpdateTo(p, SnapshotEvent{RoomID: r.ID, Game: gameSnapshot})
	}
}

func (r *Room) disconnect(p *Player) {
	r.mu.Lock()
	defer r.mu.Unlock()

	p.ConnectionState = Disconnected

	if p.Updates != nil {
		close(p.Updates)
	}

	p.Commands = nil
	p.Updates = nil
}

func (r *Room) reconnect(p *Player, commands <-chan Command, updates chan Event) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if p.Updates != nil {
		close(p.Updates)
	}

	p.ConnectionState = Connected
	p.Commands = commands // TODO: should we close the old commands channel?
	p.Updates = updates
}

func (r *Room) PlayerColor(playerID PlayerID) engine.Color {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for i := range r.Players {
		if r.Players[i].ID == playerID {
			return r.Players[i].Color
		}
	}

	panic("player not found")
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

func sendUpdateTo(player Player, msg any) {
	if player.Updates == nil {
		return
	}

	// non-blocking send - if nobody listens, the message is dropped
	// otherwise it would block the sender until the message is consumed
	select {
	case player.Updates <- msg:
	default: // skip if nobody listens
		logger.Warn("room.message_dropped", "msg", msg)
	}
}
