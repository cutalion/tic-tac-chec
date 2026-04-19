package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type Game struct {
	ID            string
	RoomID        string
	WhitePlayerID string
	BlackPlayerID string
	Status        string
	Winner        *string
	State         []byte
	CreatedAt     time.Time
	UpdatedAt     time.Time
	EndedAt       *time.Time
}

type GameStore struct {
	db *sql.DB
}

const (
	insertGameSQL = `
	INSERT INTO games
		(id, room_id, white_player_id, black_player_id, status, winner, state, created_at, updated_at, ended_at)
	VALUES
		(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	selectGameSQL = `
	SELECT id, room_id, white_player_id, black_player_id, status, winner, state, created_at, updated_at, ended_at
	FROM games
	WHERE id = ?
	`

	updateGameStateSQL = `
	UPDATE games
	SET state = ?, updated_at = ?
	WHERE id = ?
	`

	finishGameSQL = `
	UPDATE games
	SET winner = ?, state = ?, ended_at = ?, status = 'finished'
	WHERE id = ?
	`

	selectLatestGameByRoomSQL = `
	SELECT id, room_id, white_player_id, black_player_id, status, winner, state, created_at, updated_at, ended_at
	FROM games
	WHERE room_id = ?
	ORDER BY created_at DESC
	LIMIT 1
	`
)

func NewGame(roomID, whitePlayerID, blackPlayerID string) (Game, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Game{}, err
	}

	game := Game{
		ID:            id.String(),
		RoomID:        roomID,
		WhitePlayerID: whitePlayerID,
		BlackPlayerID: blackPlayerID,
		Status:        "active",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	return game, nil
}

// Caller generates game.ID (uuid.NewV7().String()). Keeps persistor in control.
func (g *GameStore) Create(ctx context.Context, game Game) error {
	_, err := g.db.ExecContext(ctx, insertGameSQL,
		game.ID, game.RoomID, game.WhitePlayerID, game.BlackPlayerID,
		game.Status, game.Winner, game.State,
		formatTime(game.CreatedAt), formatTime(game.UpdatedAt), formatNullableTime(game.EndedAt),
	)

	return err
}

func (g *GameStore) UpdateState(ctx context.Context, id string, state []byte) error {
	_, err := g.db.ExecContext(ctx, updateGameStateSQL,
		state, formatTime(time.Now()), id,
	)
	return err
}

func (g *GameStore) Finish(ctx context.Context, id string, winner string, state []byte, endedAt time.Time) error {
	_, err := g.db.ExecContext(ctx, finishGameSQL,
		winner, state, formatTime(endedAt), id,
	)
	return err
}

func (g *GameStore) Load(ctx context.Context, id string) (Game, error) {
	var game Game
	var winnerNS sql.NullString
	var endedAtNS sql.NullString
	var createdAtStr string
	var updatedAtStr string

	err := g.db.QueryRowContext(ctx, selectGameSQL, id).Scan(
		&game.ID, &game.RoomID, &game.WhitePlayerID, &game.BlackPlayerID,
		&game.Status, &winnerNS, &game.State,
		&createdAtStr, &updatedAtStr, &endedAtNS,
	)

	if err != nil {
		return Game{}, err
	}

	if winnerNS.Valid {
		s := winnerNS.String
		game.Winner = &s
	}

	if endedAtNS.Valid {
		s := endedAtNS.String
		t, err := parseTime(s)
		if err != nil {
			return Game{}, err
		}
		game.EndedAt = &t
	}

	if game.CreatedAt, err = parseTime(createdAtStr); err != nil {
		return Game{}, err
	}
	if game.UpdatedAt, err = parseTime(updatedAtStr); err != nil {
		return Game{}, err
	}

	return game, err
}

func (g *GameStore) LoadLatestByRoom(ctx context.Context, roomID string) (Game, error) {
	var game Game
	err := g.db.QueryRowContext(ctx, selectLatestGameByRoomSQL, roomID).Scan(
		&game.ID, &game.RoomID, &game.WhitePlayerID, &game.BlackPlayerID,
		&game.Status, &game.Winner, &game.State,
		&game.CreatedAt, &game.UpdatedAt, &game.EndedAt,
	)
	return game, err
}
