package main

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"tic-tac-chec/internal/game"
)

type Participant struct {
	room      *game.Room
	playerIdx int // 0 or 1
	token     string
}

type Participants struct {
	mu      sync.Mutex
	entries map[string]*Participant
}

func NewParticipants() *Participants {
	return &Participants{
		entries: make(map[string]*Participant),
	}
}

func (p *Participants) register(room *game.Room, playerIdx int) (token string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for {
		token = newParticipantToken()
		if _, exists := p.entries[token]; !exists {
			break
		}
	}

	p.entries[token] = &Participant{room: room, playerIdx: playerIdx, token: token}

	return token
}

func (p *Participants) lookup(token string) (*Participant, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	participant, exists := p.entries[token]
	return participant, exists
}

func newParticipantToken() (token string) {
	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)
	token = base64.URLEncoding.EncodeToString(tokenBytes)

	return token
}
