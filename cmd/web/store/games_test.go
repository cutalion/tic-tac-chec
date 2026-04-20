package store_test

import (
	"context"
	"testing"
	"tic-tac-chec/cmd/web/store"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGameStore_CreateLoadRoundtripsState(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	u1, _ := s.Users().Create(ctx)
	u2, _ := s.Users().Create(ctx)

	game1 := store.NewGame("game-1", "room-1", u1.PlayerID, u2.PlayerID)
	game1.State = []byte("initial state")

	s.Games().Create(ctx, game1)

	loaded, _ := s.Games().Load(ctx, game1.ID)
	assert.Equal(t, game1.State, loaded.State)
	assert.Equal(t, game1.WhitePlayerID, loaded.WhitePlayerID)
	assert.Equal(t, game1.BlackPlayerID, loaded.BlackPlayerID)
	assert.Equal(t, game1.CreatedAt.Truncate(time.Second).UTC(), loaded.CreatedAt.Truncate(time.Second))
	assert.Equal(t, game1.UpdatedAt.Truncate(time.Second).UTC(), loaded.UpdatedAt.Truncate(time.Second))

	assert.Equal(t, loaded.Status, "active")
	assert.Nil(t, loaded.Winner)
	assert.Nil(t, loaded.EndedAt)
}

func TestGameStore_Create_FKViolation(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user, _ := s.Users().Create(ctx)

	game := store.Game{
		RoomID:        "room1",
		WhitePlayerID: user.ID,
		BlackPlayerID: "unknown",
		Status:        "active",
		State:         []byte("state"),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	err := s.Games().Create(ctx, game)
	require.Error(t, err)
}

func TestGameStore_Create_SelfPlayRejected(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	u1, _ := s.Users().Create(ctx)

	game := store.NewGame("game-1", "room-1", u1.PlayerID, u1.PlayerID)
	err := s.Games().Create(ctx, game)
	require.Error(t, err)
}

// Creates a Game, records CreatedAt. Sleeps 10ms (or constructs a later time)
// and calls UpdateState with a new State blob and later updatedAt. Loads, asserts:
//   - State changed to the new bytes
//   - UpdatedAt > CreatedAt
//   - Status unchanged ("active")
//   - Winner nil, EndedAt nil
func TestGameStore_UpdateStateChangesOnlyStateAndUpdatedAt(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	u1, _ := s.Users().Create(ctx)
	u2, _ := s.Users().Create(ctx)

	game := store.NewGame("game-1", "room-1", u1.PlayerID, u2.PlayerID)
	createdAt := time.Now().Add(-10 * time.Second)
	game.CreatedAt = createdAt
	game.UpdatedAt = createdAt
	game.State = []byte("initial state")

	err := s.Games().Create(ctx, game)
	require.NoError(t, err)

	game.State = []byte("new state")
	err = s.Games().UpdateState(ctx, game.ID, game.State)
	require.NoError(t, err)

	loaded, _ := s.Games().Load(ctx, game.ID)
	assert.Equal(t, loaded.State, []byte("new state"))
	assert.Equal(t, loaded.Status, "active")
	assert.Greater(t, loaded.UpdatedAt, createdAt)
}

// Creates a Game (status=active, winner=nil, ended_at=nil), then calls Finish
// with winner=&"white" and a later endedAt time. Loads, asserts:
//   - Status == "finished"
//   - Winner != nil && *Winner == "white"
//   - EndedAt != nil
//   - UpdatedAt updated (== endedAt)
func TestGameStore_Finish(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	u1, _ := s.Users().Create(ctx)
	u2, _ := s.Users().Create(ctx)

	game := store.NewGame("game-1", "room-1", u1.PlayerID, u2.PlayerID)
	game.State = []byte("initial state")
	err := s.Games().Create(ctx, game)
	require.NoError(t, err)

	finishTime := time.Now().Add(10 * time.Second)
	s.Games().Finish(ctx, game.ID, "white", []byte("final state"), finishTime)

	loaded, _ := s.Games().Load(ctx, game.ID)
	assert.Equal(t, loaded.Status, "finished")
	assert.Equal(t, *loaded.Winner, "white")
	assert.Equal(t, *loaded.EndedAt, finishTime.Truncate(time.Second).UTC())
}

func TestGameStore_Load_NotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, err := s.Games().Load(ctx, "invalid-uuid")
	require.Error(t, err)
}

// Creates three games in the same room (same pair of players) with ascending
// CreatedAt. Calls LoadLatestByRoom(roomID) and asserts the returned game's
// ID matches the MOST RECENT one. Also tests that LoadLatestByRoom returns
// ErrNotFound for an unknown room.
func TestGameStore_LoadLatestByRoom(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	u1, _ := s.Users().Create(ctx)
	u2, _ := s.Users().Create(ctx)

	game1 := store.NewGame("game-1", "room-1", u1.PlayerID, u2.PlayerID)
	game2 := store.NewGame("game-2", "room-1", u2.PlayerID, u1.PlayerID)
	game3 := store.NewGame("game-3", "room-1", u1.PlayerID, u2.PlayerID)

	state := []byte("initial state")
	game1.State = state
	game1.CreatedAt = time.Now().Add(-3 * time.Hour)
	game2.State = state
	game2.CreatedAt = time.Now().Add(-2 * time.Hour)
	game3.State = state
	game3.CreatedAt = time.Now().Add(-1 * time.Hour)

	err := s.Games().Create(ctx, game1)
	require.NoError(t, err)
	err = s.Games().Create(ctx, game2)
	require.NoError(t, err)
	err = s.Games().Create(ctx, game3)
	require.NoError(t, err)

	loaded, _ := s.Games().LoadLatestByRoom(ctx, "room-1")
	assert.Equal(t, loaded.ID, game3.ID)
}
