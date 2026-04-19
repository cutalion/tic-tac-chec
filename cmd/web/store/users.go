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
	CreatedAt time.Time
}

type UserStore struct {
	db *sql.DB
}

const (
	insertSQL = `INSERT INTO users (id, created_at) VALUES (?, ?)`
	selectSQL = `SELECT id, created_at FROM users WHERE id = ?`
)

func (s *UserStore) Create(ctx context.Context) (User, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return User{}, err
	}
	idStr := id.String()
	createdAt := time.Now()
	_, err = s.db.ExecContext(ctx, insertSQL, idStr, formatTime(createdAt))
	if err != nil {
		return User{}, err
	}
	return User{ID: idStr, CreatedAt: createdAt}, nil
}

func (s *UserStore) Get(ctx context.Context, id string) (User, error) {
	row := s.db.QueryRowContext(ctx, selectSQL, id)
	var idStr string
	var createdAtStr string
	if err := row.Scan(&idStr, &createdAtStr); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	createdAt, err := parseTime(createdAtStr)
	if err != nil {
		return User{}, err
	}
	return User{ID: idStr, CreatedAt: createdAt}, nil
}
