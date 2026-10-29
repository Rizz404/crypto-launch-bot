package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func TestPaperSizeIncludesCostsAndNeverEnablesEntry(t *testing.T) {
	p := ApprovedPaperPolicy()
	s := InitialPaperContext(testNow, p)
	for _, kind := range []string{"presale", "dex", "cex"} {
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
