-- +goose Up
-- Physical wallet identity is separate from virtual instance and execution IDs.
CREATE TABLE managed_wallets (
    id text PRIMARY KEY,
    exchange text NOT NULL,
    mode text NOT NULL CHECK (mode IN ('backtest','paper','testnet','live')),
    market text NOT NULL CHECK (market = 'spot'),
    quote text NOT NULL,
    revision bigint NOT NULL DEFAULT 0,
    state jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(exchange, mode, market)
);
CREATE TABLE managed_journal (
    wallet_id text NOT NULL REFERENCES managed_wallets(id),
    sequence bigint NOT NULL,
    operation text NOT NULL,
    request_id text NOT NULL,
    payload jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(wallet_id, sequence),
    UNIQUE(wallet_id, request_id)
);
-- Legacy records remain immutable. Ambiguous ownership is never guessed.
CREATE TABLE managed_legacy_imports (
    wallet_id text NOT NULL REFERENCES managed_wallets(id),
    legacy_run_id bigint NOT NULL REFERENCES strategy_runs(id),
    status text NOT NULL CHECK (status IN ('review','imported')),
    notes jsonb NOT NULL DEFAULT '{}',
    PRIMARY KEY(wallet_id, legacy_run_id)
);
-- +goose Down
DROP TABLE managed_legacy_imports;
DROP TABLE managed_journal;
DROP TABLE managed_wallets;
