package binance

import (
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
	"sync"
	"sync/atomic"
	"time"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/exchange"
)

const recvWindow = "5000"

// Trader places orders and streams account events on Binance spot and
// USDⓈ-M futures. It implements broker.Broker, broker.BracketPlacer and
// broker.LeverageSetter.
type Trader struct {
	c       *Client
	key     string
	secret  []byte
	markets []domain.Market
	log     *slog.Logger

	timeOffset atomic.Int64 // server time - local time, ms

	mu      sync.Mutex
	handler broker.Handler
	// Futures conditional orders live in the algo service; when one
	// triggers, the exchange creates a regular order we must map back.
	algos        map[string]*algoState // by clientAlgoId
	triggered    map[int64]string      // matching-engine order id -> client id
	pendingFills map[int64][]pendingFill
	// orders we placed and still consider open, for event mapping
	open map[string]domain.Order

	resync chan struct{}
}

type algoState struct {
	order   domain.Order
	orderID int64 // matching-engine order id once triggered
}

type pendingFill struct {
	ev broker.Event
	at time.Time
}

var _ broker.Live = (*Trader)(nil)

// NewTrader returns a trader for the given markets using c's environment.
func NewTrader(c *Client, apiKey, apiSecret string, markets []domain.Market) *Trader {
	return &Trader{
		c: c, key: apiKey, secret: []byte(apiSecret), markets: markets, log: c.log,
		algos:        map[string]*algoState{},
		triggered:    map[int64]string{},
		pendingFills: map[int64][]pendingFill{},
		open:         map[string]domain.Order{},
		resync:       make(chan struct{}, 1),
	}
}

func (t *Trader) Exchange() string { return t.c.name }

func (t *Trader) SetHandler(h broker.Handler) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.handler = h
}

// Resync signals (coalesced) that account events may have been missed,
// e.g. after a user stream reconnect. The owner should reconcile.
func (t *Trader) Resync() <-chan struct{} { return t.resync }

func (t *Trader) requestResync() {
	select {
	case t.resync <- struct{}{}:
	default:
	}
}

func (t *Trader) emit(ev broker.Event) {
	t.mu.Lock()
	h := t.handler
	t.mu.Unlock()
	if h != nil {
		h(ev)
	}
}

func (t *Trader) sign(payload string) string {
	m := hmac.New(sha256.New, t.secret)
	_, _ = m.Write([]byte(payload))
	return hex.EncodeToString(m.Sum(nil))
}

func (t *Trader) now() int64 { return time.Now().UnixMilli() + t.timeOffset.Load() }

// SyncTime measures the offset between local and server clocks.
func (t *Trader) SyncTime(ctx context.Context) error {
	var r struct {
		ServerTime int64 `json:"serverTime"`
	}
	m := t.markets[0]
	path := "/api/v3/time"
	if m == domain.Futures {
		path = "/fapi/v1/time"
	}
	start := time.Now()
	if err := t.c.get(ctx, m, path, nil, 1, &r); err != nil {
		return fmt.Errorf("server time: %w", err)
	}
	rtt := time.Since(start)
	local := start.Add(rtt / 2).UnixMilli()
	t.timeOffset.Store(r.ServerTime - local)
	if off := time.Duration(r.ServerTime-local) * time.Millisecond; off.Abs() > time.Second {
		t.log.Warn("local clock differs from exchange", "offset", off)
	}
	return nil
}

// signed performs a signed request (SIGNED/TRADE/USER_DATA endpoints).
func (t *Trader) signed(ctx context.Context, m domain.Market, method, path string, q url.Values, weight int, out any) error {
	if q == nil {
		q = url.Values{}
	}
	q.Set("timestamp", strconv.FormatInt(t.now(), 10))
	q.Set("recvWindow", recvWindow)
	payload := q.Encode()
	return t.do(ctx, m, method, path, payload+"&signature="+t.sign(payload), weight, out)
}

// keyed performs a request that needs the API key but no signature
// (futures listen key).
func (t *Trader) keyed(ctx context.Context, m domain.Market, method, path string, weight int, out any) error {
	return t.do(ctx, m, method, path, "", weight, out)
}

func (t *Trader) do(ctx context.Context, m domain.Market, method, path, query string, weight int, out any) error {
	base, lim := t.c.restBase(m)
	if err := lim.Wait(ctx, weight); err != nil {
		return err
	}
	u := base + path
	if query != "" {
		u += "?" + query
	}
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-MBX-APIKEY", t.key)
	resp, err := t.c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s %s: %w", t.c.name, method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		apiErr := &exchange.APIError{Exchange: t.c.name, HTTPStatus: resp.StatusCode}
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
		}
		// -1021: timestamp outside recvWindow; resync the clock.
		if apiErr.Code == -1021 {
			go func() { _ = t.SyncTime(context.Background()) }()
		}
		return apiErr
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%s %s %s: decode: %w", t.c.name, method, path, err)
	}
	return nil
}

// apiCode returns the Binance error code of err, or 0.
func apiCode(err error) int {
	var e *exchange.APIError
	if errors.As(err, &e) {
		return e.Code
	}
	return 0
}
