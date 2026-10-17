-- +goose Up
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- Trading rules per symbol, refreshed from the exchange.
CREATE TABLE instruments (
    exchange      text        NOT NULL,
    market        text        NOT NULL CHECK (market IN ('spot', 'futures')),
    symbol        text        NOT NULL,
    base_asset    text        NOT NULL,
    quote_asset   text        NOT NULL,
    status        text        NOT NULL,
    tick_size     numeric     NOT NULL,
    step_size     numeric     NOT NULL,
    min_qty       numeric     NOT NULL,
    min_notional  numeric     NOT NULL,
    contract_size numeric     NOT NULL DEFAULT 1,
    max_leverage  integer,
    updated_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (exchange, market, symbol)
);

CREATE TABLE candles (
    exchange     text        NOT NULL,
    market       text        NOT NULL CHECK (market IN ('spot', 'futures')),
    symbol       text        NOT NULL,
    timeframe    text        NOT NULL,
    open_time    timestamptz NOT NULL,
    open         numeric     NOT NULL,
    high         numeric     NOT NULL,
    low          numeric     NOT NULL,
    close        numeric     NOT NULL,
    volume       numeric     NOT NULL,
    quote_volume numeric     NOT NULL DEFAULT 0,
    trades       bigint      NOT NULL DEFAULT 0,
    PRIMARY KEY (exchange, market, symbol, timeframe, open_time)
);
SELECT create_hypertable('candles', by_range('open_time', INTERVAL '7 days'));

-- One row per strategy instance execution (backtest, paper, testnet or live).
CREATE TABLE strategy_runs (
    id          bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    strategy    text        NOT NULL,
    mode        text        NOT NULL CHECK (mode IN ('backtest', 'paper', 'testnet', 'live')),
    params      jsonb       NOT NULL DEFAULT '{}',
    started_at  timestamptz NOT NULL DEFAULT now(),
    stopped_at  timestamptz,
    stop_reason text
);

CREATE TABLE orders (
    id                bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id            bigint      NOT NULL REFERENCES strategy_runs (id),
    exchange          text        NOT NULL,
    market            text        NOT NULL CHECK (market IN ('spot', 'futures')),
    symbol            text        NOT NULL,
    client_order_id   text        NOT NULL UNIQUE,
    exchange_order_id text,
    side              text        NOT NULL CHECK (side IN ('buy', 'sell')),
    type              text        NOT NULL,
    time_in_force     text,
    reduce_only       boolean     NOT NULL DEFAULT false,
    price             numeric,
    stop_price        numeric,
    quantity          numeric     NOT NULL,
    filled_quantity   numeric     NOT NULL DEFAULT 0,
    avg_fill_price    numeric,
    status            text        NOT NULL,
    tag               text,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX orders_exchange_order_id_key
    ON orders (exchange, market, exchange_order_id) WHERE exchange_order_id IS NOT NULL;
CREATE INDEX orders_open_idx
    ON orders (run_id, symbol) WHERE status IN ('new', 'partially_filled');

CREATE TABLE fills (
    id        bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    order_id  bigint      NOT NULL REFERENCES orders (id),
    trade_id  text        NOT NULL,
    price     numeric     NOT NULL,
    quantity  numeric     NOT NULL,
    fee       numeric     NOT NULL DEFAULT 0,
    fee_asset text,
    is_maker  boolean     NOT NULL DEFAULT false,
    filled_at timestamptz NOT NULL,
    UNIQUE (order_id, trade_id)
);

CREATE TABLE positions (
    id           bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id       bigint      NOT NULL REFERENCES strategy_runs (id),
    exchange     text        NOT NULL,
    market       text        NOT NULL CHECK (market IN ('spot', 'futures')),
    symbol       text        NOT NULL,
    side         text        NOT NULL CHECK (side IN ('long', 'short')),
    quantity     numeric     NOT NULL,
    entry_price  numeric     NOT NULL,
    exit_price   numeric,
    leverage     integer     NOT NULL DEFAULT 1,
    margin_mode  text        CHECK (margin_mode IN ('isolated', 'cross')),
    realized_pnl numeric     NOT NULL DEFAULT 0,
    fees         numeric     NOT NULL DEFAULT 0,
    status       text        NOT NULL CHECK (status IN ('open', 'closed')),
    opened_at    timestamptz NOT NULL DEFAULT now(),
    closed_at    timestamptz
);
-- One-way mode: at most one open position per symbol per run.
CREATE UNIQUE INDEX positions_one_open_idx
    ON positions (run_id, exchange, market, symbol) WHERE status = 'open';

CREATE TABLE equity_snapshots (
    run_id         bigint      NOT NULL REFERENCES strategy_runs (id),
    exchange       text        NOT NULL,
    ts             timestamptz NOT NULL,
    equity         numeric     NOT NULL,
    balance        numeric     NOT NULL,
    unrealized_pnl numeric     NOT NULL DEFAULT 0,
    PRIMARY KEY (run_id, exchange, ts)
);
SELECT create_hypertable('equity_snapshots', by_range('ts', INTERVAL '30 days'));

-- +goose Down
DROP TABLE equity_snapshots;
DROP TABLE positions;
DROP TABLE fills;
DROP TABLE orders;
DROP TABLE strategy_runs;
DROP TABLE candles;
DROP TABLE instruments;
