package main

import (
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestCSVRejectsGapsInvalidOHLCAndDuplicateTimes(t *testing.T) {
	for _, tc := range []struct {
		rows string
		gap  bool
		ok   bool
	}{
		{"open_time,open,high,low,close,volume\n1697673600,100,102,99,101,1\n1697673660,101,102,100,101,1", false, true},
		{"invalid,100,102,99,101,1", false, false},
		{"1697673600,100,99,99,101,1", false, false},
		{"1697673600,100,102,99,101,1\n1697673720,101,102,100,101,1", false, false},
		{"1697673600,100,102,99,101,1\n1697673720,101,102,100,101,1", true, true},
		{"1697673600,100,102,99,101,1\n1697673600,101,102,100,101,1", true, false},
	} {
		i := input{reader: csv.NewReader(strings.NewReader(tc.rows)), symbol: "ETHUSDT"}
		var err error
		for {
			err = i.advance(tc.gap)
			if err != nil || i.next == nil {
				break
			}
		}
		if tc.ok {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}
}

func TestAggregateRequiresCompleteClosedTimeframe(t *testing.T) {
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	bar := func(n int) domain.Candle {
		return domain.Candle{OpenTime: start.Add(time.Duration(n) * time.Minute), Timeframe: "1m", Open: decimal.NewFromInt(100), High: decimal.NewFromInt(102), Low: decimal.NewFromInt(99), Close: decimal.NewFromInt(101), Volume: decimal.NewFromInt(1), Closed: true}
	}
	a := aggregate{}
	for n := 0; n < 60; n++ {
		c, ok := a.push(bar(n), "1h")
		require.Equal(t, n == 59, ok)
		if ok {
			require.Equal(t, "60", c.Volume.String())
			require.Equal(t, start, c.OpenTime)
		}
	}
	a = aggregate{}
	for n := 1; n < 60; n++ {
		_, ok := a.push(bar(n), "1h")
		require.False(t, ok, "a partial starting hour must not create a signal candle")
	}
}
