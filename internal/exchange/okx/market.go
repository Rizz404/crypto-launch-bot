package okx

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

// bars uses the UTC-aligned variants for 6h and above (the plain ones are
// aligned to Hong Kong time).
var bars = map[domain.Timeframe]string{
	"1m": "1m", "3m": "3m", "5m": "5m", "15m": "15m", "30m": "30m", "1h": "1H", "2h": "2H",
	"4h": "4H", "6h": "6Hutc", "12h": "12Hutc", "1d": "1Dutc", "3d": "3Dutc", "1w": "1Wutc",
}

func bar(tf domain.Timeframe) (string, error) {
	if b, ok := bars[tf]; ok {
		return b, nil
	}
	return "", fmt.Errorf("okx has no %s candles", tf)
}

func timeframe(b string) domain.Timeframe {
	for tf, v := range bars {
		if v == b {
			return tf
		}
	}
	return ""
}

// Instruments returns spot pairs, or USDT linear swaps for futures. Swap
// sizes are converted to base units.
func (c *Client) Instruments(ctx context.Context, m domain.Market) ([]domain.Instrument, error) {
	var r []struct {
		InstID    string `json:"instId"`
		BaseCcy   string `json:"baseCcy"`
		QuoteCcy  string `json:"quoteCcy"`
		SettleCcy string `json:"settleCcy"`
		CtVal     string `json:"ctVal"`
		CtValCcy  string `json:"ctValCcy"`
		CtType    string `json:"ctType"`
		LotSz     string `json:"lotSz"`
		MinSz     string `json:"minSz"`
		TickSz    string `json:"tickSz"`
		Lever     string `json:"lever"`
		State     string `json:"state"`
	}
	if err := c.get(ctx, "/api/v5/public/instruments", url.Values{"instType": {instType(m)}}, &r); err != nil {
		return nil, err
	}
	out := make([]domain.Instrument, 0, len(r))
	for _, s := range r {
		in := domain.Instrument{
			Exchange: c.name, Market: m, Symbol: s.InstID, Status: s.State, Tradable: s.State == "live",
			TickSize: dec(s.TickSz), StepSize: dec(s.LotSz), MinQty: dec(s.MinSz), ContractSize: decimal.NewFromInt(1),
			Base: s.BaseCcy, Quote: s.QuoteCcy,
		}
		if m == domain.Futures {
			if s.CtType != "linear" || s.SettleCcy != "USDT" {
				continue
			}
			ct := dec(s.CtVal)
			in.Base, in.Quote, in.ContractSize = s.CtValCcy, s.SettleCcy, ct
			in.StepSize, in.MinQty = in.StepSize.Mul(ct), in.MinQty.Mul(ct)
			in.MaxLeverage, _ = strconv.Atoi(s.Lever)
			c.mu.Lock()
			c.ctVal[s.InstID] = ct
			c.mu.Unlock()
		}
		out = append(out, in)
	}
	return out, nil
}

// Tickers returns 24h statistics with best bid/ask.
func (c *Client) Tickers(ctx context.Context, m domain.Market) ([]domain.Ticker, error) {
	var r []struct {
		InstID    string `json:"instId"`
		Last      string `json:"last"`
		BidPx     string `json:"bidPx"`
		AskPx     string `json:"askPx"`
		VolCcy24h string `json:"volCcy24h"`
	}
	if err := c.get(ctx, "/api/v5/market/tickers", url.Values{"instType": {instType(m)}}, &r); err != nil {
		return nil, err
	}
	out := make([]domain.Ticker, len(r))
	for i, t := range r {
		last, vol := dec(t.Last), dec(t.VolCcy24h)
		if m == domain.Futures {
			vol = vol.Mul(last) // swap volCcy24h is in base currency
		}
		out[i] = domain.Ticker{Symbol: t.InstID, Last: last, QuoteVolume: vol, Bid: dec(t.BidPx), Ask: dec(t.AskPx)}
	}
	return out, nil
}

// Klines returns candles opening at or after req.Start, oldest first.
// OKX pages backwards: "before" excludes older, "after" excludes newer.
func (c *Client) Klines(ctx context.Context, req exchange.KlineRequest) ([]domain.Candle, error) {
	b, err := bar(req.Timeframe)
	if err != nil {
		return nil, err
	}
	limit := req.Limit
	if limit <= 0 || limit > klineLimit {
		limit = klineLimit
	}
	end := req.Start.Add(time.Duration(limit) * req.Timeframe.Duration())
	if !req.End.IsZero() && req.End.Before(end) {
		end = req.End
	}
	q := url.Values{
		"instId": {req.Symbol}, "bar": {b}, "limit": {strconv.Itoa(limit)},
		"before": {strconv.FormatInt(req.Start.UnixMilli()-1, 10)},
		"after":  {strconv.FormatInt(end.UnixMilli(), 10)},
	}
	var rows [][]string
	if err := c.get(ctx, "/api/v5/market/history-candles", q, &rows); err != nil {
		return nil, err
	}
	out := make([]domain.Candle, 0, len(rows))
	for _, row := range slices.Backward(rows) {
		k, err := parseCandle(row, req.Market)
		if err != nil {
			return nil, fmt.Errorf("%s candle: %w", req.Symbol, err)
		}
		k.Exchange, k.Market, k.Symbol, k.Timeframe = c.name, req.Market, req.Symbol, req.Timeframe
		k.Closed = !k.CloseTime().After(time.Now())
		out = append(out, k)
	}
	return out, nil
}

