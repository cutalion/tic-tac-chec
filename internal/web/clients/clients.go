package clients

import (
	"context"
	"log/slog"
	store "tic-tac-chec/internal/web/persistence/sqlite"
)

type Client struct {
	ID       ClientID
	PlayerID string
}

type ClientID string

func (id ClientID) LogValue() slog.Value { return slog.StringValue(string(id)) }

const (
	BotClientID ClientID = "bot"
)

type ClientService interface {
	Create(ctx context.Context) (*Client, error)
	Lookup(ctx context.Context, id ClientID) (*Client, error)
}

type clientService struct {
	users *store.UserStore
}

func NewService(users *store.UserStore) ClientService {
	return &clientService{
		users: users,
	}
}

func (s *clientService) Create(ctx context.Context) (*Client, error) {
	user, err := s.users.Create(ctx)
	if err != nil {
		return nil, err
	}
	return &Client{ID: ClientID(user.ID), PlayerID: user.PlayerID}, nil
}

func (s *clientService) Lookup(ctx context.Context, id ClientID) (*Client, error) {
	user, err := s.users.Get(ctx, string(id))
	if err != nil {
		return nil, err
	}
	return &Client{ID: ClientID(user.ID), PlayerID: user.PlayerID}, nil
}
