package main

import (
	"sync"

	"tic-tac-chec/cmd/web/store"

	"github.com/google/uuid"
)

type LobbyRegistry interface {
	DefaultLobby() *lobby
	Create() *lobby
	Find(id LobbyID) *lobby
}

type lobbyRegistry struct {
	mu           sync.Mutex
	lobbies      map[LobbyID]*lobby
	roomRegistry RoomRegistry
	games        *store.GameStore
}

var (
	DefaultLobbyID LobbyID = "default"
)

func NewLobbyRegistry(roomRegistry RoomRegistry, games *store.GameStore) *lobbyRegistry {
	reg := lobbyRegistry{
		lobbies:      make(map[LobbyID]*lobby),
		roomRegistry: roomRegistry,
		games:        games,
	}

	reg.createDefaultLobby()

	return &reg
}

func (lr *lobbyRegistry) DefaultLobby() *lobby {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	return lr.lobbies[DefaultLobbyID]
}

func (lr *lobbyRegistry) Find(id LobbyID) *lobby {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	lobby, exists := lr.lobbies[id]
	if !exists {
		return nil
	}

	return lobby
}

func (lr *lobbyRegistry) Create() *lobby {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	id := lr.generateLobbyID()
	if lobby, exists := lr.lobbies[id]; exists {
		return lobby
	}

	lobby := NewLobby(id, lr.roomRegistry, lr.games, EphemeralLobby)
	lr.lobbies[id] = lobby
	return lobby
}

func (lr *lobbyRegistry) createDefaultLobby() {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	if _, exists := lr.lobbies[DefaultLobbyID]; exists {
		return
	}

	lobby := NewLobby(DefaultLobbyID, lr.roomRegistry, lr.games, PersistentLobby)
	lr.lobbies[DefaultLobbyID] = lobby
}

func (lr *lobbyRegistry) generateLobbyID() LobbyID {
	for {
		id := LobbyID(uuid.New().String())
		if _, exists := lr.lobbies[id]; !exists {
			return id
		}
	}
}
