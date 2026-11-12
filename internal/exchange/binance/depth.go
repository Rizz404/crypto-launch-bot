package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rizz404/crypto-launch-bot/internal/domain"
	"github.com/rizz404/crypto-launch-bot/internal/exchange"
)

// depthWeight is the spot request weight of /api/v3/depth by limit.
func depthWeight(limit int) int {
	switch {
	case limit <= 100:
		return 5
	case limit <= 500:
		return 25
	case limit <= 1000:
		return 50
	}
	return 250
}

func levels(raw [][2]decimal.Decimal) []exchange.Level {
	out := make([]exchange.Level, len(raw))
	for i, l := range raw {
		out[i] = exchange.Level{Price: l[0], Qty: l[1]}
	}
	return out
}

// Depth returns a spot order book snapshot.
func (c *Client) Depth(ctx context.Context, symbol string, limit int) (exchange.DepthSnapshot, error) {
	var raw struct {
		LastUpdateID int64                `json:"lastUpdateId"`
		Bids         [][2]decimal.Decimal `json:"bids"`
		Asks         [][2]decimal.Decimal `json:"asks"`
	}
	q := url.Values{"symbol": {symbol}, "limit": {strconv.Itoa(limit)}}
	if err := c.get(ctx, domain.Spot, "/api/v3/depth", q, depthWeight(limit), &raw); err != nil {
		return exchange.DepthSnapshot{}, err
	}
	return exchange.DepthSnapshot{LastUpdateID: raw.LastUpdateID, Bids: levels(raw.Bids), Asks: levels(raw.Asks)}, nil
}

// marketMsg mirrors the combined-stream events used by the trading screen.
// Upper/lower-case twins need separate fields (see klineMsg).
type marketMsg struct {
	Stream string          `json:"stream"`
	Data   json.RawMessage `json:"data"`
}
type depthMsg struct {
	Event     string               `json:"e"`
	EventTime int64                `json:"E"`
	Symbol    string               `json:"s"`
	FirstID   int64                `json:"U"`
	FinalID   int64                `json:"u"`
	Bids      [][2]decimal.Decimal `json:"b"`
	Asks      [][2]decimal.Decimal `json:"a"`
}
type aggTradeMsg struct {
	Event      string          `json:"e"`
	EventTime  int64           `json:"E"`
	Symbol     string          `json:"s"`
	ID         int64           `json:"a"`
	Price      decimal.Decimal `json:"p"`
	Qty        decimal.Decimal `json:"q"`
	FirstID    int64           `json:"f"`
	LastID     int64           `json:"l"`
	TradeTime  int64           `json:"T"`
	BuyerMaker bool            `json:"m"`
	Ignore     bool            `json:"M"`
}
type tickerMsg struct {
	Event     string          `json:"e"`
	EventTime int64           `json:"E"`
	Symbol    string          `json:"s"`
	Change    decimal.Decimal `json:"p"`
	ChangePct decimal.Decimal `json:"P"`
	Last      decimal.Decimal `json:"c"`
	LastQty   decimal.Decimal `json:"Q"`
	High      decimal.Decimal `json:"h"`
	Low       decimal.Decimal `json:"l"`
	Volume    decimal.Decimal `json:"v"`
	QuoteVol  decimal.Decimal `json:"q"`
	Open      decimal.Decimal `json:"o"`
	OpenTime  int64           `json:"O"`
	CloseTime int64           `json:"C"`
	Weighted  decimal.Decimal `json:"w"`
	First     int64           `json:"F"`
	LastID    int64           `json:"L"`
	Count     int64           `json:"n"`
	Bid       decimal.Decimal `json:"b"`
	BidQty    decimal.Decimal `json:"B"`
	Ask       decimal.Decimal `json:"a"`
	AskQty    decimal.Decimal `json:"A"`
	PrevClose decimal.Decimal `json:"x"`
}

