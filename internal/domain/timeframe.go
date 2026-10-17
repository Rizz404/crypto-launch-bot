package domain

import (
	"fmt"
	"time"
)

// Timeframe is a candle interval using exchange-style notation ("1m", "4h").
// Monthly bars are not supported because their length varies.
type Timeframe string

var timeframes = map[Timeframe]time.Duration{
	"1m":  time.Minute,
	"3m":  3 * time.Minute,
	"5m":  5 * time.Minute,
	"15m": 15 * time.Minute,
	"30m": 30 * time.Minute,
	"1h":  time.Hour,
	"2h":  2 * time.Hour,
	"4h":  4 * time.Hour,
	"6h":  6 * time.Hour,
	"8h":  8 * time.Hour,
	"12h": 12 * time.Hour,
	"1d":  24 * time.Hour,
	"3d":  3 * 24 * time.Hour,
	"1w":  7 * 24 * time.Hour,
}

// ParseTimeframe validates s.
func ParseTimeframe(s string) (Timeframe, error) {
	tf := Timeframe(s)
	if _, ok := timeframes[tf]; !ok {
		return "", fmt.Errorf("unsupported timeframe %q", s)
	}
	return tf, nil
}

// Duration returns the bar length, or 0 for an invalid timeframe.
func (tf Timeframe) Duration() time.Duration { return timeframes[tf] }

// Truncate returns the open time of the bar containing t.
// time.Truncate counts from the zero time (Monday, year 1), so weekly bars
// open on Monday 00:00 UTC, matching Binance.
func (tf Timeframe) Truncate(t time.Time) time.Time {
	return t.UTC().Truncate(tf.Duration())
}
