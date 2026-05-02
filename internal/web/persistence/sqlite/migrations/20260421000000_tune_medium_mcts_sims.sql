-- +goose Up
UPDATE bots SET mcts_sims = 100 WHERE difficulty = 'medium';

-- +goose Down
UPDATE bots SET mcts_sims = 250 WHERE difficulty = 'medium';
