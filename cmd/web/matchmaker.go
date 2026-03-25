package main

import (
	"errors"
	"sync"
)

type Pairing struct {
	Players [2]ClientID
}

type MatchResult struct {
	Pairing   Pairing
	RoomEntry RoomEntry
}

type waiter struct {
	clientID ClientID
	results  chan MatchResult
}

type Matchmaker interface {
	Join(clientID ClientID) (<-chan MatchResult, error)
	Leave(clientID ClientID)
}

type matchmaker struct {
	roomRegistry RoomRegistry
	waiter       *waiter
	mu           sync.Mutex
}

var (
	ErrAlreadyInMatchmaker = errors.New("you are already in the matchmaker")
)

func NewMatchmaker(roomRegistry RoomRegistry) *matchmaker {
	return &matchmaker{roomRegistry: roomRegistry}
}

func (mm *matchmaker) Join(clientID ClientID) (<-chan MatchResult, error) {
	// if there is no waiter, create a new one
	mm.mu.Lock()

	if mm.waiter == nil {
		waiter := &waiter{
			clientID: clientID,
			results:  make(chan MatchResult, 1),
		}

		mm.waiter = waiter
		mm.mu.Unlock()

		return waiter.results, nil
	}

	if mm.waiter.clientID == clientID {
		mm.mu.Unlock()
		return nil, ErrAlreadyInMatchmaker
	}

	waiter := mm.waiter
	mm.waiter = nil
	mm.mu.Unlock()

	// pairings1 and pairings2 are buffered channels,
	// so we can send the pairing to both without blocking
	results1 := waiter.results
	results2 := make(chan MatchResult, 1)

	roomEntry := mm.roomRegistry.Create(Pairing{Players: [2]ClientID{waiter.clientID, clientID}})
	go roomEntry.Room.Run()

	result := MatchResult{
		Pairing:   Pairing{Players: [2]ClientID{waiter.clientID, clientID}},
		RoomEntry: roomEntry,
	}

	results1 <- result
	results2 <- result

	return results2, nil
}

func (mm *matchmaker) Leave(clientID ClientID) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if mm.waiter == nil {
		return
	}

	if mm.waiter.clientID == clientID {
		mm.waiter = nil
	}
}
