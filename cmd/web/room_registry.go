package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"tic-tac-chec/cmd/web/store"
	"tic-tac-chec/engine"
	"tic-tac-chec/internal/game"
)

type RoomRegistry interface {
	Create(pairing Pairing) RoomEntry
	CreateWithPlayers(p1, p2 game.Player, clients [2]ClientID) RoomEntry
	Lookup(id game.RoomID) (RoomEntry, bool)
	Add(entry RoomEntry)
	Restore(ctx context.Context, id game.RoomID) (RoomEntry, error)
}

type Participant struct {
	ClientID ClientID
	PlayerID game.PlayerID
}

type RoomEntry struct {
	Room         *game.Room
	Participants [2]Participant
}

type botSpawner func(botID string) (game.Player, bool)

type roomRegistry struct {
	mu       sync.Mutex
	rooms    map[game.RoomID]RoomEntry
	games    *store.GameStore
	players  *store.PlayerStore
	spawnBot botSpawner
}

func NewRoomRegistry(games *store.GameStore, players *store.PlayerStore, spawnBot botSpawner) *roomRegistry {
	return &roomRegistry{rooms: make(map[game.RoomID]RoomEntry), games: games, players: players, spawnBot: spawnBot}
}

func (rr *roomRegistry) Create(pairing Pairing) RoomEntry {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	p1 := game.NewPlayerWithID(make(chan game.Command), pairing.Players[0].PlayerID)
	p2 := game.NewPlayerWithID(make(chan game.Command), pairing.Players[1].PlayerID)
	room := game.NewRoom(p1, p2)

	entry := RoomEntry{
		Room: room,
		Participants: [2]Participant{
			Participant{ClientID: pairing.Players[0].ID, PlayerID: p1.ID},
			Participant{ClientID: pairing.Players[1].ID, PlayerID: p2.ID},
		},
	}

	rr.rooms[entry.Room.ID] = entry

	return entry
}

func (rr *roomRegistry) CreateWithPlayers(p1, p2 game.Player, clients [2]ClientID) RoomEntry {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	entry := RoomEntry{
		Room: game.NewRoom(p1, p2),
		Participants: [2]Participant{
			{ClientID: clients[0], PlayerID: p1.ID},
			{ClientID: clients[1], PlayerID: p2.ID},
		},
	}

	rr.rooms[entry.Room.ID] = entry

	return entry
}

func (rr *roomRegistry) Add(entry RoomEntry) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	rr.rooms[entry.Room.ID] = entry
}

func (rr *roomRegistry) Lookup(id game.RoomID) (RoomEntry, bool) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	entry, exists := rr.rooms[id]
	return entry, exists
}

func (rr *roomRegistry) Restore(ctx context.Context, roomId game.RoomID) (RoomEntry, error) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	g, err := rr.games.LoadLatestByRoom(ctx, string(roomId))
	if err != nil {
		return RoomEntry{}, ErrRoomNotFound
	}

	whitePlayer, err := rr.players.Get(ctx, g.WhitePlayerID)
	if err != nil {
		return RoomEntry{}, ErrRoomNotFound
	}
	blackPlayer, err := rr.players.Get(ctx, g.BlackPlayerID)
	if err != nil {
		return RoomEntry{}, ErrRoomNotFound
	}

	gamePlayerWhite, clientWhite, err := rr.playerFor(whitePlayer)
	if err != nil {
		return RoomEntry{}, ErrRoomNotFound
	}
	gamePlayerBlack, clientBlack, err := rr.playerFor(blackPlayer)
	if err != nil {
		return RoomEntry{}, ErrRoomNotFound
	}

	var gameState engine.Game
	err = json.Unmarshal(g.State, &gameState)
	if err != nil {
		return RoomEntry{}, ErrRoomNotFound
	}

	room := game.NewRoom(gamePlayerWhite, gamePlayerBlack)
	room.ID = roomId
	room.GameID = game.GameID(g.ID)
	room.Game = &gameState

	entry := RoomEntry{
		Room: room,
		Participants: [2]Participant{
			{ClientID: clientWhite, PlayerID: gamePlayerWhite.ID},
			{ClientID: clientBlack, PlayerID: gamePlayerBlack.ID},
		},
	}

	rr.rooms[roomId] = entry

	return entry, nil
}

func (rr *roomRegistry) playerFor(p store.Player) (game.Player, ClientID, error) {
	switch {
	case p.BotID != nil:
		player, ok := rr.spawnBot(*p.BotID)
		if !ok {
			return game.Player{}, "", fmt.Errorf("bot %s is not available", *p.BotID)
		}

		return player, BotClientID, nil
	case p.UserID != nil:
		player := game.Player{
			ID:              game.PlayerID(p.ID),
			ConnectionState: game.Disconnected,
			// disconnected, will establish channels and set color on reconnect
			Commands: nil,
			Updates:  nil,
			Color:    engine.Color(0),
		}

		return player, ClientID(*p.UserID), nil
	default:
		return game.Player{}, "", fmt.Errorf("player neither bot nor user")
	}
}

func (re *RoomEntry) participantByClientID(clientID ClientID) (Participant, bool) {
	for _, participant := range re.Participants {
		if participant.ClientID == clientID {
			return participant, true
		}
	}
	return Participant{}, false
}
