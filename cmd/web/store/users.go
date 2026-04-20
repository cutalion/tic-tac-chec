package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        string
	PlayerID  string
	CreatedAt time.Time
}

type UserStore struct {
	db *sql.DB
}

const (
	insertUserSQL = `INSERT INTO users (id, created_at) VALUES (?, ?)`
	selectUserSQL = `SELECT users.id as user_id, players.id as player_id, users.created_at FROM users INNER JOIN players ON users.id = players.user_id WHERE users.id = ?`
)

func (s *UserStore) Create(ctx context.Context) (User, error) {
	userId := uuid.Must(uuid.NewV7()).String()
	playerID := uuid.Must(uuid.NewV7()).String()
	createdAt := time.Now()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, insertUserSQL, userId, formatTime(createdAt)); err != nil {
		return User{}, err
	}

	if _, err = tx.ExecContext(ctx, insertPlayerSQL, playerID, userId, nil); err != nil {
		return User{}, err
	}

	if err := tx.Commit(); err != nil {
		return User{}, err
	}

	return User{ID: userId, PlayerID: playerID, CreatedAt: createdAt}, nil
}

func (s *UserStore) Get(ctx context.Context, id string) (User, error) {
	row := s.db.QueryRowContext(ctx, selectUserSQL, id)
	var idStr string
	var playerID string
	var createdAtStr string
	if err := row.Scan(&idStr, &playerID, &createdAtStr); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	createdAt, err := parseTime(createdAtStr)
	if err != nil {
		return User{}, err
	}
	return User{ID: idStr, PlayerID: playerID, CreatedAt: createdAt}, nil
}
