package store_test

import (
	"context"
	"errors"
	"testing"
	"tic-tac-chec/cmd/web/store"
	"time"

	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.NewStore(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestUserStore_CreateAndGet(t *testing.T) {
	s := newTestStore(t)
	user1, err := s.Users().Create(context.Background())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	user2, err := s.Users().Create(context.Background())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if user1.ID == user2.ID {
		t.Errorf("Expected different IDs, got %s", user1.ID)
	}
	if user1.CreatedAt.IsZero() {
		t.Errorf("Expected created_at to be set, got %v", user1.CreatedAt)
	}
	if user2.CreatedAt.IsZero() {
		t.Errorf("Expected created_at to be set, got %v", user2.CreatedAt)
	}

	u2, err := s.Users().Get(context.Background(), user2.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if u2.ID != user2.ID {
		t.Errorf("Expected user1, got %s", u2.ID)
	}
	if !u2.CreatedAt.Equal(user2.CreatedAt.Truncate(time.Second)) {
		t.Errorf("Expected created_at to be %v, got %v", user2.CreatedAt, u2.CreatedAt)
	}

}

func TestUserStore_Get_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Users().Get(context.Background(), "invalid-id")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestUserStore_CreateInsertsPlayer(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user, err := s.Users().Create(ctx)

	require.NoError(t, err)
	require.NotEmpty(t, user.ID)
	require.NotEmpty(t, user.PlayerID)
	require.NotZero(t, user.CreatedAt)
}
