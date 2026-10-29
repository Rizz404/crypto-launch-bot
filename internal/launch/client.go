package launch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/rizz404/trading-bot/internal/catalyst"
	"golang.org/x/time/rate"
)

const rpcURL = "https://bsc-dataseed.bnbchain.org"
const maxBody = 4 << 20

type rawRecord struct {
	URL     string          `json:"url"`
	At      time.Time       `json:"at"`
	Request json.RawMessage `json:"request,omitempty"`
	Status  int             `json:"http_status"`
	Body    string          `json:"body"`
	Error   string          `json:"error,omitempty"`
}

type Client struct {
	Archive  catalyst.Archive
	HTTP     *http.Client
	limiter  *rate.Limiter
	requests int
}

func NewClient(archive catalyst.Archive) *Client {
	return &Client{Archive: archive, HTTP: catalyst.DiscoveryClient(), limiter: rate.NewLimiter(rate.Every(2100*time.Millisecond), 1)}
}

func (c *Client) fetch(ctx context.Context, target string, request any, out any) (string, error) {
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" || !slices.Contains([]string{"api.dexscreener.com", "api.gopluslabs.io", "data-api.binance.vision", "bsc-dataseed.bnbchain.org"}, u.Host) {
		return "", errors.New("API origin outside discovery allowlist")
	}
	if c.requests >= 160 {
		return "", errors.New("collection request budget exhausted")
	}
	c.requests++
	if err = c.limiter.Wait(ctx); err != nil {
		return "", errors.New("collection canceled before request")
	}
	record := rawRecord{URL: target, At: time.Now().UTC()}
	method := http.MethodGet
	var body []byte
	if request != nil {
		method = http.MethodPost
		body, err = json.Marshal(request)
		if err != nil {
			return "", err
		}
		record.Request = body
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return "", errors.New("invalid discovery request")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "crypto-launch-bot-research/1.0")
	if request != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, fetchErr := c.HTTP.Do(req)
	if fetchErr != nil {
		record.Error = "network_request_failed"
	} else {
		record.Status = resp.StatusCode
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
		_ = resp.Body.Close()
		if readErr != nil || len(data) > maxBody {
			record.Error = "response_unreadable_or_oversized"
		} else {
			record.Body = string(data)
			if resp.StatusCode != http.StatusOK {
				record.Error = fmt.Sprintf("HTTP_%d", resp.StatusCode)
			}
		}
	}
	id, err := c.Archive.Put("launch-raw", record)
	if err != nil {
		return "", err
	}
	if record.Error != "" {
		return id, errors.New(record.Error)
	}
	if err = json.Unmarshal([]byte(record.Body), out); err != nil {
		return id, errors.New("invalid upstream JSON")
	}
	return id, nil
}

func (c *Client) rpc(ctx context.Context, method string, params any, out any) (string, error) {
	var reply struct {
		ID      int             `json:"id"`
		JSONRPC string          `json:"jsonrpc"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
	}
	id, err := c.fetch(ctx, rpcURL, map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params}, &reply)
	if err != nil {
		return id, err
	}
	if reply.ID != 1 || reply.JSONRPC != "2.0" || (len(reply.Error) > 0 && string(reply.Error) != "null") || len(reply.Result) == 0 || string(reply.Result) == "null" {
		return id, errors.New("RPC error or missing result")
	}
	if err = json.Unmarshal(reply.Result, out); err != nil {
		return id, errors.New("invalid RPC result")
	}
	return id, nil
}
