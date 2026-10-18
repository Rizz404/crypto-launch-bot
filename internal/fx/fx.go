// Package fx keeps the value of one USDT in US dollars and Indonesian rupiah,
// so notifications can show profits and losses in money the user thinks in.
// It only informs messages; trading never depends on it.
package fx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

// Rates is the value of one USDT.
type Rates struct {
	USD    decimal.Decimal
	IDR    decimal.Decimal
	Source string
	At     time.Time
}

// Default public endpoints (no API key). CoinGecko prices USDT itself;
// Frankfurter (ECB reference rates) is the fallback, assuming 1 USDT = 1 USD.
const (
	CoinGeckoURL   = "https://api.coingecko.com/api/v3/simple/price?ids=tether&vs_currencies=usd,idr"
	FrankfurterURL = "https://api.frankfurter.dev/v1/latest?base=USD&symbols=IDR"
)

// Service refreshes Rates periodically and keeps the last good value.
type Service struct {
	CoinGecko, Frankfurter string
	Interval               time.Duration
	HTTP                   *http.Client
	Log                    *slog.Logger
	Clock                  func() time.Time

	mu    sync.Mutex
	rates Rates
	ok    bool
}

func New(log *slog.Logger) *Service {
	return &Service{CoinGecko: CoinGeckoURL, Frankfurter: FrankfurterURL, Interval: time.Hour,
		HTTP: &http.Client{Timeout: 15 * time.Second}, Log: log, Clock: time.Now}
}

// Rates returns the last good rates; ok is false until one fetch succeeded.
func (s *Service) Rates() (Rates, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rates, s.ok
}

// Run refreshes immediately and then every Interval until ctx is cancelled.
func (s *Service) Run(ctx context.Context) {
	t := time.NewTicker(s.Interval)
	defer t.Stop()
	for {
		if err := s.Refresh(ctx); err != nil && ctx.Err() == nil {
			s.Log.Warn("exchange rates unavailable, keeping the last ones", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Refresh fetches new rates, trying CoinGecko first.
func (s *Service) Refresh(ctx context.Context) error {
	r, err := s.coinGecko(ctx)
	if err != nil {
		var ferr error
		if r, ferr = s.frankfurter(ctx); ferr != nil {
			return errors.Join(err, ferr)
		}
	}
	r.At = s.Clock()
	s.mu.Lock()
	s.rates, s.ok = r, true
	s.mu.Unlock()
	return nil
}

func (s *Service) coinGecko(ctx context.Context) (Rates, error) {
	var body struct {
		Tether struct {
			USD decimal.Decimal `json:"usd"`
			IDR decimal.Decimal `json:"idr"`
		} `json:"tether"`
	}
	if err := s.get(ctx, s.CoinGecko, &body); err != nil {
		return Rates{}, fmt.Errorf("coingecko: %w", err)
	}
	if !body.Tether.USD.IsPositive() || !body.Tether.IDR.IsPositive() {
		return Rates{}, errors.New("coingecko: missing tether price")
	}
	return Rates{USD: body.Tether.USD, IDR: body.Tether.IDR, Source: "CoinGecko"}, nil
}

func (s *Service) frankfurter(ctx context.Context) (Rates, error) {
	var body struct {
		Rates struct {
			IDR decimal.Decimal `json:"IDR"`
		} `json:"rates"`
	}
	if err := s.get(ctx, s.Frankfurter, &body); err != nil {
		return Rates{}, fmt.Errorf("frankfurter: %w", err)
	}
	if !body.Rates.IDR.IsPositive() {
		return Rates{}, errors.New("frankfurter: missing IDR rate")
	}
	return Rates{USD: decimal.NewFromInt(1), IDR: body.Rates.IDR, Source: "ECB, 1 USDT ≈ 1 USD"}, nil
}

func (s *Service) get(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
}
