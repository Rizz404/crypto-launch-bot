package catalyst

import (
	"context"
	"errors"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/exchange"
	"github.com/rizz404/trading-bot/internal/exchange/binance"
	"github.com/rizz404/trading-bot/internal/indicator"
	"github.com/shopspring/decimal"
)

// Market caches only slow public metadata. Each decision gets a fresh book.
// No trading credentials or order client exist in this adapter.
type Market struct {
	Client      *binance.Client
	instruments map[string]domain.Instrument
	volume      map[string]decimal.Decimal
	atr         map[string]Quote
	refreshed   time.Time
}

func NewMarket(client *binance.Client) *Market {
	return &Market{Client: client, atr: map[string]Quote{}}
}

func (m *Market) Refresh(ctx context.Context, p Policy, now time.Time) ([]domain.Instrument, error) {
	if m.Client == nil || m.Client.Name() != "binance" {
		return nil, errors.New("public Binance mainnet market client required")
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if m.refreshed.IsZero() || now.Sub(m.refreshed) >= 5*time.Minute || now.Before(m.refreshed) {
		ins, err := m.Client.Instruments(ctx, domain.Spot)
		if err != nil {
			return nil, err
		}
		tickers, err := m.Client.Tickers(ctx, domain.Spot)
		if err != nil {
			return nil, err
		}
		m.instruments, m.volume = map[string]domain.Instrument{}, map[string]decimal.Decimal{}
		tags, err := m.Client.AssetTags(ctx)
		if err != nil {
			return nil, errors.New("asset classification unavailable")
		}
		for _, i := range ins {
			stock := slices.ContainsFunc(tags[i.Base], func(s string) bool { return strings.EqualFold(s, "bStocks") })
			if i.Tradable && i.Quote == "USDT" && !stock && slices.Contains(p.Symbols, i.Symbol) {
				m.instruments[i.Symbol] = i
			}
		}
		for _, t := range tickers {
			m.volume[t.Symbol] = t.QuoteVolume
		}
		m.refreshed = now
	}
	var out []domain.Instrument
	for _, sym := range p.Symbols {
		i, ok := m.instruments[sym]
		if !ok || !i.TickSize.IsPositive() || !i.StepSize.IsPositive() || !m.volume[sym].IsPositive() {
			return nil, errors.New("policy symbol lacks tradable Spot instrument or volume")
		}
		out = append(out, i)
	}
	return out, nil
}

func (m *Market) Snapshot(ctx context.Context, p Policy, symbol string) (Quote, error) {
	if !slices.Contains(p.Symbols, symbol) {
		return Quote{}, errors.New("market symbol outside policy")
	}
	now := time.Now().UTC()
	if _, err := m.Refresh(ctx, p, now); err != nil {
		return Quote{}, err
	}
	q := m.atr[symbol]
	closed := now.Truncate(15 * time.Minute)
	if !q.ATRAt.Equal(closed) {
		candles, err := m.Client.Klines(ctx, exchange.KlineRequest{Market: domain.Spot, Symbol: symbol, Timeframe: "15m", Start: closed.Add(-100 * 15 * time.Minute), End: closed.Add(-time.Millisecond), Limit: 100})
		if err != nil {
			return Quote{}, err
		}
		q, err = quoteATR(candles, symbol, now)
		if err != nil {
			return Quote{}, err
		}
		m.atr[symbol] = q
	}
	book, err := m.Client.SpotQuote(ctx, symbol)
	if err != nil {
		return Quote{}, err
	}
	if time.Since(book.At) > DevelopmentEntryPolicy().QuoteTTL || !q.ATRAt.Equal(time.Now().UTC().Truncate(15*time.Minute)) {
		return Quote{}, errors.New("market request crossed freshness boundary")
	}
	q.Symbol, q.Instrument, q.QuoteVolume = symbol, m.instruments[symbol], m.volume[symbol]
	q.Bid, q.Ask, q.BidQty, q.AskQty, q.At = book.Bid, book.Ask, book.BidQty, book.AskQty, book.At
	return q, nil
}

func quoteATR(candles []domain.Candle, symbol string, now time.Time) (Quote, error) {
	if len(candles) < 15 || len(candles) > 100 {
		return Quote{}, errors.New("ATR needs 15–100 consecutive closed candles")
	}
	atr := indicator.NewATR(14)
	var last domain.Candle
	for n, c := range candles {
		if c.Symbol != symbol || c.Exchange != "binance" || c.Market != domain.Spot || c.Timeframe != "15m" || !c.Closed || c.CloseTime().After(now) || !c.OpenTime.Equal(c.OpenTime.Truncate(15*time.Minute)) || !c.Open.IsPositive() || !c.Low.IsPositive() || !c.Close.IsPositive() || c.High.LessThan(c.Low) || c.High.LessThan(decimal.Max(c.Open, c.Close)) || c.Low.GreaterThan(decimal.Min(c.Open, c.Close)) || (n > 0 && !c.OpenTime.Equal(last.CloseTime())) {
			return Quote{}, errors.New("invalid, open or gapped ATR candles")
		}
		h, _ := c.High.Float64()
		l, _ := c.Low.Float64()
		v, _ := c.Close.Float64()
		atr.Update(h, l, v)
		last = c
	}
	if !atr.Ready() || math.IsNaN(atr.Value()) || math.IsInf(atr.Value(), 0) || atr.Value() <= 0 || !last.CloseTime().Equal(now.Truncate(15*time.Minute)) {
		return Quote{}, errors.New("ATR is invalid or stale")
	}
	return Quote{ATR: decimal.NewFromFloat(atr.Value()), ATRAt: last.CloseTime(), EntryAnchor: last.Close}, nil
}
