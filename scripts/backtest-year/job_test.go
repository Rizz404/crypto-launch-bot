package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/risk"
)

func TestRangeSourceFiltersDatesAndRejectsGaps(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "hourly.csv")
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	var all string
	for i := -1; i <= 48; i++ {
		all += fmt.Sprintf("%d,100,102,99,101,1,100,3\n", start.Add(time.Duration(i)*time.Hour).Unix())
	}
	if err := os.WriteFile(file, []byte(all), 0600); err != nil {
		t.Fatal(err)
	}
	src, err := rangeSource(file, "BTCUSDT", domain.Timeframe("1h"), start, start.Add(48*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(src["1h"]) != 48 || len(src["4h"]) != 12 || len(src["1d"]) != 2 {
		t.Fatal("unexpected source coverage")
	}
	var gap string
	for i := 0; i < 48; i++ {
		if i == 20 {
			continue
		}
		gap += fmt.Sprintf("%d,100,102,99,101,1,100,3\n", start.Add(time.Duration(i)*time.Hour).Unix())
	}
	if err := os.WriteFile(file, []byte(gap), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := rangeSource(file, "BTCUSDT", "1h", start, start.Add(48*time.Hour)); err == nil {
		t.Fatal("missing candle must fail")
	}
}
func TestResearchJobRejectsUnboundedRisk(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "job.json")
	for _, change := range []func(*researchJob){func(j *researchJob) { j.Risk.MaxLeverage = 2 }, func(j *researchJob) { j.Risk.RiskPerTrade = .5 }, func(j *researchJob) { j.Risk.MaxExposurePct = 2 }, func(j *researchJob) { j.Risk.MaxDrawdown = 0 }, func(j *researchJob) { j.Fee = 0 }} {
		r := risk.DefaultConfig()
		r.MaxLeverage = 1
		r.MaxExposurePct = 1
		r.MaxOpenPositions = 1
		j := researchJob{Risk: r, Fee: .001, SlippageBps: 2}
		change(&j)
		b, err := json.Marshal(j)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(input, b, 0600); err != nil {
			t.Fatal(err)
		}
		if err = runJob(input, filepath.Join(dir, "result.json")); err == nil {
			t.Fatal("invalid risk accepted")
		}
	}
}

func TestDocumentedSpotHaltKeepsMissingHourAndClockAlignment(t *testing.T) {
	file := filepath.Join(t.TempDir(), "halt.csv")
	start := time.Date(2023, 3, 24, 0, 0, 0, 0, time.UTC)
	var csv string
	for i := 0; i < 24; i++ {
		if i == 13 {
			continue
		}
		p := 100 + i
		csv += fmt.Sprintf("%d,%d,%d,%d,%d,1,100,3\n", start.Add(time.Duration(i)*time.Hour).Unix(), p, p+2, p-1, p+1)
	}
	if err := os.WriteFile(file, []byte(csv), 0600); err != nil {
		t.Fatal(err)
	}
	src, err := rangeSource(file, "BTCUSDT", "1h", start, start.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(src["1h"]) != 23 || len(src["4h"]) != 6 || len(src["1d"]) != 1 {
		t.Fatal("wrong trading-bar counts")
	}
	for _, c := range src["1h"] {
		if c.OpenTime.Equal(start.Add(13 * time.Hour)) {
			t.Fatal("must not invent an executable candle during the halt")
		}
	}
	b := src["4h"][3]
	if !b.OpenTime.Equal(start.Add(12*time.Hour)) || !b.CloseTime().Equal(start.Add(16*time.Hour)) || b.Volume.String() != "3" || b.High.String() != "117" || b.Low.String() != "111" || b.Close.String() != "116" {
		t.Fatalf("bad halted bucket: %+v", b)
	}
	if src["1d"][0].Volume.String() != "23" {
		t.Fatal("daily volume must include real trading bars only")
	}
}
