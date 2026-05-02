package lobby

import (
	"sync"

	store "tic-tac-chec/internal/web/persistence/sqlite"
	"tic-tac-chec/internal/web/room"

	"github.com/google/uuid"
)

type Registry interface {
	DefaultLobby() *Lobby
	Create() *Lobby
	Find(id LobbyID) *Lobby
}

type registry struct {
	mu           sync.Mutex
	lobbies      map[LobbyID]*Lobby
	roomRegistry room.Registry
	games        *store.GameStore
}

var (
	DefaultLobbyID LobbyID = "default"
)

func NewRegistry(roomRegistry room.Registry, games *store.GameStore) *registry {
	reg := registry{
		lobbies:      make(map[LobbyID]*Lobby),
		roomRegistry: roomRegistry,
		games:        games,
	}

	reg.createDefaultLobby()

	return &reg
}

func (r *registry) DefaultLobby() *Lobby {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.lobbies[DefaultLobbyID]
}

func (r *registry) Find(id LobbyID) *Lobby {
	r.mu.Lock()
	defer r.mu.Unlock()

	lobby, exists := r.lobbies[id]
	if !exists {
		return nil
	}

	return lobby
}

func (r *registry) Create() *Lobby {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := r.generateLobbyID()
	if lobby, exists := r.lobbies[id]; exists {
		return lobby
	}

	lobby := NewLobby(id, r.roomRegistry, r.games, EphemeralLobby)
	r.lobbies[id] = lobby
	return lobby
}

func (r *registry) createDefaultLobby() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.lobbies[DefaultLobbyID]; exists {
		return
	}

	lobby := NewLobby(DefaultLobbyID, r.roomRegistry, r.games, PersistentLobby)
	r.lobbies[DefaultLobbyID] = lobby
}

func (r *registry) generateLobbyID() LobbyID {
	for {
		id := LobbyID(uuid.New().String())
		if _, exists := r.lobbies[id]; !exists {
			return id
		}
	}
}
