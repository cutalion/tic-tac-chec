package room

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"tic-tac-chec/engine"
	"tic-tac-chec/internal/game"
	"tic-tac-chec/internal/web/clients"
	store "tic-tac-chec/internal/web/persistence/sqlite"
)

var ErrRoomNotFound = errors.New("room not found")

type Registry interface {
	Create(pairing Pairing) Entry
	CreateWithPlayers(p1, p2 game.Player, clients [2]clients.ClientID) Entry
	Lookup(id game.RoomID) (Entry, bool)
	Add(entry Entry)
	Restore(ctx context.Context, id game.RoomID) (Entry, error)
}

type Participant struct {
	ClientID clients.ClientID
	PlayerID game.PlayerID
}

type Entry struct {
	Room         *game.Room
	Participants [2]Participant
}

type botSpawner func(botID string) (game.Player, bool)

type registry struct {
	mu       sync.Mutex
	rooms    map[game.RoomID]Entry
	games    *store.GameStore
	players  *store.PlayerStore
	spawnBot botSpawner
}

func NewRegistry(games *store.GameStore, players *store.PlayerStore, spawnBot botSpawner) *registry {
	return &registry{rooms: make(map[game.RoomID]Entry), games: games, players: players, spawnBot: spawnBot}
}

func (rr *registry) Create(pairing Pairing) Entry {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	p1 := game.NewPlayerWithID(make(chan game.Command), pairing.Players[0].PlayerID)
	p2 := game.NewPlayerWithID(make(chan game.Command), pairing.Players[1].PlayerID)
	room := game.NewRoom(p1, p2)

	entry := Entry{
		Room: room,
		Participants: [2]Participant{
			Participant{ClientID: pairing.Players[0].ID, PlayerID: p1.ID},
			Participant{ClientID: pairing.Players[1].ID, PlayerID: p2.ID},
		},
	}

	rr.rooms[entry.Room.ID] = entry

	return entry
}

func (rr *registry) CreateWithPlayers(p1, p2 game.Player, clients [2]clients.ClientID) Entry {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	entry := Entry{
		Room: game.NewRoom(p1, p2),
		Participants: [2]Participant{
			{ClientID: clients[0], PlayerID: p1.ID},
			{ClientID: clients[1], PlayerID: p2.ID},
		},
	}

	rr.rooms[entry.Room.ID] = entry

	return entry
}

func (rr *registry) Add(entry Entry) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	rr.rooms[entry.Room.ID] = entry
}

func (rr *registry) Lookup(id game.RoomID) (Entry, bool) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	entry, exists := rr.rooms[id]
	return entry, exists
}

func (rr *registry) Restore(ctx context.Context, roomId game.RoomID) (Entry, error) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	g, err := rr.games.LoadLatestByRoom(ctx, string(roomId))
	if err != nil {
		return Entry{}, ErrRoomNotFound
	}

	whitePlayer, err := rr.players.Get(ctx, g.WhitePlayerID)
	if err != nil {
		return Entry{}, ErrRoomNotFound
	}
	blackPlayer, err := rr.players.Get(ctx, g.BlackPlayerID)
	if err != nil {
		return Entry{}, ErrRoomNotFound
	}

	gamePlayerWhite, clientWhite, err := rr.playerFor(whitePlayer)
	if err != nil {
		return Entry{}, ErrRoomNotFound
	}
	gamePlayerBlack, clientBlack, err := rr.playerFor(blackPlayer)
	if err != nil {
		return Entry{}, ErrRoomNotFound
	}

	var gameState engine.Game
	err = json.Unmarshal(g.State, &gameState)
	if err != nil {
		return Entry{}, ErrRoomNotFound
	}

	room := game.NewRoom(gamePlayerWhite, gamePlayerBlack)
	room.ID = roomId
	room.GameID = game.GameID(g.ID)
	room.Game = &gameState

	entry := Entry{
		Room: room,
		Participants: [2]Participant{
			{ClientID: clientWhite, PlayerID: gamePlayerWhite.ID},
			{ClientID: clientBlack, PlayerID: gamePlayerBlack.ID},
		},
	}

	rr.rooms[roomId] = entry

	return entry, nil
}

func (re *Entry) ParticipantByClientID(clientID clients.ClientID) (Participant, bool) {
	for _, participant := range re.Participants {
		if participant.ClientID == clientID {
			return participant, true
		}
	}
	return Participant{}, false
}

func (rr *registry) playerFor(p store.Player) (game.Player, clients.ClientID, error) {
	switch {
	case p.BotID != nil:
		player, ok := rr.spawnBot(*p.BotID)
		if !ok {
			return game.Player{}, "", fmt.Errorf("bot %s is not available", *p.BotID)
		}

		return player, clients.BotClientID, nil
	case p.UserID != nil:
		player := game.Player{
			ID:              game.PlayerID(p.ID),
			ConnectionState: game.Disconnected,
			// disconnected, will establish channels and set color on reconnect
			Commands: nil,
			Updates:  nil,
			Color:    engine.Color(0),
		}

		return player, clients.ClientID(*p.UserID), nil
	default:
		return game.Player{}, "", fmt.Errorf("player neither bot nor user")
	}
}
