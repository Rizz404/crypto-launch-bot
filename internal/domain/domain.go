// Package domain holds exchange-agnostic trading types.
//
// Prices and quantities always use decimal.Decimal, never float64.
package domain

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// Market is the kind of market a symbol trades on.
type Market string

const (
	Spot    Market = "spot"
	Futures Market = "futures" // linear (USDT-margined) perpetuals
)

// ParseMarket validates a market name.
func ParseMarket(s string) (Market, error) {
	switch m := Market(s); m {
	case Spot, Futures:
		return m, nil
	}
	return "", fmt.Errorf("unknown market %q (want spot or futures)", s)
}

// Candle is one OHLCV bar. OpenTime is the start of the bar in UTC.
type Candle struct {
	Exchange    string
	Market      Market
	Symbol      string
	Timeframe   Timeframe
	OpenTime    time.Time
	Open        decimal.Decimal
	High        decimal.Decimal
	Low         decimal.Decimal
	Close       decimal.Decimal
	Volume      decimal.Decimal
	QuoteVolume decimal.Decimal
	Trades      int64
	// Closed is false while the bar is still forming (live streams only).
	Closed bool
}

// CloseTime is the exclusive end of the bar.
func (c Candle) CloseTime() time.Time { return c.OpenTime.Add(c.Timeframe.Duration()) }

// Instrument holds the trading rules of a symbol.
type Instrument struct {
	Exchange     string
	Market       Market
	Symbol       string
	Base         string
	Quote        string
	Status       string // exchange-specific status, e.g. TRADING
	Tradable     bool
	TickSize     decimal.Decimal
	StepSize     decimal.Decimal
	MinQty       decimal.Decimal
	MinNotional  decimal.Decimal
	ContractSize decimal.Decimal
	MaxLeverage  int // 0 if unknown
}

// RoundPrice rounds p down to a multiple of TickSize.
func (i Instrument) RoundPrice(p decimal.Decimal) decimal.Decimal { return floorTo(p, i.TickSize) }

// RoundQty rounds q down to a multiple of StepSize.
func (i Instrument) RoundQty(q decimal.Decimal) decimal.Decimal { return floorTo(q, i.StepSize) }

// CheckOrder reports why an order of qty at price would be rejected by the
// exchange filters, or nil if it passes.
func (i Instrument) CheckOrder(price, qty decimal.Decimal) error {
	if qty.LessThan(i.MinQty) {
		return fmt.Errorf("%s: quantity %s below min %s", i.Symbol, qty, i.MinQty)
	}
	if n := price.Mul(qty); n.LessThan(i.MinNotional) {
		return fmt.Errorf("%s: notional %s below min %s", i.Symbol, n, i.MinNotional)
	}
	return nil
}

func floorTo(v, step decimal.Decimal) decimal.Decimal {
	if step.Sign() <= 0 {
		return v
	}
	return v.Div(step).Floor().Mul(step)
}

// Ticker is a 24h rolling market summary plus the best bid/ask.
type Ticker struct {
	Symbol      string
	Last        decimal.Decimal
	QuoteVolume decimal.Decimal
	Bid         decimal.Decimal
	Ask         decimal.Decimal
}

// SpreadBps returns the bid/ask spread in basis points of the mid price,
// and false if bid or ask is missing.
func (t Ticker) SpreadBps() (float64, bool) {
	if t.Bid.Sign() <= 0 || t.Ask.Sign() <= 0 {
		return 0, false
	}
	mid := t.Bid.Add(t.Ask).Div(decimal.NewFromInt(2))
	bps, _ := t.Ask.Sub(t.Bid).Div(mid).Mul(decimal.NewFromInt(10_000)).Float64()
	return bps, true
}

// SeriesKey identifies one candle series.
type SeriesKey struct {
	Exchange  string
	Market    Market
	Symbol    string
	Timeframe Timeframe
}

func (k SeriesKey) String() string {
	return k.Exchange + ":" + string(k.Market) + ":" + k.Symbol + ":" + string(k.Timeframe)
}

// Series returns the key of the series the candle belongs to.
func (c Candle) Series() SeriesKey {
	return SeriesKey{Exchange: c.Exchange, Market: c.Market, Symbol: c.Symbol, Timeframe: c.Timeframe}
}
