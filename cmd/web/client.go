package main

import (
	"context"
	"tic-tac-chec/cmd/web/store"
)

type Client struct {
	ID ClientID
}

type ClientID string

type ClientService interface {
	Create(ctx context.Context) (*Client, error)
	Lookup(ctx context.Context, id ClientID) (*Client, error)
}

type clientService struct {
	users *store.UserStore
}

func NewClientService(users *store.UserStore) ClientService {
	return &clientService{
		users: users,
	}
}

func (s *clientService) Create(ctx context.Context) (*Client, error) {
	user, err := s.users.Create(ctx)
	if err != nil {
		return nil, err
	}
	return &Client{ID: ClientID(user.ID)}, nil
}

func (s *clientService) Lookup(ctx context.Context, id ClientID) (*Client, error) {
	user, err := s.users.Get(ctx, string(id))
	if err != nil {
		return nil, err
	}
	return &Client{ID: ClientID(user.ID)}, nil
}
