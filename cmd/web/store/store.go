package store

import (
	"database/sql"
	"errors"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

var ErrNotFound = errors.New("store: not found")

func NewStore(path string) (*Store, error) {
	params := "?_pragma=foreign_keys(on)&_pragma=journal_mode(wal)&_pragma=synchronous(normal)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", path+params)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Users() *UserStore {
	return &UserStore{db: s.db}
}

func (s *Store) Players() *PlayerStore {
	return &PlayerStore{db: s.db}
}

func parseTime(str string) (time.Time, error) {
	return time.Parse(time.RFC3339, str)
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
