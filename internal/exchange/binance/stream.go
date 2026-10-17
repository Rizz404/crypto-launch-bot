package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/exchange"
)

// Binance allows 1024 streams per spot connection and 200 per futures
// connection; use the lower limit for both.
const maxStreamsPerConn = 200

// klineMsg mirrors a combined-stream kline event.
//
// encoding/json matches keys case-insensitively when no field matches
// exactly, and Binance uses keys that differ only in case ("l" low vs "L"
// last trade id, "v" vs "V", "t" vs "T", ...). Every upper-case key therefore
// needs its own field, or it would silently overwrite its lower-case twin.
type klineMsg struct {
	Stream string `json:"stream"`
	Data   struct {
		Event     string `json:"e"`
		EventTime int64  `json:"E"`
		Symbol    string `json:"s"`
		K         struct {
			OpenMs        int64           `json:"t"`
			CloseMs       int64           `json:"T"`
			Interval      string          `json:"i"`
			FirstTradeID  int64           `json:"f"`
			LastTradeID   int64           `json:"L"`
			Open          decimal.Decimal `json:"o"`
			Close         decimal.Decimal `json:"c"`
			High          decimal.Decimal `json:"h"`
			Low           decimal.Decimal `json:"l"`
			Volume        decimal.Decimal `json:"v"`
			QuoteVolume   decimal.Decimal `json:"q"`
			TakerBuyBase  decimal.Decimal `json:"V"`
			TakerBuyQuote decimal.Decimal `json:"Q"`
			Trades        int64           `json:"n"`
			Closed        bool            `json:"x"`
			Ignore        json.RawMessage `json:"B"`
		} `json:"k"`
	} `json:"data"`
}

// StreamURLs returns the combined-stream URLs for subs, split by the
// per-connection stream limit.
func (c *Client) StreamURLs(m domain.Market, subs []exchange.KlineSub) []string {
	base := c.ep.SpotWS
	if m == domain.Futures {
		base = c.ep.FuturesWS
	}
	var urls []string
	for start := 0; start < len(subs); start += maxStreamsPerConn {
		chunk := subs[start:min(start+maxStreamsPerConn, len(subs))]
		names := make([]string, len(chunk))
		for i, s := range chunk {
			names[i] = strings.ToLower(s.Symbol) + "@kline_" + string(s.Timeframe)
		}
		urls = append(urls, base+"/stream?streams="+strings.Join(names, "/"))
	}
	return urls
}

// StreamKlines implements exchange.MarketData.
func (c *Client) StreamKlines(ctx context.Context, m domain.Market, subs []exchange.KlineSub) (<-chan exchange.StreamEvent, error) {
	if len(subs) == 0 {
		return nil, fmt.Errorf("no kline subscriptions")
	}
	out := make(chan exchange.StreamEvent, 1024)
	var wg sync.WaitGroup
	for _, u := range c.StreamURLs(m, subs) {
		ws := &exchange.WSStream{
			URL:        u,
			HTTPClient: c.http,
			Log:        c.log.With("market", m),
			// 1m klines update every ~2s; a minute of silence means the
			// connection is dead even if TCP is still open.
			IdleTimeout: time.Minute,
		}
		onConnect := func() {
			c.log.Info("kline stream connected", "market", m, "streams", strings.Count(u, "@kline_"))
			send(ctx, out, exchange.StreamEvent{Kind: exchange.EventConnected})
		}
		handle := func(b []byte) {
			k, ok, err := c.parseKlineMsg(m, b)
			if err != nil {
				c.log.Warn("bad kline message", "err", err, "msg", truncate(b, 200))
				return
			}
			if ok {
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

func (c *Client) parseKlineMsg(m domain.Market, b []byte) (domain.Candle, bool, error) {
	var msg klineMsg
	if err := json.Unmarshal(b, &msg); err != nil {
		return domain.Candle{}, false, err
	}
	if msg.Data.Event != "kline" {
		return domain.Candle{}, false, nil
	}
	k := msg.Data.K
	return domain.Candle{
		Exchange:    c.name,
		Market:      m,
		Symbol:      msg.Data.Symbol,
		Timeframe:   domain.Timeframe(k.Interval),
		OpenTime:    time.UnixMilli(k.OpenMs).UTC(),
		Open:        k.Open,
		High:        k.High,
		Low:         k.Low,
		Close:       k.Close,
		Volume:      k.Volume,
		QuoteVolume: k.QuoteVolume,
		Trades:      k.Trades,
		Closed:      k.Closed,
	}, true, nil
}

func send(ctx context.Context, ch chan<- exchange.StreamEvent, ev exchange.StreamEvent) {
	select {
	case ch <- ev:
	case <-ctx.Done():
	}
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "..."
	}
	return string(b)
}
