package main

import (
	"sync"
	"tic-tac-chec/engine"
	"tic-tac-chec/internal/game"
)

type RoomRegistry interface {
	Create(pairing Pairing) RoomEntry
	Lookup(id game.RoomID) (RoomEntry, bool)
}

type Participant struct {
	ClientID ClientID
	PlayerID game.PlayerID
}

type RoomEntry struct {
	Room         *game.Room
	Participants [2]Participant
}

type roomRegistry struct {
	mu    sync.Mutex
	rooms map[game.RoomID]RoomEntry
}

func NewRoomRegistry() *roomRegistry {
	return &roomRegistry{rooms: make(map[game.RoomID]RoomEntry)}
}

func (rr *roomRegistry) Create(pairing Pairing) RoomEntry {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	p1 := game.NewPlayer(make(chan game.Command))
	p1.Color = engine.White // TODO: let the room set the color before game start

	p2 := game.NewPlayer(make(chan game.Command))
	p2.Color = engine.Black
	room := game.NewRoom(p1, p2)

	entry := RoomEntry{
		Room: room,
		Participants: [2]Participant{
			Participant{ClientID: pairing.Players[0], PlayerID: p1.ID},
			Participant{ClientID: pairing.Players[1], PlayerID: p2.ID},
		},
	}

	rr.rooms[entry.Room.ID] = entry

	return entry
}

func (rr *roomRegistry) Lookup(id game.RoomID) (RoomEntry, bool) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	entry, exists := rr.rooms[id]
	return entry, exists
}

func (re *RoomEntry) participantByClientID(clientID ClientID) (Participant, bool) {
	for _, participant := range re.Participants {
		if participant.ClientID == clientID {
			return participant, true
		}
	}
	return Participant{}, false
}
