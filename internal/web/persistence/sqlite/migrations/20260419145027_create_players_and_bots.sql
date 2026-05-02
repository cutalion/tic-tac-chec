-- +goose Up
CREATE TABLE bots (
    id         TEXT PRIMARY KEY,
    label      TEXT NOT NULL,
    difficulty TEXT NOT NULL,
    version    INTEGER NOT NULL,
    model_path TEXT NOT NULL,
    -- Number of MCTS simulations to run.
    -- 0 means greedy argmax, >0 means MCTS with that many simulations.
    mcts_sims  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE players (
    id      TEXT PRIMARY KEY,
    user_id TEXT UNIQUE REFERENCES users(id),
    bot_id  TEXT UNIQUE REFERENCES bots(id),
    CHECK ((user_id IS NULL) <> (bot_id IS NULL))
);

INSERT INTO bots (id, label, difficulty, version, model_path, mcts_sims) VALUES
  ('easy-v1',   'Easy',   'easy',   1, 'bot/models/bot.onnx',   0),
  ('medium-v1', 'Medium', 'medium', 1, 'bot/models/bot.onnx', 250),
  ('hard-v1',   'Hard',   'hard',   1, 'bot/models/bot.onnx', 500);

INSERT INTO players (id, bot_id) VALUES
  ('0194c000-0000-7001-8000-000000000001', 'easy-v1'),
  ('0194c000-0000-7001-8000-000000000002', 'medium-v1'),
  ('0194c000-0000-7001-8000-000000000003', 'hard-v1');

-- +goose Down
DROP TABLE players;
DROP TABLE bots;
