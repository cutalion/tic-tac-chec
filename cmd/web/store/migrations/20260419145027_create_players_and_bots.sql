-- +goose Up
CREATE TABLE bots (
    id         TEXT PRIMARY KEY,
    label      TEXT NOT NULL,
    model_path TEXT NOT NULL,
    mcts_sims  INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);

CREATE TABLE players (
    id      TEXT PRIMARY KEY,
    user_id TEXT UNIQUE REFERENCES users(id),
    bot_id  TEXT UNIQUE REFERENCES bots(id),
    CHECK ((user_id IS NULL) <> (bot_id IS NULL))
);

INSERT INTO bots (id, label, model_path, mcts_sims, created_at) VALUES
  ('easy-v1',   'Easy',   'bot/models/bot.onnx',   0, '1970-01-01T00:00:00Z'),
  ('medium-v1', 'Medium', 'bot/models/bot.onnx', 250, '1970-01-01T00:00:00Z'),
  ('hard-v1',   'Hard',   'bot/models/bot.onnx', 500, '1970-01-01T00:00:00Z');

INSERT INTO players (id, bot_id) VALUES
  ('0194c000-0000-7001-8000-000000000001', 'easy-v1'),
  ('0194c000-0000-7001-8000-000000000002', 'medium-v1'),
  ('0194c000-0000-7001-8000-000000000003', 'hard-v1');

-- +goose Down
DROP TABLE players;
DROP TABLE bots;
