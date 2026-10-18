// Package marketdata selects the tradable universe, downloads history and
// runs live candle feeds with gap backfill.
package marketdata

import (
	"slices"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/domain"
)

// UniverseRules filter and rank symbols by liquidity.
type UniverseRules struct {
	Quote          string // e.g. USDT
	TopN           int
	MinQuoteVolume decimal.Decimal // 24h volume in quote currency
	MaxSpreadBps   float64         // 0 disables the spread filter
	ExcludeBases   []string        // stablecoins, fiat, wrapped tokens, ...
	ExcludeTags    []string        // asset categories, e.g. "bStocks" (tokenized stocks)
}

// WithTagged returns r with every base asset carrying one of r.ExcludeTags
// added to ExcludeBases.
func (r UniverseRules) WithTagged(tags map[string][]string) UniverseRules {
	r.ExcludeBases = slices.Clone(r.ExcludeBases)
	for base, ts := range tags {
		if slices.ContainsFunc(ts, func(t string) bool {
			return slices.ContainsFunc(r.ExcludeTags, func(x string) bool { return strings.EqualFold(x, t) })
		}) {
			r.ExcludeBases = append(r.ExcludeBases, base)
		}
	}
	return r
}

// Pick is one selected symbol.
type Pick struct {
	Symbol      string
	QuoteVolume decimal.Decimal
	SpreadBps   float64
}

// SelectUniverse returns up to TopN tradable symbols with the highest 24h
// quote volume that pass every filter, most liquid first.
func SelectUniverse(ins []domain.Instrument, tickers []domain.Ticker, r UniverseRules) []Pick {
	bySymbol := make(map[string]domain.Instrument, len(ins))
	for _, in := range ins {
		bySymbol[in.Symbol] = in
	}

	var picks []Pick
	for _, t := range tickers {
		in, ok := bySymbol[t.Symbol]
		if !ok || !in.Tradable || !strings.EqualFold(in.Quote, r.Quote) {
			continue
		}
		if slices.ContainsFunc(r.ExcludeBases, func(b string) bool { return strings.EqualFold(b, in.Base) }) {
			continue
		}
		if t.QuoteVolume.LessThan(r.MinQuoteVolume) {
			continue
		}
		spread, ok := t.SpreadBps()
		if r.MaxSpreadBps > 0 && (!ok || spread > r.MaxSpreadBps) {
			continue
		}
		picks = append(picks, Pick{Symbol: t.Symbol, QuoteVolume: t.QuoteVolume, SpreadBps: spread})
	}

	slices.SortFunc(picks, func(a, b Pick) int {
		if c := b.QuoteVolume.Cmp(a.QuoteVolume); c != 0 {
			return c
		}
		return strings.Compare(a.Symbol, b.Symbol)
	})
	if r.TopN > 0 && len(picks) > r.TopN {
		picks = picks[:r.TopN]
	}
	return picks
}

// Symbols returns the symbol names of picks.
func Symbols(picks []Pick) []string {
	out := make([]string, len(picks))
	for i, p := range picks {
		out[i] = p.Symbol
	}
	return out
}
