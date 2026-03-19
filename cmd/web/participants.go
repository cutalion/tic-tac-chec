package main

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"tic-tac-chec/engine"
	"tic-tac-chec/internal/game"
)

type Participant struct {
	room  *game.Room
	color engine.Color
	token string
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

func (p *Participants) register(room *game.Room, color engine.Color) (token string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for {
		token = p.generateToken()
		if _, exists := p.entries[token]; !exists {
			break
		}
	}

	p.entries[token] = &Participant{room: room, color: color}

	return token
}

func (p *Participants) lookup(token string) (*Participant, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	participant, exists := p.entries[token]
	return participant, exists
}

func (p *Participants) generateToken() (token string) {
	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)
	token = base64.URLEncoding.EncodeToString(tokenBytes)

	return token
}
