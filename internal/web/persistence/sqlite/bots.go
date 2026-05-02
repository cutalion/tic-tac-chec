package store

import (
	"context"
	"database/sql"
	"errors"
)

type Bot struct {
	ID         string
	PlayerID   string
	Label      string
	Difficulty string
	Version    int
	Mcts_Sims  int
	ModelPath  string
}

type BotStore struct {
	db *sql.DB
}

const (
	selectBotsByVersionSQL = `
	SELECT bots.id as bot_id, players.id as player_id, label, difficulty, version, mcts_sims, model_path
	FROM bots
	INNER JOIN players ON bots.id = players.bot_id
	WHERE version = ?`

	selectBotSQL = `
	SELECT bots.id as bot_id, players.id as player_id, label, difficulty, version, mcts_sims, model_path
	FROM bots
	INNER JOIN players ON bots.id = players.bot_id
	WHERE bots.id = ?`

	selectBotByPlayerSQL = `
	SELECT bots.id as bot_id, players.id as player_id, label, difficulty, version, mcts_sims, model_path
	FROM bots
	INNER JOIN players ON bots.id = players.bot_id
	WHERE players.id = ?`

	selectLatestBotByDifficultySQL = `
	SELECT bots.id as bot_id, players.id as player_id, label, difficulty, version, mcts_sims, model_path
	FROM bots
	INNER JOIN players ON bots.id = players.bot_id
	WHERE difficulty = ?
	ORDER BY version DESC
	LIMIT 1`
)

func (s *BotStore) LoadBots(ctx context.Context, version int) ([]Bot, error) {
	rows, err := s.db.QueryContext(ctx, selectBotsByVersionSQL, version)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bots []Bot
	for rows.Next() {
		var bot Bot
		if err := rows.Scan(&bot.ID, &bot.PlayerID, &bot.Label, &bot.Difficulty, &bot.Version, &bot.Mcts_Sims, &bot.ModelPath); err != nil {
			return nil, err
		}
		bots = append(bots, bot)
	}
	return bots, nil
}

func (s *BotStore) Get(ctx context.Context, id string) (Bot, error) {
	row := s.db.QueryRowContext(ctx, selectBotSQL, id)
	return s.parseRow(row)
}

func (s *BotStore) GetByPlayer(ctx context.Context, playerID string) (Bot, error) {
	row := s.db.QueryRowContext(ctx, selectBotByPlayerSQL, playerID)
	return s.parseRow(row)
}

func (s *BotStore) GetLatestByDifficulty(ctx context.Context, difficulty string) (Bot, error) {
	row := s.db.QueryRowContext(ctx, selectLatestBotByDifficultySQL, difficulty)
	return s.parseRow(row)
}

func (s *BotStore) parseRow(row *sql.Row) (Bot, error) {
	var bot Bot
	err := row.Scan(&bot.ID, &bot.PlayerID, &bot.Label, &bot.Difficulty, &bot.Version, &bot.Mcts_Sims, &bot.ModelPath)

	if errors.Is(err, sql.ErrNoRows) {
		return Bot{}, ErrNotFound
	}

	if err != nil {
		return Bot{}, err
	}

	return bot, nil
}
