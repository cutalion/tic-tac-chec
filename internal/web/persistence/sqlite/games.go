package store

import (
	"context"
	"database/sql"
	"time"
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

	upsertGameSQL = `
	INSERT INTO games
		(id, room_id, white_player_id, black_player_id, status, winner, state, created_at, updated_at, ended_at)
	VALUES
		(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT (id) DO NOTHING
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

	selectActiveGamesSQL = `
	SELECT id, room_id, white_player_id, black_player_id, status, winner, state, created_at, updated_at, ended_at
	FROM games
	WHERE status = 'active'
	`
)

func NewGame(gameID, roomID, whitePlayerID, blackPlayerID string) Game {
	game := Game{
		ID:            gameID,
		RoomID:        roomID,
		WhitePlayerID: whitePlayerID,
		BlackPlayerID: blackPlayerID,
		Status:        "active",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	return game
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

func (g *GameStore) Upsert(ctx context.Context, game Game) error {
	_, err := g.db.ExecContext(ctx, upsertGameSQL,
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
	row := g.db.QueryRowContext(ctx, selectGameSQL, id)
	return g.scan(row)
}

func (g *GameStore) LoadLatestByRoom(ctx context.Context, roomID string) (Game, error) {
	row := g.db.QueryRowContext(ctx, selectLatestGameByRoomSQL, roomID)
	return g.scan(row)
}

func (g *GameStore) LoadActive(ctx context.Context) ([]Game, error) {
	rows, err := g.db.QueryContext(ctx, selectActiveGamesSQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []Game
	for rows.Next() {
		game, err := g.scan(rows)
		if err != nil {
			return nil, err
		}
		games = append(games, game)
	}
	return games, nil
}

func (g *GameStore) scan(row rowScanner) (Game, error) {
	var game Game
	var winnerNS sql.NullString
	var endedAtNS sql.NullString
	var createdAtStr string
	var updatedAtStr string
	if err := row.Scan(
		&game.ID, &game.RoomID, &game.WhitePlayerID, &game.BlackPlayerID,
		&game.Status, &winnerNS, &game.State,
		&createdAtStr, &updatedAtStr, &endedAtNS,
	); err != nil {
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

	var err error
	if game.CreatedAt, err = parseTime(createdAtStr); err != nil {
		return Game{}, err
	}
	if game.UpdatedAt, err = parseTime(updatedAtStr); err != nil {
		return Game{}, err
	}
	return game, nil
}
