package main

import (
	"errors"
	"log"
	"sync"
)

type Pairing struct {
	Players [2]ClientID
}

type PairingResult struct {
	Pairing   Pairing
	RoomEntry RoomEntry
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
	mu           sync.Mutex
}

var (
	ErrAlreadyInLobby = errors.New("you are already in the lobby")
)

func NewLobby(id LobbyID, roomRegistry RoomRegistry) *lobby {
	return &lobby{ID: id, roomRegistry: roomRegistry}
}

func (l *lobby) Join(clientID ClientID) (<-chan PairingResult, error) {
	// if there is no waiter, create a new one
	l.mu.Lock()

	log.Println("lobby join", clientID)

	if l.waiter == nil {
		waiter := &waiter{
			clientID: clientID,
			results:  make(chan PairingResult, 1),
		}

		l.waiter = waiter
		l.mu.Unlock()

		return waiter.results, nil
	}

	if l.waiter.clientID == clientID {
		l.mu.Unlock()
		return nil, ErrAlreadyInLobby
	}

	waiter := l.waiter
	l.waiter = nil
	l.mu.Unlock()

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
