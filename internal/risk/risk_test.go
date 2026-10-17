package risk

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/portfolio"
)

func TestSizeCapsRoundsAndRejectsMinimum(t *testing.T) {
	d := decimal.RequireFromString
	m := NewManager(DefaultConfig(), nil)
	r := SizeRequest{Equity: d("1000"), FreeMargin: d("1000"), Price: d("100"), StopLoss: d("95"), Leverage: 1,
		Instrument: domain.Instrument{StepSize: d("0.01"), MinQty: d("0.01"), MinNotional: d("5")}}
	for _, tc := range []struct {
		name string
		edit func(*SizeRequest)
		want string
	}{
		{"stop risk", func(*SizeRequest) {}, "2"},
		{"fallback", func(r *SizeRequest) { r.StopLoss = decimal.Zero }, "1"},
		{"equity cap", func(r *SizeRequest) { r.Quantity = d("100") }, "2.5"},
		{"margin buffer and lot", func(r *SizeRequest) { r.FreeMargin = d("11") }, "0.1"},
		{"leverage capped", func(r *SizeRequest) { r.Leverage = 100; r.StopLoss = decimal.Zero }, "3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cp := r
			tc.edit(&cp)
			q, err := m.Size(cp)
			require.NoError(t, err)
			assert.Equal(t, tc.want, q.String())
		})
	}
	r.FreeMargin = d("1")
	_, err := m.Size(r)
	assert.ErrorIs(t, err, ErrTooSmall)
	r.FreeMargin = decimal.Zero
	_, err = m.Size(r)
	assert.ErrorContains(t, err, "free margin")
	r.Price = decimal.Zero
	_, err = m.Size(r)
	assert.ErrorContains(t, err, "price")
	assert.Equal(t, 1, m.Leverage(10, domain.Spot))
}

func TestKillSwitchUTCDayDrawdownAndReset(t *testing.T) {
	d := decimal.NewFromInt
	m := NewManager(DefaultConfig(), nil)
	now := time.Date(2026, 1, 1, 23, 0, 0, 0, time.UTC)
	tripped, _ := m.Observe(now, d(1000))
	assert.False(t, tripped)
	tripped, reason := m.Observe(now.Add(time.Minute), d(950))
	assert.True(t, tripped)
	assert.Contains(t, reason, "daily loss")
	tripped, _ = m.Observe(now.Add(2*time.Minute), d(900))
	assert.False(t, tripped, "trip only once")
	m.Reset(d(900))
	tripped, _ = m.Observe(now.Add(time.Hour), d(860))
	assert.False(t, tripped, "new UTC day starts at the observed equity")
	m.Halt("manual")
	halted, reason := m.Halted()
	assert.True(t, halted)
	assert.Equal(t, "manual", reason)
	cfg := DefaultConfig()
	cfg.DailyLossLimit = 0
	m = NewManager(cfg, nil)
	m.Observe(now, d(1000))
	m.Observe(now, d(1200))
	tripped, reason = m.Observe(now, d(960))
	assert.True(t, tripped)
	assert.Contains(t, reason, "drawdown")
}

func TestOrderLimits(t *testing.T) {
	d := decimal.NewFromInt
	cfg := DefaultConfig()
	cfg.MaxOpenPositions = 1
	m := NewManager(cfg, nil)
	b := portfolio.NewBook()
	a := portfolio.NewAccount(portfolio.AccountKey{Exchange: "sim", Market: domain.Futures}, d(1000), nil)
	a.SetPosition(domain.Position{PositionKey: domain.PositionKey{Exchange: "sim", Market: domain.Futures, Symbol: "BTCUSDT"},
		Side: domain.Long, Quantity: d(1), EntryPrice: d(100), Leverage: 1})
	b.Add(a)
	o := domain.Order{Exchange: "sim", Market: domain.Futures, Symbol: "ETHUSDT", Tag: domain.TagEntry, Quantity: d(1)}
	assert.ErrorContains(t, m.CheckOrder(o, d(100), b), "max open positions")
	o.Symbol = "BTCUSDT"
	assert.NoError(t, m.CheckOrder(o, d(100), b), "adding to an existing position is permitted")
	o.Quantity = d(20)
	assert.ErrorContains(t, m.CheckOrder(o, d(100), b), "exposure")
	m.Halt("manual")
	assert.ErrorContains(t, m.CheckOrder(o, d(100), b), "kill switch")
}
