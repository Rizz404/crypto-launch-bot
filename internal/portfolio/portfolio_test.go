package portfolio

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rizz404/trading-bot/internal/domain"
)

func TestLedgerPartialCloseFlipAndFunding(t *testing.T) {
	d := decimal.RequireFromString
	key := AccountKey{Exchange: "sim", Market: domain.Futures}
	var trades []domain.Trade
	a := NewAccount(key, d("1000"), func(tr domain.Trade) { trades = append(trades, tr) })
	fill := func(side domain.Side, qty, price, fee string) {
		a.ApplyFill(domain.Fill{Exchange: key.Exchange, Market: key.Market, Symbol: "BTCUSDT", Side: side,
			Quantity: d(qty), Price: d(price), Fee: d(fee), Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
			FillMeta{Strategy: "test", Leverage: 2, Tag: domain.TagExit})
	}
	fill(domain.Buy, "2", "100", "2")
	fill(domain.Buy, "2", "120", "2")
	p, ok := a.Position("BTCUSDT")
	require.True(t, ok)
	assert.Equal(t, "110", p.EntryPrice.String())
	a.Mark("BTCUSDT", d("125"))
	s := a.Snapshot()
	assert.Equal(t, "1056", s.Equity.String())
	assert.Equal(t, "250", s.UsedMargin.String())
	assert.Equal(t, "806", s.FreeMargin().String())
	assert.Equal(t, "5", a.ApplyFunding("BTCUSDT", d("0.01"), time.Time{}).String())
	fill(domain.Sell, "1", "130", "1")
	require.Len(t, trades, 1)
	assert.Equal(t, "20", trades[0].GrossPnL.String())
	assert.Equal(t, "2", trades[0].Fees.String())
	assert.Equal(t, "1.25", trades[0].Funding.String())
	assert.Equal(t, "16.75", trades[0].NetPnL().String())
	fill(domain.Sell, "5", "90", "5") // closes remaining long 3, opens short 2
	require.Len(t, trades, 2)
	assert.Equal(t, "-60", trades[1].GrossPnL.String())
	assert.Equal(t, "6", trades[1].Fees.String())
	assert.Equal(t, "3.75", trades[1].Funding.String())
	p, ok = a.Position("BTCUSDT")
	require.True(t, ok)
	assert.Equal(t, domain.Short, p.Side)
	assert.Equal(t, "2", p.Quantity.String())
	assert.Equal(t, "2", p.Fees.String())
	assert.Equal(t, "945", a.Snapshot().Balance.String())
	assert.Equal(t, "-1.8", a.ApplyFunding("BTCUSDT", d("0.01"), time.Time{}).String())
	fill(domain.Buy, "2", "80", "2")
	assert.Empty(t, a.Positions())
	assert.Equal(t, "964.8", a.Snapshot().Balance.String())
	assert.Equal(t, "964.8", a.Snapshot().Equity.String())
	assert.Equal(t, "16.75", trades[0].NetPnL().String(), "closed trade is an immutable snapshot")
}

func TestBookAndPositionCopies(t *testing.T) {
	d := decimal.NewFromInt
	b := NewBook()
	a := NewAccount(AccountKey{"b", domain.Spot}, d(100), nil)
	p := domain.Position{PositionKey: domain.PositionKey{Exchange: "b", Market: domain.Spot, Symbol: "BTCUSDT"},
		Side: domain.Long, Quantity: d(2), EntryPrice: d(10), Leverage: 1}
	a.SetPosition(p)
	b.Add(a)
	b.Add(NewAccount(AccountKey{"a", domain.Futures}, d(200), nil))
	p.Quantity = d(100)
	copyPos, _ := a.Position("BTCUSDT")
	assert.Equal(t, "2", copyPos.Quantity.String())
	assert.Equal(t, "a", b.Accounts()[0].Key.Exchange)
	assert.Equal(t, "300", b.Total().Equity.String())
	assert.Equal(t, 1, b.Total().OpenPositions)
	copyPos.Quantity = decimal.Zero
	a.SetPosition(copyPos)
	assert.Empty(t, a.Positions())
}
