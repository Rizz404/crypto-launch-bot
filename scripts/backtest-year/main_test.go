package main

import (
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/shopspring/decimal"
	"testing"
	"time"
)

func TestAggregateUsesClosedAlignedBars(t *testing.T) {
	start := time.Date(2025, 10, 19, 0, 0, 0, 0, time.UTC)
	bars := make([]domain.Candle, 240)
	for i := range bars {
		p := decimal.NewFromInt(int64(i + 100))
		bars[i] = domain.Candle{Timeframe: "1m", OpenTime: start.Add(time.Duration(i) * time.Minute), Open: p, High: p.Add(decimal.NewFromInt(2)), Low: p.Sub(decimal.NewFromInt(2)), Close: p.Add(decimal.NewFromInt(1)), Volume: decimal.NewFromInt(1), QuoteVolume: decimal.NewFromInt(2), Trades: 3, Closed: true}
	}
	got := aggregate(bars, "4h", 240)
	if len(got) != 1 {
		t.Fatal("wrong number of bars")
	}
	c := got[0]
	if !c.Open.Equal(decimal.NewFromInt(100)) || !c.Close.Equal(decimal.NewFromInt(340)) || !c.High.Equal(decimal.NewFromInt(341)) || !c.Low.Equal(decimal.NewFromInt(98)) || !c.Volume.Equal(decimal.NewFromInt(240)) || c.Trades != 720 || !c.CloseTime().Equal(start.Add(4*time.Hour)) {
		t.Fatalf("wrong aggregate: %+v", c)
	}
	if !bars[0].Close.Equal(decimal.NewFromInt(101)) {
		t.Fatal("input was mutated")
	}
}
func TestAggregateRejectsPartialOrUnalignedBars(t *testing.T) {
	for _, tc := range []struct {
		n      int
		offset time.Duration
	}{{239, 0}, {240, time.Minute}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("expected rejection")
				}
			}()
			bars := make([]domain.Candle, tc.n)
			bars[0].OpenTime = time.Date(2025, 10, 19, 0, 0, 0, 0, time.UTC).Add(tc.offset)
			aggregate(bars, "4h", 240)
		}()
	}
}
