-- +goose Up
-- Settled detail moved out of managed_wallets.state by compaction. Rows are
-- written in the same transaction that removes them from the state and are
-- never updated; aggregates (PnL, fees, holdings) stay in the state.
CREATE TABLE managed_archive (
    wallet_id text NOT NULL REFERENCES managed_wallets(id),
    kind text NOT NULL CHECK (kind IN ('order','execution','audit')),
    item_key text NOT NULL,
    occurred_at timestamptz NOT NULL,
    payload jsonb NOT NULL,
    archived_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (wallet_id, kind, item_key)
);
CREATE INDEX managed_archive_occurred ON managed_archive(wallet_id, occurred_at);
-- +goose Down
DROP TABLE managed_archive;
