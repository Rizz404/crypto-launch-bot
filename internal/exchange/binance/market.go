package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/exchange"
)

type exchangeInfo struct {
	Symbols []struct {
		Symbol       string            `json:"symbol"`
		Status       string            `json:"status"`
		BaseAsset    string            `json:"baseAsset"`
		QuoteAsset   string            `json:"quoteAsset"`
		ContractType string            `json:"contractType"` // futures only
		Filters      []json.RawMessage `json:"filters"`
	} `json:"symbols"`
}

type filter struct {
	FilterType  string          `json:"filterType"`
	TickSize    decimal.Decimal `json:"tickSize"`
	StepSize    decimal.Decimal `json:"stepSize"`
	MinQty      decimal.Decimal `json:"minQty"`
	MinNotional decimal.Decimal `json:"minNotional"` // spot NOTIONAL / MIN_NOTIONAL
	Notional    decimal.Decimal `json:"notional"`    // futures MIN_NOTIONAL
}

// Instruments returns spot symbols, or USDⓈ-M perpetual contracts for
// futures (quarterly and TradFi contracts are skipped).
func (c *Client) Instruments(ctx context.Context, m domain.Market) ([]domain.Instrument, error) {
	var (
		info exchangeInfo
		err  error
	)
	if m == domain.Futures {
		err = c.get(ctx, m, "/fapi/v1/exchangeInfo", nil, 1, &info)
	} else {
		// Without showPermissionSets=false the response is ~17 MB.
		q := url.Values{"permissions": {"SPOT"}, "showPermissionSets": {"false"}}
		err = c.get(ctx, m, "/api/v3/exchangeInfo", q, 20, &info)
	}
	if err != nil {
		return nil, err
	}

	out := make([]domain.Instrument, 0, len(info.Symbols))
	for _, s := range info.Symbols {
		if m == domain.Futures && s.ContractType != "PERPETUAL" {
			continue
		}
		in := domain.Instrument{
			Exchange:     c.name,
			Market:       m,
			Symbol:       s.Symbol,
			Base:         s.BaseAsset,
			Quote:        s.QuoteAsset,
			Status:       s.Status,
			Tradable:     s.Status == "TRADING",
			ContractSize: decimal.NewFromInt(1),
		}
		for _, raw := range s.Filters {
			var f filter
			if err := json.Unmarshal(raw, &f); err != nil {
				return nil, fmt.Errorf("%s filter %s: %w", s.Symbol, raw, err)
			}
			switch f.FilterType {
			case "PRICE_FILTER":
				in.TickSize = f.TickSize
			case "LOT_SIZE":
				in.StepSize, in.MinQty = f.StepSize, f.MinQty
			case "NOTIONAL":
				in.MinNotional = f.MinNotional
			case "MIN_NOTIONAL":
				in.MinNotional = decimal.Max(f.MinNotional, f.Notional)
			}
		}
		out = append(out, in)
	}
	return out, nil
}

type ticker24h struct {
	Symbol      string          `json:"symbol"`
	LastPrice   decimal.Decimal `json:"lastPrice"`
	QuoteVolume decimal.Decimal `json:"quoteVolume"`
	BidPrice    decimal.Decimal `json:"bidPrice"` // spot only
	AskPrice    decimal.Decimal `json:"askPrice"` // spot only
}

type bookTicker struct {
	Symbol   string          `json:"symbol"`
	BidPrice decimal.Decimal `json:"bidPrice"`
	AskPrice decimal.Decimal `json:"askPrice"`
}

// Tickers returns 24h statistics with best bid/ask for every symbol.
func (c *Client) Tickers(ctx context.Context, m domain.Market) ([]domain.Ticker, error) {
	var stats []ticker24h
	if m == domain.Futures {
		if err := c.get(ctx, m, "/fapi/v1/ticker/24hr", nil, 40, &stats); err != nil {
			return nil, err
		}
		// The futures 24h ticker has no bid/ask; merge the book ticker.
		var books []bookTicker
		if err := c.get(ctx, m, "/fapi/v1/ticker/bookTicker", nil, 5, &books); err != nil {
			return nil, err
		}
		bySymbol := make(map[string]bookTicker, len(books))
		for _, b := range books {
			bySymbol[b.Symbol] = b
		}
		for i := range stats {
			b := bySymbol[stats[i].Symbol]
			stats[i].BidPrice, stats[i].AskPrice = b.BidPrice, b.AskPrice
		}
	} else if err := c.get(ctx, m, "/api/v3/ticker/24hr", nil, 80, &stats); err != nil {
		return nil, err
	}

	out := make([]domain.Ticker, len(stats))
	for i, s := range stats {
		out[i] = domain.Ticker{Symbol: s.Symbol, Last: s.LastPrice, QuoteVolume: s.QuoteVolume, Bid: s.BidPrice, Ask: s.AskPrice}
	}
	return out, nil
}

// Klines returns closed and open candles starting at req.Start.
func (c *Client) Klines(ctx context.Context, req exchange.KlineRequest) ([]domain.Candle, error) {
	limit := req.Limit
	if limit <= 0 || limit > klineLimit {
		limit = klineLimit
	}
	q := url.Values{
		"symbol":    {req.Symbol},
		"interval":  {string(req.Timeframe)},
		"limit":     {strconv.Itoa(limit)},
		"startTime": {strconv.FormatInt(req.Start.UnixMilli(), 10)},
	}
	if !req.End.IsZero() {
		q.Set("endTime", strconv.FormatInt(req.End.UnixMilli(), 10))
	}

	path, weight := "/api/v3/klines", 2
	if req.Market == domain.Futures {
		path, weight = "/fapi/v1/klines", futuresKlineWeight(limit)
	}
	var rows [][]json.RawMessage
	if err := c.get(ctx, req.Market, path, q, weight, &rows); err != nil {
		return nil, err
	}

	now := time.Now()
	out := make([]domain.Candle, 0, len(rows))
	for _, r := range rows {
		k, err := parseKlineRow(r)
		if err != nil {
			return nil, fmt.Errorf("%s %s kline: %w", req.Symbol, req.Timeframe, err)
		}
		k.Exchange, k.Market, k.Symbol, k.Timeframe = c.name, req.Market, req.Symbol, req.Timeframe
		k.Closed = !k.CloseTime().After(now)
		out = append(out, k)
	}
	return out, nil
}

func futuresKlineWeight(limit int) int {
	switch {
	case limit < 100:
		return 1
	case limit < 500:
		return 2
	case limit <= 1000:
		return 5
	default:
		return 10
	}
}

// parseKlineRow parses [openTime, open, high, low, close, volume, closeTime,
// quoteVolume, trades, ...].
func parseKlineRow(r []json.RawMessage) (domain.Candle, error) {
	var k domain.Candle
	if len(r) < 9 {
		return k, fmt.Errorf("expected at least 9 fields, got %d", len(r))
	}
	var openMs int64
	fields := []struct {
		raw json.RawMessage
		dst any
	}{
		{r[0], &openMs}, {r[1], &k.Open}, {r[2], &k.High}, {r[3], &k.Low},
		{r[4], &k.Close}, {r[5], &k.Volume}, {r[7], &k.QuoteVolume}, {r[8], &k.Trades},
	}
	for _, f := range fields {
		if err := json.Unmarshal(f.raw, f.dst); err != nil {
			return k, err
		}
	}
	k.OpenTime = time.UnixMilli(openMs).UTC()
	return k, nil
}
