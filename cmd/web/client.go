package main

import (
	"sync"

	"github.com/google/uuid"
)

type Client struct {
	ID ClientID
}

type ClientID string

type ClientService interface {
	Create() *Client
	lookup(id ClientID) (*Client, bool)
}

type clientService struct {
	mu      sync.Mutex
	clients map[ClientID]*Client
}

func NewClientService() ClientService {
	return &clientService{
		clients: make(map[ClientID]*Client),
	}
}

func (s *clientService) Create() *Client {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.generateID()
	s.clients[id] = &Client{ID: id}
	return s.clients[id]
}

func (s *clientService) generateID() (id ClientID) {
	val, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}

	id = ClientID(val.String())

	if _, exists := s.lookup(id); exists {
		return s.generateID()
	}

	return id
}

func (s *clientService) lookup(id ClientID) (*Client, bool) {
	return s.clients[id], s.clients[id] != nil
}
