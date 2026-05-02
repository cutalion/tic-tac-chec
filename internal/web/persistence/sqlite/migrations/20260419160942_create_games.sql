-- +goose Up
CREATE TABLE games (
    id               TEXT PRIMARY KEY,
    room_id          TEXT NOT NULL,
    white_player_id  TEXT NOT NULL REFERENCES players(id),
    black_player_id  TEXT NOT NULL REFERENCES players(id),
    status           TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','finished')),
    winner           TEXT CHECK (winner IN ('white','black','draw') OR winner IS NULL),
    state            TEXT NOT NULL,
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL,
    ended_at         TEXT,
    CHECK(white_player_id != black_player_id)
);
CREATE INDEX idx_games_room   ON games(room_id);
CREATE INDEX idx_games_white  ON games(white_player_id);
CREATE INDEX idx_games_black  ON games(black_player_id);
CREATE INDEX idx_games_status ON games(status);

-- +goose Down
DROP INDEX idx_games_status;
DROP INDEX idx_games_black;
DROP INDEX idx_games_white;
DROP INDEX idx_games_room;
DROP TABLE games;