// marketStreams lists the combined-stream names for symbols and timeframes.
func marketStreams(symbols []string, timeframes []domain.Timeframe) []string {
	var names []string
	for _, s := range symbols {
		s = strings.ToLower(s)
		names = append(names, s+"@depth@100ms", s+"@aggTrade", s+"@ticker")
		for _, tf := range timeframes {
			names = append(names, s+"@kline_"+string(tf))
		}
	}
	return names
}

// StreamMarket streams depth diffs, trades, 24h tickers and klines of spot
// symbols on shared connections until ctx is cancelled.
func (c *Client) StreamMarket(ctx context.Context, symbols []string, timeframes []domain.Timeframe) (<-chan exchange.MarketEvent, error) {
	names := marketStreams(symbols, timeframes)
	if len(names) == 0 {
		return nil, fmt.Errorf("no market streams")
	}
	out := make(chan exchange.MarketEvent, 4096)
	var wg sync.WaitGroup
	for start := 0; start < len(names); start += maxStreamsPerConn {
		chunk := names[start:min(start+maxStreamsPerConn, len(names))]
		ws := &exchange.WSStream{URL: c.ep.SpotWS + "/stream?streams=" + strings.Join(chunk, "/"), HTTPClient: c.http, Log: c.log, IdleTimeout: time.Minute}
		onConnect := func() {
			c.log.Info("market stream connected", "streams", len(chunk))
			sendMarket(ctx, out, exchange.MarketEvent{Kind: exchange.MarketConnected})
		}
		handle := func(b []byte) {
			ev, ok, err := c.parseMarketMsg(b)
			if err != nil {
				c.log.Warn("bad market message", "err", err, "msg", truncate(b, 200))
				return
			}
			if ok {
				sendMarket(ctx, out, ev)
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

func (c *Client) parseMarketMsg(b []byte) (exchange.MarketEvent, bool, error) {
	var msg marketMsg
	if err := json.Unmarshal(b, &msg); err != nil {
		return exchange.MarketEvent{}, false, err
	}
	_, kind, _ := strings.Cut(msg.Stream, "@")
	switch {
	case strings.HasPrefix(kind, "depth"):
		var d depthMsg
		if err := json.Unmarshal(msg.Data, &d); err != nil {
			return exchange.MarketEvent{}, false, err
		}
		return exchange.MarketEvent{Kind: exchange.MarketDepth, Depth: exchange.DepthUpdate{Symbol: d.Symbol, FirstID: d.FirstID, FinalID: d.FinalID, Bids: levels(d.Bids), Asks: levels(d.Asks)}}, true, nil
	case kind == "aggTrade":
		var t aggTradeMsg
		if err := json.Unmarshal(msg.Data, &t); err != nil {
			return exchange.MarketEvent{}, false, err
		}
		return exchange.MarketEvent{Kind: exchange.MarketTrade, Trade: exchange.Trade{Symbol: t.Symbol, Price: t.Price, Qty: t.Qty, Time: time.UnixMilli(t.TradeTime).UTC(), BuyerMaker: t.BuyerMaker}}, true, nil
	case kind == "ticker":
		var t tickerMsg
		if err := json.Unmarshal(msg.Data, &t); err != nil {
			return exchange.MarketEvent{}, false, err
		}
		return exchange.MarketEvent{Kind: exchange.MarketTicker, Ticker: exchange.Ticker24{Symbol: t.Symbol, Last: t.Last, Change: t.Change, ChangePct: t.ChangePct, High: t.High, Low: t.Low, Volume: t.Volume, QuoteVol: t.QuoteVol}}, true, nil
	case strings.HasPrefix(kind, "kline_"):
		k, ok, err := c.parseKlineMsg(domain.Spot, b)
		return exchange.MarketEvent{Kind: exchange.MarketCandle, Candle: k}, ok, err
	}
	return exchange.MarketEvent{}, false, nil
}

func sendMarket(ctx context.Context, ch chan<- exchange.MarketEvent, ev exchange.MarketEvent) {
	select {
	case ch <- ev:
	case <-ctx.Done():
	}
}
