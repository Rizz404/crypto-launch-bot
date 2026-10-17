package backtest

import (
	"encoding/json"
	"math"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/domain"
)

// Metrics summarise a backtest. Ratios are fractions (0.1 = 10%).
type Metrics struct {
	Start, End     time.Time
	InitialEquity  float64
	FinalEquity    float64
	TotalReturn    float64
	CAGR           float64
	MaxDrawdown    float64
	Sharpe         float64 // annualised from daily returns, 365 days
	Sortino        float64
	Trades         int
	WinRate        float64
	ProfitFactor   float64 // gross profit / gross loss (net of costs); +Inf with no losses
	AvgTrade       float64 // average net PnL per trade
	AvgWin         float64
	AvgLoss        float64
	TotalFees      float64
	TotalFunding   float64
	AvgHoldingTime time.Duration
}

// Compute derives metrics from the equity curve and closed trades.
func Compute(initial decimal.Decimal, equity []EquityPoint, trades []domain.Trade) Metrics {
	m := Metrics{}
	m.InitialEquity, _ = initial.Float64()
	if len(equity) == 0 || m.InitialEquity <= 0 {
		return m
	}
	m.Start, m.End = equity[0].Time, equity[len(equity)-1].Time
	m.FinalEquity, _ = equity[len(equity)-1].Equity.Float64()
	m.TotalReturn = m.FinalEquity/m.InitialEquity - 1
	if years := m.End.Sub(m.Start).Hours() / 24 / 365; years > 0 && m.FinalEquity > 0 {
		m.CAGR = math.Pow(m.FinalEquity/m.InitialEquity, 1/years) - 1
	}

	peak := m.InitialEquity
	for _, p := range equity {
		v, _ := p.Equity.Float64()
		peak = math.Max(peak, v)
		if peak > 0 {
			m.MaxDrawdown = math.Max(m.MaxDrawdown, (peak-v)/peak)
		}
	}

	m.Sharpe, m.Sortino = ratios(dailyReturns(m.InitialEquity, equity))

	var grossWin, grossLoss, hold float64
	var wins, losses int
	for _, t := range trades {
		pnl, _ := t.NetPnL().Float64()
		fees, _ := t.Fees.Float64()
		funding, _ := t.Funding.Float64()
		m.TotalFees += fees
		m.TotalFunding += funding
		hold += t.ClosedAt.Sub(t.OpenedAt).Seconds()
		if pnl > 0 {
			wins++
			grossWin += pnl
		} else {
			losses++
			grossLoss -= pnl
		}
	}
	m.Trades = len(trades)
	if m.Trades > 0 {
		m.WinRate = float64(wins) / float64(m.Trades)
		m.AvgTrade = (grossWin - grossLoss) / float64(m.Trades)
		m.AvgHoldingTime = time.Duration(hold/float64(m.Trades)) * time.Second
	}
	if wins > 0 {
		m.AvgWin = grossWin / float64(wins)
	}
	if losses > 0 {
		m.AvgLoss = -grossLoss / float64(losses)
	}
	switch {
	case grossLoss > 0:
		m.ProfitFactor = grossWin / grossLoss
	case grossWin > 0:
		m.ProfitFactor = math.Inf(1)
	}
	return m
}

// dailyReturns samples equity at each UTC day end.
func dailyReturns(initial float64, equity []EquityPoint) []float64 {
	var out []float64
	prev := initial
	day := equity[0].Time.UTC().Truncate(24 * time.Hour)
	last := prev
	for _, p := range equity {
		d := p.Time.UTC().Truncate(24 * time.Hour)
		if d.After(day) {
			if prev > 0 {
				out = append(out, last/prev-1)
			}
			prev, day = last, d
		}
		last, _ = p.Equity.Float64()
	}
	if prev > 0 {
		out = append(out, last/prev-1)
	}
	return out
}

func ratios(rets []float64) (sharpe, sortino float64) {
	if len(rets) < 2 {
		return 0, 0
	}
	var sum float64
	for _, r := range rets {
		sum += r
	}
	mean := sum / float64(len(rets))
	var variance, downside float64
	for _, r := range rets {
		variance += (r - mean) * (r - mean)
		if r < 0 {
			downside += r * r
		}
	}
	sd := math.Sqrt(variance / float64(len(rets)-1))
	dd := math.Sqrt(downside / float64(len(rets)))
	ann := math.Sqrt(365)
	if sd > 0 {
		sharpe = mean / sd * ann
	}
	if dd > 0 {
		sortino = mean / dd * ann
	}
	return sharpe, sortino
}

// MarshalJSON writes an infinite profit factor (no losing trades) as null,
// since JSON has no infinity.
func (m Metrics) MarshalJSON() ([]byte, error) {
	type plain Metrics
	out := struct {
		plain
		ProfitFactor *float64
	}{plain: plain(m)}
	if !math.IsInf(m.ProfitFactor, 0) && !math.IsNaN(m.ProfitFactor) {
		out.ProfitFactor = &m.ProfitFactor
	}
	return json.Marshal(out)
}
