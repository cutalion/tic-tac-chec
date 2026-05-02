-- +goose Up
CREATE TABLE users (
    id         TEXT PRIMARY KEY,
    created_at TEXT NOT NULL
);

-- +goose Down
DROP TABLE users;
