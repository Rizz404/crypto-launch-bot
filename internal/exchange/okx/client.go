// Package okx implements the exchange interfaces for OKX v5: spot and USDT
// linear perpetual swaps. Swap sizes are in contracts on OKX; this package
// converts them to base units (size × ctVal) so the rest of the bot always
// works in base units.
package okx

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/exchange"
)

type Endpoints struct {
	REST       string
	PublicWS   string
	BusinessWS string // candles
	PrivateWS  string
}

var (
	Mainnet = Endpoints{
		REST: "https://www.okx.com", PublicWS: "wss://ws.okx.com:8443/ws/v5/public",
		BusinessWS: "wss://ws.okx.com:8443/ws/v5/business", PrivateWS: "wss://ws.okx.com:8443/ws/v5/private",
	}
	// Demo trading uses the same REST host with the simulated-trading header.
	Demo = Endpoints{
		REST: "https://www.okx.com", PublicWS: "wss://wspap.okx.com/ws/v5/public",
		BusinessWS: "wss://wspap.okx.com/ws/v5/business", PrivateWS: "wss://wspap.okx.com/ws/v5/private",
	}
)

const (
	requestsPerMin = 1200 // per-endpoint limits are ~20-60 per 2s
	klineLimit     = 100  // history-candles page size
)

type Options struct {
	Testnet    bool // demo trading
	Endpoints  *Endpoints
	HTTPClient *http.Client
	Log        *slog.Logger
}

type Client struct {
	name       string
	ep         Endpoints
	demo       bool
	http       *http.Client
	log        *slog.Logger
	lim        *exchange.WeightLimiter
	timeOffset atomic.Int64

	key, passphrase string
	secret          []byte

	mu    sync.Mutex
	ctVal map[string]decimal.Decimal // swap contract value by instId
}

var _ exchange.MarketData = (*Client)(nil)

func New(opts Options) *Client {
	c := &Client{name: "okx", ep: Mainnet, http: opts.HTTPClient, log: opts.Log,
		lim: exchange.NewWeightLimiter(requestsPerMin), ctVal: map[string]decimal.Decimal{}}
	if opts.Testnet {
		c.name, c.ep, c.demo = "okx-testnet", Demo, true
	}
	if opts.Endpoints != nil {
		c.ep = *opts.Endpoints
	}
	if c.http == nil {
		c.http = &http.Client{Timeout: 30 * time.Second}
	}
	if c.log == nil {
		c.log = slog.Default()
	}
	c.log = c.log.With("exchange", c.name)
	return c
}

func (c *Client) Name() string    { return c.name }
func (c *Client) KlineLimit() int { return klineLimit }

func instType(m domain.Market) string {
	if m == domain.Futures {
		return "SWAP"
	}
	return "SPOT"
}

// contractValue returns ctVal of a swap (1 for spot).
func (c *Client) contractValue(ctx context.Context, m domain.Market, instID string) (decimal.Decimal, error) {
	if m != domain.Futures {
		return decimal.NewFromInt(1), nil
	}
	c.mu.Lock()
	v, ok := c.ctVal[instID]
	c.mu.Unlock()
	if ok {
		return v, nil
	}
	if _, err := c.Instruments(ctx, domain.Futures); err != nil {
		return decimal.Zero, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if v, ok = c.ctVal[instID]; !ok {
		return decimal.Zero, fmt.Errorf("unknown swap %s", instID)
	}
	return v, nil
}

type envelope struct {
	Code string          `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func (c *Client) call(ctx context.Context, method, path string, q url.Values, body any, signed bool, out any) error {
	if err := c.lim.Wait(ctx, 1); err != nil {
		return err
	}
	requestPath := path
	if len(q) > 0 {
		requestPath += "?" + q.Encode()
	}
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.ep.REST+requestPath, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.demo {
		req.Header.Set("x-simulated-trading", "1")
	}
	if signed {
		ts := time.Now().Add(time.Duration(c.timeOffset.Load()) * time.Millisecond).UTC().Format("2006-01-02T15:04:05.000Z")
		req.Header.Set("OK-ACCESS-KEY", c.key)
		req.Header.Set("OK-ACCESS-PASSPHRASE", c.passphrase)
		req.Header.Set("OK-ACCESS-TIMESTAMP", ts)
		req.Header.Set("OK-ACCESS-SIGN", c.sign(ts+method+requestPath+string(payload)))
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s %s: %w", c.name, method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return err
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("%s %s %s: http %d: %w", c.name, method, path, resp.StatusCode, err)
	}
	if env.Code != "0" {
		code, _ := strconv.Atoi(env.Code)
		apiErr := &exchange.APIError{Exchange: c.name, HTTPStatus: resp.StatusCode, Code: code, Message: env.Msg}
		// Batch-style endpoints report the reason per item.
		var items []struct {
			SCode string `json:"sCode"`
			SMsg  string `json:"sMsg"`
		}
		if json.Unmarshal(env.Data, &items) == nil && len(items) > 0 && items[0].SCode != "" && items[0].SCode != "0" {
			apiErr.Code, _ = strconv.Atoi(items[0].SCode)
			apiErr.Message = items[0].SMsg
		}
		if code == 50011 { // rate limit
			apiErr.RetryAfter = 2 * time.Second
			c.lim.Pause(apiErr.RetryAfter)
		}
		return apiErr
	}
	if out != nil {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("%s %s %s: decode: %w", c.name, method, path, err)
		}
	}
	return nil
}

func (c *Client) sign(payload string) string {
	m := hmac.New(sha256.New, c.secret)
	_, _ = m.Write([]byte(payload))
	return base64.StdEncoding.EncodeToString(m.Sum(nil))
}

func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	return c.call(ctx, http.MethodGet, path, q, nil, false, out)
}

// SyncTime measures the offset between local and server clocks.
func (c *Client) SyncTime(ctx context.Context) error {
	var r []struct {
		TS string `json:"ts"`
	}
	start := time.Now()
	if err := c.get(ctx, "/api/v5/public/time", nil, &r); err != nil {
		return err
	}
	if len(r) == 0 {
		return errors.New("empty server time")
	}
	ms, err := strconv.ParseInt(r[0].TS, 10, 64)
	if err != nil {
		return err
	}
	c.timeOffset.Store(ms - start.Add(time.Since(start)/2).UnixMilli())
	return nil
}

func msTime(s string) time.Time {
	v, _ := strconv.ParseInt(s, 10, 64)
	return time.UnixMilli(v).UTC()
}

func dec(s string) decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return d
}
