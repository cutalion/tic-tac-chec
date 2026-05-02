package store_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlayerStore_ByBot_Seeded(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	player, err := store.Players().ByBot(ctx, "easy-v1")
	require.NoError(t, err)
	assert.NotNil(t, player)

	assert.Equal(t, "easy-v1", *player.BotID)
	assert.Nil(t, player.UserID)
}

func TestPlayerStore_ByBot_NotFound(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	player, err := store.Players().ByBot(ctx, "unknown-bot")
	require.Error(t, err)
	assert.Zero(t, player)
}
