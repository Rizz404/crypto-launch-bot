// Package binance implements the exchange interfaces for Binance spot and
// USDⓈ-M futures.
package binance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/exchange"
)

// Endpoints are the base URLs of one environment (mainnet or testnet).
type Endpoints struct {
	SpotREST    string
	FuturesREST string
	SpotWS      string // combined streams are served at SpotWS + "/stream"
	FuturesWS   string
}

var (
	Mainnet = Endpoints{
		SpotREST:    "https://api.binance.com",
		FuturesREST: "https://fapi.binance.com",
		SpotWS:      "wss://stream.binance.com:9443",
		// The legacy wss://fstream.binance.com/stream still accepts
		// connections but no longer sends market data.
		FuturesWS: "wss://fstream.binance.com/market",
	}
	Testnet = Endpoints{
		SpotREST:    "https://testnet.binance.vision",
		FuturesREST: "https://testnet.binancefuture.com",
		SpotWS:      "wss://stream.testnet.binance.vision",
		FuturesWS:   "wss://fstream.binancefuture.com",
	}
)

// Request weight budgets per minute. Official limits are 6000 (spot) and
// 2400 (futures); keep headroom for other clients on the same IP.
const (
	spotWeightPerMin    = 4800
	futuresWeightPerMin = 1900
	klineLimit          = 1000
)

type Options struct {
	Testnet    bool
	Endpoints  *Endpoints   // overrides the Testnet choice, for tests
	HTTPClient *http.Client // optional
	Log        *slog.Logger // optional
}

type Client struct {
	name       string
	ep         Endpoints
	http       *http.Client
	log        *slog.Logger
	spotLim    *exchange.WeightLimiter
	futuresLim *exchange.WeightLimiter
}

var _ exchange.MarketData = (*Client)(nil)

func New(opts Options) *Client {
	c := &Client{
		name:       "binance",
		ep:         Mainnet,
		http:       opts.HTTPClient,
		log:        opts.Log,
		spotLim:    exchange.NewWeightLimiter(spotWeightPerMin),
		futuresLim: exchange.NewWeightLimiter(futuresWeightPerMin),
	}
	if opts.Testnet {
		c.name, c.ep = "binance-testnet", Testnet
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

func (c *Client) restBase(m domain.Market) (string, *exchange.WeightLimiter) {
	if m == domain.Futures {
		return c.ep.FuturesREST, c.futuresLim
	}
	return c.ep.SpotREST, c.spotLim
}

// get performs a public GET request and decodes the JSON body into out.
func (c *Client) get(ctx context.Context, m domain.Market, path string, q url.Values, weight int, out any) error {
	base, lim := c.restBase(m)
	if err := lim.Wait(ctx, weight); err != nil {
		return err
	}
	u := base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s GET %s: %w", c.name, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		apiErr := &exchange.APIError{Exchange: c.name, HTTPStatus: resp.StatusCode}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		var e struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		if json.Unmarshal(body, &e) == nil && e.Msg != "" {
			apiErr.Code, apiErr.Message = e.Code, e.Msg
		} else {
			apiErr.Message = string(body)
		}
		if apiErr.RateLimited() {
			apiErr.RetryAfter = retryAfter(resp.Header)
			lim.Pause(apiErr.RetryAfter)
			c.log.Error("rate limited by exchange", "path", path, "status", resp.StatusCode, "retry_after", apiErr.RetryAfter)
		}
		return apiErr
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("%s GET %s: decode: %w", c.name, path, err)
	}
	return nil
}

func retryAfter(h http.Header) time.Duration {
	if s, err := strconv.Atoi(h.Get("Retry-After")); err == nil && s > 0 {
		return time.Duration(s) * time.Second
	}
	return time.Minute
}

// IsInvalidSymbol reports whether err is Binance's "Invalid symbol" error.
func IsInvalidSymbol(err error) bool {
	var apiErr *exchange.APIError
	return errors.As(err, &apiErr) && apiErr.Code == -1121
}
