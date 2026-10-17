package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/exchange"
)

var intervals = map[domain.Timeframe]string{
	"1m": "1", "3m": "3", "5m": "5", "15m": "15", "30m": "30", "1h": "60", "2h": "120",
	"4h": "240", "6h": "360", "12h": "720", "1d": "D", "1w": "W",
}

func interval(tf domain.Timeframe) (string, error) {
	if i, ok := intervals[tf]; ok {
		return i, nil
	}
	return "", fmt.Errorf("bybit has no %s klines", tf)
}

func timeframe(i string) domain.Timeframe {
	for tf, v := range intervals {
		if v == i {
			return tf
		}
	}
	return ""
}

type instrument struct {
	Symbol         string `json:"symbol"`
	ContractType   string `json:"contractType"`
	Status         string `json:"status"`
	BaseCoin       string `json:"baseCoin"`
	QuoteCoin      string `json:"quoteCoin"`
	LeverageFilter struct {
		MaxLeverage string `json:"maxLeverage"`
	} `json:"leverageFilter"`
	PriceFilter struct {
		TickSize decimal.Decimal `json:"tickSize"`
	} `json:"priceFilter"`
	LotSizeFilter struct {
		BasePrecision    decimal.Decimal `json:"basePrecision"` // spot step
		QtyStep          decimal.Decimal `json:"qtyStep"`       // linear step
		MinOrderQty      decimal.Decimal `json:"minOrderQty"`
		MinOrderAmt      decimal.Decimal `json:"minOrderAmt"`      // spot
		MinNotionalValue decimal.Decimal `json:"minNotionalValue"` // linear
	} `json:"lotSizeFilter"`
}

// Instruments returns spot pairs, or linear perpetuals for futures.
func (c *Client) Instruments(ctx context.Context, m domain.Market) ([]domain.Instrument, error) {
	var out []domain.Instrument
	cursor := ""
	for {
		q := url.Values{"category": {category(m)}, "limit": {"1000"}}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		var r struct {
			List   []instrument `json:"list"`
			Cursor string       `json:"nextPageCursor"`
		}
		if err := c.get(ctx, "/v5/market/instruments-info", q, &r); err != nil {
			return nil, err
		}
		for _, s := range r.List {
			if m == domain.Futures && s.ContractType != "LinearPerpetual" {
				continue
			}
			in := domain.Instrument{
				Exchange: c.name, Market: m, Symbol: s.Symbol, Base: s.BaseCoin, Quote: s.QuoteCoin,
				Status: s.Status, Tradable: s.Status == "Trading", TickSize: s.PriceFilter.TickSize,
				MinQty: s.LotSizeFilter.MinOrderQty, ContractSize: decimal.NewFromInt(1),
			}
			if m == domain.Futures {
				in.StepSize, in.MinNotional = s.LotSizeFilter.QtyStep, s.LotSizeFilter.MinNotionalValue
				if lev, err := strconv.ParseFloat(s.LeverageFilter.MaxLeverage, 64); err == nil {
					in.MaxLeverage = int(lev)
				}
			} else {
				in.StepSize, in.MinNotional = s.LotSizeFilter.BasePrecision, s.LotSizeFilter.MinOrderAmt
			}
			out = append(out, in)
		}
		if r.Cursor == "" || len(r.List) == 0 {
			return out, nil
		}
		cursor = r.Cursor
	}
}

// Tickers returns 24h statistics with best bid/ask.
func (c *Client) Tickers(ctx context.Context, m domain.Market) ([]domain.Ticker, error) {
	var r struct {
		List []struct {
			Symbol      string          `json:"symbol"`
			LastPrice   decimal.Decimal `json:"lastPrice"`
			Bid1Price   decimal.Decimal `json:"bid1Price"`
			Ask1Price   decimal.Decimal `json:"ask1Price"`
			Turnover24h decimal.Decimal `json:"turnover24h"`
		} `json:"list"`
	}
	if err := c.get(ctx, "/v5/market/tickers", url.Values{"category": {category(m)}}, &r); err != nil {
		return nil, err
	}
	out := make([]domain.Ticker, len(r.List))
	for i, t := range r.List {
		out[i] = domain.Ticker{Symbol: t.Symbol, Last: t.LastPrice, QuoteVolume: t.Turnover24h, Bid: t.Bid1Price, Ask: t.Ask1Price}
	}
	return out, nil
}

// Klines returns candles opening at or after req.Start, oldest first.
func (c *Client) Klines(ctx context.Context, req exchange.KlineRequest) ([]domain.Candle, error) {
	iv, err := interval(req.Timeframe)
	if err != nil {
		return nil, err
	}
	limit := req.Limit
	if limit <= 0 || limit > klineLimit {
		limit = klineLimit
	}
	end := req.End
	if end.IsZero() {
		end = req.Start.Add(time.Duration(limit) * req.Timeframe.Duration())
	}
	q := url.Values{
		"category": {category(req.Market)}, "symbol": {req.Symbol}, "interval": {iv}, "limit": {strconv.Itoa(limit)},
		"start": {strconv.FormatInt(req.Start.UnixMilli(), 10)}, "end": {strconv.FormatInt(end.UnixMilli(), 10)},
	}
	var r struct {
		List [][]string `json:"list"` // newest first: start, open, high, low, close, volume, turnover
	}
	if err := c.get(ctx, "/v5/market/kline", q, &r); err != nil {
		return nil, err
	}
	now := time.Now()
	out := make([]domain.Candle, 0, len(r.List))
	for _, row := range slices.Backward(r.List) {
		k, err := parseKline(row)
		if err != nil {
			return nil, fmt.Errorf("%s kline: %w", req.Symbol, err)
		}
		if k.OpenTime.Before(req.Start) {
			continue
		}
		k.Exchange, k.Market, k.Symbol, k.Timeframe = c.name, req.Market, req.Symbol, req.Timeframe
		k.Closed = !k.CloseTime().After(now)
		out = append(out, k)
	}
	return out, nil
}