// parseCandle parses [ts, o, h, l, c, vol, volCcy, volCcyQuote, confirm].
// For swaps vol is in contracts, so the base volume is volCcy.
func parseCandle(row []string, m domain.Market) (domain.Candle, error) {
	var k domain.Candle
	if len(row) < 9 {
		return k, fmt.Errorf("expected 9 fields, got %d", len(row))
	}
	ms, err := strconv.ParseInt(row[0], 10, 64)
	if err != nil {
		return k, err
	}
	k.OpenTime = time.UnixMilli(ms).UTC()
	k.Open, k.High, k.Low, k.Close = dec(row[1]), dec(row[2]), dec(row[3]), dec(row[4])
	k.Volume, k.QuoteVolume = dec(row[5]), dec(row[7])
	if m == domain.Futures {
		k.Volume = dec(row[6])
	}
	k.Closed = row[8] == "1"
	return k, nil
}

const (
	argsPerConn  = 200
	pingInterval = 25 * time.Second
)

func ping(ctx context.Context, conn *websocket.Conn) error {
	return conn.Write(ctx, websocket.MessageText, []byte("ping"))
}

type wsArg struct {
	Channel  string `json:"channel"`
	InstID   string `json:"instId,omitempty"`
	InstType string `json:"instType,omitempty"`
}

// StreamKlines implements exchange.MarketData.
func (c *Client) StreamKlines(ctx context.Context, m domain.Market, subs []exchange.KlineSub) (<-chan exchange.StreamEvent, error) {
	if len(subs) == 0 {
		return nil, fmt.Errorf("no kline subscriptions")
	}
	args := make([]wsArg, len(subs))
	for i, s := range subs {
		b, err := bar(s.Timeframe)
		if err != nil {
			return nil, err
		}
		args[i] = wsArg{Channel: "candle" + b, InstID: s.Symbol}
	}
	out := make(chan exchange.StreamEvent, 1024)
	var wg sync.WaitGroup
	for start := 0; start < len(args); start += argsPerConn {
		chunk := args[start:min(start+argsPerConn, len(args))]
		ws := &exchange.WSStream{
			URL: c.ep.BusinessWS, HTTPClient: c.http, Log: c.log.With("market", m),
			IdleTimeout: time.Minute, Ping: ping, PingInterval: pingInterval,
			Setup: func(ctx context.Context, conn *websocket.Conn) error {
				return wsjson.Write(ctx, conn, map[string]any{"op": "subscribe", "args": chunk})
			},
		}
		onConnect := func() { send(ctx, out, exchange.StreamEvent{Kind: exchange.EventConnected}) }
		handle := func(b []byte) {
			for _, k := range c.parseCandleMsg(m, b) {
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

func (c *Client) parseCandleMsg(m domain.Market, b []byte) []domain.Candle {
	if string(b) == "pong" {
		return nil
	}
	var msg struct {
		Event string     `json:"event"`
		Msg   string     `json:"msg"`
		Arg   wsArg      `json:"arg"`
		Data  [][]string `json:"data"`
	}
	if err := json.Unmarshal(b, &msg); err != nil {
		return nil
	}
	if msg.Event == "error" {
		c.log.Error("candle subscription failed", "msg", msg.Msg)
		return nil
	}
	if !strings.HasPrefix(msg.Arg.Channel, "candle") {
		return nil
	}
	tf := timeframe(strings.TrimPrefix(msg.Arg.Channel, "candle"))
	out := make([]domain.Candle, 0, len(msg.Data))
	for _, row := range msg.Data {
		k, err := parseCandle(row, m)
		if err != nil {
			continue
		}
		k.Exchange, k.Market, k.Symbol, k.Timeframe = c.name, m, msg.Arg.InstID, tf
		out = append(out, k)
	}
	return out
}

func send(ctx context.Context, ch chan<- exchange.StreamEvent, ev exchange.StreamEvent) {
	select {
	case ch <- ev:
	case <-ctx.Done():
	}
}
