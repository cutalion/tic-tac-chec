package store

import (
	"context"
	"database/sql"
	"errors"
)

type Player struct {
	ID     string
	UserID *string
	BotID  *string
}

type PlayerStore struct {
	db *sql.DB
}

const (
	insertPlayerSQL = `INSERT INTO players (id, user_id, bot_id) VALUES (?, ?, ?)`
	selectPlayerSQL = `SELECT id, user_id, bot_id FROM players WHERE id = ?`
	byBotPlayerSQL  = `SELECT id, user_id, bot_id FROM players WHERE bot_id = ? LIMIT 1`
)

func (s *PlayerStore) Get(ctx context.Context, id string) (Player, error) {
	row := s.db.QueryRowContext(ctx, selectPlayerSQL, id)
	var (
		idStr  string
		userID sql.NullString
		botID  sql.NullString
	)
	if err := row.Scan(&idStr, &userID, &botID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Player{}, ErrNotFound
		}
		return Player{}, err
	}

	player := Player{ID: idStr}

	if userID.Valid {
		s := userID.String
		player.UserID = &s
	}
	if botID.Valid {
		s := botID.String
		player.BotID = &s
	}

	return player, nil
}

func (s *PlayerStore) ByBot(ctx context.Context, botID string) (Player, error) {
	row := s.db.QueryRowContext(ctx, byBotPlayerSQL, botID)
	var (
		idStr    string
		userIDNS sql.NullString
		botIDNS  sql.NullString
	)
	if err := row.Scan(&idStr, &userIDNS, &botIDNS); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Player{}, ErrNotFound
		}
		return Player{}, err
	}

	player := Player{ID: idStr}

	if userIDNS.Valid {
		s := userIDNS.String
		player.UserID = &s
	}
	if botIDNS.Valid {
		s := botIDNS.String
		player.BotID = &s
	}

	return player, nil
}
