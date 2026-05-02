package store_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBotStore_Get(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	bot, err := s.Bots().Get(ctx, "easy-v1")
	require.NoError(t, err)
	assert.Equal(t, bot.ID, "easy-v1")
	assert.Equal(t, bot.Label, "Easy")
	assert.Equal(t, bot.Difficulty, "easy")
	assert.Equal(t, bot.Version, 1)
	assert.Equal(t, bot.PlayerID, "0194c000-0000-7001-8000-000000000001")
	assert.Equal(t, bot.Mcts_Sims, 0)
}

func TestBotStore_GetByPlayer(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	bot, err := s.Bots().GetByPlayer(ctx, "0194c000-0000-7001-8000-000000000002")
	require.NoError(t, err)
	assert.Equal(t, bot.ID, "medium-v1")
	assert.Equal(t, bot.Label, "Medium")
	assert.Equal(t, bot.Difficulty, "medium")
	assert.Equal(t, bot.Version, 1)
	assert.Equal(t, bot.PlayerID, "0194c000-0000-7001-8000-000000000002")
	assert.Equal(t, bot.Mcts_Sims, 100)
}

func TestBotStore_LoadBots(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	bots, err := s.Bots().LoadBots(ctx, 1)
	require.NoError(t, err)
	assert.Len(t, bots, 3)
}