func parseKline(row []string) (domain.Candle, error) {
	var k domain.Candle
	if len(row) < 7 {
		return k, fmt.Errorf("expected 7 fields, got %d", len(row))
	}
	ms, err := strconv.ParseInt(row[0], 10, 64)
	if err != nil {
		return k, err
	}
	k.OpenTime = time.UnixMilli(ms).UTC()
	for i, dst := range []*decimal.Decimal{&k.Open, &k.High, &k.Low, &k.Close, &k.Volume, &k.QuoteVolume} {
		if *dst, err = decimal.NewFromString(row[i+1]); err != nil {
			return k, err
		}
	}
	return k, nil
}

const (
	topicsPerConn = 200
	argsPerSub    = 10 // spot rejects larger subscribe requests
	pingInterval  = 20 * time.Second
)

func ping(ctx context.Context, conn *websocket.Conn) error {
	return wsjson.Write(ctx, conn, map[string]string{"op": "ping"})
}

// StreamKlines implements exchange.MarketData.
func (c *Client) StreamKlines(ctx context.Context, m domain.Market, subs []exchange.KlineSub) (<-chan exchange.StreamEvent, error) {
	if len(subs) == 0 {
		return nil, fmt.Errorf("no kline subscriptions")
	}
	topics := make([]string, len(subs))
	for i, s := range subs {
		iv, err := interval(s.Timeframe)
		if err != nil {
			return nil, err
		}
		topics[i] = "kline." + iv + "." + s.Symbol
	}
	out := make(chan exchange.StreamEvent, 1024)
	var wg sync.WaitGroup
	for start := 0; start < len(topics); start += topicsPerConn {
		chunk := topics[start:min(start+topicsPerConn, len(topics))]
		ws := &exchange.WSStream{
			URL: c.ep.PublicWS + "/" + category(m), HTTPClient: c.http, Log: c.log.With("market", m),
			IdleTimeout: time.Minute, Ping: ping, PingInterval: pingInterval,
			Setup: func(ctx context.Context, conn *websocket.Conn) error {
				for i := 0; i < len(chunk); i += argsPerSub {
					args := chunk[i:min(i+argsPerSub, len(chunk))]
					if err := wsjson.Write(ctx, conn, map[string]any{"op": "subscribe", "args": args}); err != nil {
						return err
					}
				}
				return nil
			},
		}
		onConnect := func() { send(ctx, out, exchange.StreamEvent{Kind: exchange.EventConnected}) }
		handle := func(b []byte) {
			for _, k := range c.parseKlineMsg(m, b) {
				send(ctx, out, exchange.StreamEvent{Kind: exchange.EventCandle, Candle: k})
			}
		}
		wg.Go(func() { _ = ws.Run(ctx, onConnect, handle) })
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out, nil
}

func (c *Client) parseKlineMsg(m domain.Market, b []byte) []domain.Candle {
	var msg struct {
		Topic string `json:"topic"`
		Data  []struct {
			Start    int64           `json:"start"`
			Interval string          `json:"interval"`
			Open     decimal.Decimal `json:"open"`
			Close    decimal.Decimal `json:"close"`
			High     decimal.Decimal `json:"high"`
			Low      decimal.Decimal `json:"low"`
			Volume   decimal.Decimal `json:"volume"`
			Turnover decimal.Decimal `json:"turnover"`
			Confirm  bool            `json:"confirm"`
		} `json:"data"`
	}
	if err := json.Unmarshal(b, &msg); err != nil || !strings.HasPrefix(msg.Topic, "kline.") {
		return nil // pong and subscription acks
	}
	symbol := msg.Topic[strings.LastIndexByte(msg.Topic, '.')+1:]
	out := make([]domain.Candle, 0, len(msg.Data))
	for _, d := range msg.Data {
		out = append(out, domain.Candle{
			Exchange: c.name, Market: m, Symbol: symbol, Timeframe: timeframe(d.Interval),
			OpenTime: time.UnixMilli(d.Start).UTC(), Open: d.Open, High: d.High, Low: d.Low, Close: d.Close,
			Volume: d.Volume, QuoteVolume: d.Turnover, Closed: d.Confirm,
		})
	}
	return out
}

func send(ctx context.Context, ch chan<- exchange.StreamEvent, ev exchange.StreamEvent) {
	select {
	case ch <- ev:
	case <-ctx.Done():
	}
}

// FundingRates implements exchange.FundingSource for linear perpetuals.
func (c *Client) FundingRates(ctx context.Context) ([]exchange.FundingRate, error) {
	var r struct {
		List []struct {
			Symbol          string          `json:"symbol"`
			FundingRate     decimal.Decimal `json:"fundingRate"`
			NextFundingTime string          `json:"nextFundingTime"`
		} `json:"list"`
	}
	if err := c.get(ctx, "/v5/market/tickers", url.Values{"category": {"linear"}}, &r); err != nil {
		return nil, err
	}
	out := make([]exchange.FundingRate, 0, len(r.List))
	for _, f := range r.List {
		next, _ := strconv.ParseInt(f.NextFundingTime, 10, 64)
		out = append(out, exchange.FundingRate{Symbol: f.Symbol, Rate: f.FundingRate, Next: time.UnixMilli(next).UTC()})
	}
	return out, nil
}
