-- +goose Up
ALTER TABLE strategy_runs
    ADD COLUMN instance_id text NOT NULL DEFAULT '',
    ADD COLUMN exchange    text NOT NULL DEFAULT '',
    ADD COLUMN market      text NOT NULL DEFAULT '',
    ADD COLUMN symbols     text[] NOT NULL DEFAULT '{}',
    ADD COLUMN summary     jsonb;

-- Completed round trips, for every mode.
CREATE TABLE trades (
    id          bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id      bigint      NOT NULL REFERENCES strategy_runs (id) ON DELETE CASCADE,
    exchange    text        NOT NULL,
    market      text        NOT NULL,
    symbol      text        NOT NULL,
    strategy    text        NOT NULL,
    side        text        NOT NULL CHECK (side IN ('long', 'short')),
    quantity    numeric     NOT NULL,
    entry_price numeric     NOT NULL,
    exit_price  numeric     NOT NULL,
    opened_at   timestamptz NOT NULL,
    closed_at   timestamptz NOT NULL,
    gross_pnl   numeric     NOT NULL,
    fees        numeric     NOT NULL,
    funding     numeric     NOT NULL,
    net_pnl     numeric     NOT NULL,
    exit_reason text
);
CREATE INDEX trades_run_idx ON trades (run_id, closed_at);

-- +goose Down
DROP TABLE trades;
ALTER TABLE strategy_runs
    DROP COLUMN instance_id,
    DROP COLUMN exchange,
    DROP COLUMN market,
    DROP COLUMN symbols,
    DROP COLUMN summary;
