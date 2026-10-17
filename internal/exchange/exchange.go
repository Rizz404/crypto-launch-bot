// Package exchange defines the exchange-agnostic interfaces implemented by
// each exchange adapter (binance, bybit, okx).
package exchange

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/domain"
)

// MarketData is the public (unauthenticated) part of an exchange.
// Order and account methods live in a separate interface (fase 4).
type MarketData interface {
	// Name identifies the data source, e.g. "binance" or "binance-testnet".
	// It is stored with every candle so testnet data never mixes with mainnet.
	Name() string
	Instruments(ctx context.Context, m domain.Market) ([]domain.Instrument, error)
	Tickers(ctx context.Context, m domain.Market) ([]domain.Ticker, error)
	// Klines returns up to KlineLimit candles opening at or after req.Start.
	Klines(ctx context.Context, req KlineRequest) ([]domain.Candle, error)
	KlineLimit() int
	// StreamKlines streams candle updates until ctx is cancelled, reconnecting
	// on failure. The channel is closed when streaming stops.
	StreamKlines(ctx context.Context, m domain.Market, subs []KlineSub) (<-chan StreamEvent, error)
}

type KlineRequest struct {
	Market    domain.Market
	Symbol    string
	Timeframe domain.Timeframe
	Start     time.Time
	End       time.Time // optional
	Limit     int       // optional, defaults to KlineLimit
}

type KlineSub struct {
	Symbol    string
	Timeframe domain.Timeframe
}

type StreamEventKind int

const (
	// EventConnected is sent after every successful (re)connect. Consumers
	// should backfill any gap through REST when they receive it.
	EventConnected StreamEventKind = iota + 1
	EventCandle
)

type StreamEvent struct {
	Kind   StreamEventKind
	Candle domain.Candle // set for EventCandle
}

// APIError is an error response returned by an exchange.
type APIError struct {
	Exchange   string
	HTTPStatus int
	Code       int
	Message    string
	// RetryAfter is set when the exchange asked us to back off.
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s api error: http %d, code %d: %s", e.Exchange, e.HTTPStatus, e.Code, e.Message)
}

// RateLimited reports whether the error is a rate-limit or IP-ban response.
func (e *APIError) RateLimited() bool { return e.HTTPStatus == 429 || e.HTTPStatus == 418 }

// FundingRate is the current funding of a perpetual contract.
type FundingRate struct {
	Symbol string
	Rate   decimal.Decimal // per funding interval; positive: longs pay
	Next   time.Time
}

// FundingSource is implemented by market data clients that report funding
// rates of their perpetual contracts.
type FundingSource interface {
	FundingRates(ctx context.Context) ([]FundingRate, error)
}
