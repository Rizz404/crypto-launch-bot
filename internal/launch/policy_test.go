package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestApprovedPolicyMatchesCommittedConfig(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "configs", "launch.policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	var p PaperPolicy
	if err = json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	if err = p.Validate(); err != nil {
		t.Fatal(err)
	}
	p.CallsPerWIBDay++
	if p.Validate() == nil {
		t.Fatal("modified policy inherited approval")
	}
}

func TestExitPolicyHoldsUnknownsAndLimits(t *testing.T) {
	p := ApprovedPaperPolicy()
	safe := func(kind string) Candidate {
		// Curve at the limit: 0.02 curve + 0.1/10 gas + 0.01 + 0.01 tax = 0.05.
		return Candidate{Kind: kind, Facts: map[string]string{"buy_tax": "0.01", "sell_tax": "0.01", "top_holder_fraction": "0.10",
			"top10_holder_fraction": "0.50", "holder_count": "50", "liquidity_usd": "10000",
			"curve_round_trip_cost_fraction": "0.02", "curve_round_trip_gas_usdt": "0.1"}}
	}
	for _, kind := range []string{"dex", "curve"} {
		if reasons := p.ExitReasons(safe(kind)); len(reasons) != 0 {
			t.Fatalf("%s at the limits was held: %v", kind, reasons)
		}
	}
	for _, tc := range []struct {
		kind, key, value, reason string
	}{
		{"dex", "sell_tax", "0.0501", "sell_tax_above_policy"},
		{"dex", "buy_tax", "", "buy_tax_unknown"},
		{"dex", "buy_tax", "-0.01", "buy_tax_unknown"},
		{"dex", "top_holder_fraction", "0.11", "top_holder_fraction_above_policy"},
		{"dex", "top10_holder_fraction", "", "top10_holder_fraction_unknown"},
		{"dex", "holder_count", "49", "holder_count_below_policy"},
		{"dex", "liquidity_usd", "9999", "liquidity_usd_below_policy"},
		{"dex", "liquidity_usd", "", "liquidity_usd_unknown"},
		{"curve", "curve_round_trip_gas_usdt", "0.11", "curve_round_trip_cost_above_policy"},
		{"curve", "sell_tax", "0.02", "curve_round_trip_cost_above_policy"},
		{"curve", "curve_round_trip_gas_usdt", "", "curve_round_trip_cost_unknown"},
	} {
		c := safe(tc.kind)
		c.Facts[tc.key] = tc.value
		if reasons := p.ExitReasons(c); !slices.Contains(reasons, tc.reason) {
			t.Fatalf("%s %s=%q: want %s, got %v", tc.kind, tc.key, tc.value, tc.reason, reasons)
		}
	}
}

func TestCurveRoundTripCostSumsSpreadGasAndTaxes(t *testing.T) {
	p := ApprovedPaperPolicy()
	c := Candidate{Facts: map[string]string{"curve_round_trip_cost_fraction": "0.023288", "curve_round_trip_gas_usdt": "0.014232", "buy_tax": "0.02", "sell_tax": "0.03"}}
	// 0.023288 + 0.014232/10 + 0.02 + 0.03
	if cost, ok := p.CurveRoundTripCost(c); !ok || cost.StringFixed(6) != "0.074711" {
		t.Fatalf("got %s ok=%v", cost, ok)
	}
	delete(c.Facts, "sell_tax")
	if _, ok := p.CurveRoundTripCost(c); ok {
		t.Fatal("missing tax treated as zero")
	}
}

func TestPaperSizeIncludesCostsAndNeverEnablesEntry(t *testing.T) {
	p := ApprovedPaperPolicy()
	s := InitialPaperContext(testNow, p)
	for _, kind := range []string{"presale", "dex", "curve", "cex"} {
		result, err := p.PlanSize(kind, s, decimal.RequireFromString("0.05"), decimal.RequireFromString("0.002"), true, testNow)
		if err != nil {
			t.Fatal(err)
		}
		cap := decimal.NewFromInt(10)
		if kind == "cex" {
			cap = decimal.NewFromInt(5)
		}
		if result.ModeledLossUSDT.GreaterThan(cap) || result.EntryAllowed || !result.NotionalUSDT.IsPositive() {
			t.Fatalf("unsafe sizing: %+v", result)
		}
	}
	if _, err := p.PlanSize("cex", s, decimal.Zero, decimal.Zero, true, testNow); err == nil {
		t.Fatal("missing stop accepted")
	}
	if _, err := p.PlanSize("dex", s, decimal.Zero, decimal.Zero, false, testNow); err == nil {
		t.Fatal("unknown costs treated as free")
	}
	s.AvailableUSDT = decimal.NewFromInt(3)
	r, err := p.PlanSize("presale", s, decimal.Zero, decimal.RequireFromString("0.1"), true, testNow)
	if err != nil || r.NotionalUSDT.Mul(decimal.RequireFromString("1.1")).GreaterThan(s.AvailableUSDT) {
		t.Fatal("fees overdraw paper cash")
	}
}

func TestHaltAndLockedExposureUseSameCapital(t *testing.T) {
	p := ApprovedPaperPolicy()
	s := InitialPaperContext(testNow, p)
	s.EquityUSDT = decimal.NewFromInt(980)
	s.AvailableUSDT = s.EquityUSDT
	if !p.Halted(s) {
		t.Fatal("daily boundary not halted")
	}
	s = InitialPaperContext(testNow, p)
	s.DayStartUSDT = decimal.NewFromInt(920)
	s.EquityUSDT = s.DayStartUSDT
	s.AvailableUSDT = s.EquityUSDT
	if !p.Halted(s) {
		t.Fatal("drawdown boundary not halted")
	}
	s = InitialPaperContext(testNow, p)
	s.Positions = []PaperExposure{{ID: "locked", CandidateID: "sale", AssetKey: "asset", Kind: "presale", NotionalUSDT: decimal.NewFromInt(10), LockedUSDT: decimal.NewFromInt(10)}}
	if s.Validate(testNow) == nil {
		t.Fatal("cash included committed funds")
	}
	s.AvailableUSDT = decimal.NewFromInt(990)
	if err := s.Validate(testNow); err != nil {
		t.Fatal(err)
	}
	s.Positions = append(s.Positions, PaperExposure{ID: "other", CandidateID: "other-sale", AssetKey: "other", Kind: "presale", NotionalUSDT: decimal.NewFromInt(10), LockedUSDT: decimal.NewFromInt(10)})
	s.AvailableUSDT = decimal.NewFromInt(980)
	if _, err := p.PlanSize("cex", s, decimal.RequireFromString("0.05"), decimal.Zero, true, testNow); err == nil {
		t.Fatal("locked exposures did not count toward position limit")
	}
	s.At = testNow.Add(-EvidenceTTL - time.Second)
	if s.Validate(testNow) == nil {
		t.Fatal("stale state accepted")
	}
}
