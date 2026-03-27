package main

import (
	"errors"
	"log"
	"sync"
	"tic-tac-chec/internal/game"
)

type Pairing struct {
	Players [2]ClientID
}

type PairingResult struct {
	Pairing   Pairing
	RoomEntry RoomEntry
}

type completedPairing struct {
	Pairing Pairing
	RoomID  game.RoomID
}

type waiter struct {
	clientID ClientID
	results  chan PairingResult
}

type Lobby interface {
	Join(clientID ClientID) (<-chan PairingResult, error)
	Leave(clientID ClientID)
}

type LobbyID string

type lobby struct {
	ID           LobbyID
	roomRegistry RoomRegistry
	waiter       *waiter

	// persistent lobby persists after all players leave or both players joined
	// ephemeral lobby may be eventually removed by the server
	persistent bool
	completed  *completedPairing
	mu         sync.Mutex
}

const (
	PersistentLobby = true
	EphemeralLobby  = false
)

var (
	ErrLobbyIsFull  = errors.New("lobby is full")
	ErrRoomNotFound = errors.New("room not found")
)

func NewLobby(id LobbyID, roomRegistry RoomRegistry, persistent bool) *lobby {
	return &lobby{ID: id, roomRegistry: roomRegistry, persistent: persistent}
}

func (l *lobby) Join(clientID ClientID) (<-chan PairingResult, error) {
	// if there is no waiter, create a new one
	l.mu.Lock()
	defer l.mu.Unlock()

	log.Println("lobby join", clientID)

	if l.completed != nil {
		if clientID == l.completed.Pairing.Players[1] || clientID == l.completed.Pairing.Players[0] {
			roomEntry, ok := l.roomRegistry.Lookup(l.completed.RoomID)
			if !ok {
				return nil, ErrRoomNotFound
			}
			result := PairingResult{
				Pairing:   l.completed.Pairing,
				RoomEntry: roomEntry,
			}
			results := make(chan PairingResult, 1)
			results <- result
			return results, nil
		} else {
			return nil, ErrLobbyIsFull
		}
	}

	if l.waiter == nil {
		waiter := &waiter{
			clientID: clientID,
			results:  make(chan PairingResult, 1),
		}

		l.waiter = waiter

		return waiter.results, nil
	}

	// rejoin
	if l.waiter.clientID == clientID {
		close(l.waiter.results)

		waiter := &waiter{
			clientID: clientID,
			results:  make(chan PairingResult, 1),
		}

		l.waiter = waiter

		return waiter.results, nil
	}

	waiter := l.waiter
	l.waiter = nil

	// pairings1 and pairings2 are buffered channels,
	// so we can send the pairing to both without blocking
	results1 := waiter.results
	results2 := make(chan PairingResult, 1)

	roomEntry := l.roomRegistry.Create(Pairing{Players: [2]ClientID{waiter.clientID, clientID}})
	go roomEntry.Room.Run()

	result := PairingResult{
		Pairing:   Pairing{Players: [2]ClientID{waiter.clientID, clientID}},
		RoomEntry: roomEntry,
	}

	if !l.persistent {
		l.completed = &completedPairing{
			Pairing: Pairing{Players: [2]ClientID{waiter.clientID, clientID}},
			RoomID:  roomEntry.Room.ID,
		}
	}

	results1 <- result
	results2 <- result

	return results2, nil
}

func (l *lobby) Leave(clientID ClientID) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.waiter == nil {
		return
	}

	if l.waiter.clientID == clientID {
		l.waiter = nil
	}
}
