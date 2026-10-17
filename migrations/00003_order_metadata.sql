-- +goose Up
ALTER TABLE orders
    ADD COLUMN strategy text NOT NULL DEFAULT '',
    ADD COLUMN reason   text;
ALTER TABLE positions
    ADD COLUMN strategy text NOT NULL DEFAULT '';
CREATE INDEX strategy_runs_mode_idx ON strategy_runs (mode, instance_id, started_at DESC);

-- +goose Down
DROP INDEX strategy_runs_mode_idx;
ALTER TABLE positions DROP COLUMN strategy;
ALTER TABLE orders DROP COLUMN strategy, DROP COLUMN reason;
