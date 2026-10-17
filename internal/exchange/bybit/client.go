// Package bybit implements the exchange interfaces for Bybit v5 (unified
// trading account): spot and USDT linear perpetuals.
package bybit

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/exchange"
)

type Endpoints struct {
	REST      string
	PublicWS  string // + "/spot" or "/linear"
	PrivateWS string
}

var (
	Mainnet = Endpoints{REST: "https://api.bybit.com", PublicWS: "wss://stream.bybit.com/v5/public", PrivateWS: "wss://stream.bybit.com/v5/private"}
	Testnet = Endpoints{REST: "https://api-testnet.bybit.com", PublicWS: "wss://stream-testnet.bybit.com/v5/public", PrivateWS: "wss://stream-testnet.bybit.com/v5/private"}
)

const (
	// Bybit allows 600 requests per 5 seconds per IP; stay well below.
	requestsPerMin = 3000
	recvWindow     = "5000"
	klineLimit     = 1000
)

type Options struct {
	Testnet    bool
	Endpoints  *Endpoints
	HTTPClient *http.Client
	Log        *slog.Logger
}

type Client struct {
	name       string
	ep         Endpoints
	http       *http.Client
	log        *slog.Logger
	lim        *exchange.WeightLimiter
	timeOffset atomic.Int64 // ms
	key        string
	secret     []byte
}

var _ exchange.MarketData = (*Client)(nil)

func New(opts Options) *Client {
	c := &Client{name: "bybit", ep: Mainnet, http: opts.HTTPClient, log: opts.Log, lim: exchange.NewWeightLimiter(requestsPerMin)}
	if opts.Testnet {
		c.name, c.ep = "bybit-testnet", Testnet
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

func category(m domain.Market) string {
	if m == domain.Futures {
		return "linear"
	}
	return "spot"
}

type envelope struct {
	RetCode int             `json:"retCode"`
	RetMsg  string          `json:"retMsg"`
	Result  json.RawMessage `json:"result"`
	Time    int64           `json:"time"`
}

// call performs a request; signed requests need the API key. GET params
// go in q, POST params in body.
func (c *Client) call(ctx context.Context, method, path string, q url.Values, body any, signed bool, out any) error {
	if err := c.lim.Wait(ctx, 1); err != nil {
		return err
	}
	u := c.ep.REST + path
	query := ""
	if len(q) > 0 {
		query = q.Encode()
		u += "?" + query
	}
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if signed {
		ts := strconv.FormatInt(time.Now().UnixMilli()+c.timeOffset.Load(), 10)
		signPayload := query
		if body != nil {
			signPayload = string(payload)
		}
		m := hmac.New(sha256.New, c.secret)
		_, _ = m.Write([]byte(ts + c.key + recvWindow + signPayload))
		req.Header.Set("X-BAPI-API-KEY", c.key)
		req.Header.Set("X-BAPI-TIMESTAMP", ts)
		req.Header.Set("X-BAPI-RECV-WINDOW", recvWindow)
		req.Header.Set("X-BAPI-SIGN", hex.EncodeToString(m.Sum(nil)))
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
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusForbidden {
		c.lim.Pause(time.Minute)
		return &exchange.APIError{Exchange: c.name, HTTPStatus: resp.StatusCode, Message: string(raw), RetryAfter: time.Minute}
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("%s %s %s: http %d: %w", c.name, method, path, resp.StatusCode, err)
	}
	if env.RetCode != 0 {
		apiErr := &exchange.APIError{Exchange: c.name, HTTPStatus: resp.StatusCode, Code: env.RetCode, Message: env.RetMsg}
		if env.RetCode == 10006 { // too many visits
			apiErr.RetryAfter = 5 * time.Second
			c.lim.Pause(apiErr.RetryAfter)
		}
		return apiErr
	}
	if out != nil {
		if err := json.Unmarshal(env.Result, out); err != nil {
			return fmt.Errorf("%s %s %s: decode: %w", c.name, method, path, err)
		}
	}
	return nil
}

func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	return c.call(ctx, http.MethodGet, path, q, nil, false, out)
}

// SyncTime measures the offset between local and server clocks.
func (c *Client) SyncTime(ctx context.Context) error {
	var r struct {
		TimeNano string `json:"timeNano"`
	}
	start := time.Now()
	if err := c.get(ctx, "/v5/market/time", nil, &r); err != nil {
		return err
	}
	ns, err := strconv.ParseInt(r.TimeNano, 10, 64)
	if err != nil {
		return err
	}
	local := start.Add(time.Since(start) / 2)
	c.timeOffset.Store(ns/1e6 - local.UnixMilli())
	return nil
}

func apiCode(err error) int {
	var e *exchange.APIError
	if errors.As(err, &e) {
		return e.Code
	}
	return 0
}
