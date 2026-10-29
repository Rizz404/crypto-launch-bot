package launch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rizz404/trading-bot/internal/catalyst"
	"github.com/shopspring/decimal"
)

func decisionFixture() (AnalysisPacket, Decision) {
	p := ApprovedPaperPolicy()
	c := sampleCEX()
	packet := AnalysisPacket{Version: AnalysisVersion, At: testNow, Policy: p, PolicyHash: catalyst.Hash(p), Context: InitialPaperContext(testNow, p),
		Verification: Verify(c, testNow, nil), Evidence: []analysisEvidence{{ID: "proof", At: testNow, Text: "The first trade is available and the market is open."}}}
	d := Decision{CandidateID: c.ID, CandidateRevision: c.Revision, AssetKey: c.Asset.Key(), PolicyHash: packet.PolicyHash, Action: "PROPOSE_ENTRY",
		Thesis: "new venue liquidity", Counterargument: "price can fall", Invalidation: "liquidity disappears", HoldRationale: "review if thesis persists", Reason: "research proposal",
		Citations: []catalyst.Citation{{EvidenceID: "proof", Quote: "market is open"}}}
	return packet, d
}

func TestDecisionRejectsEscalationFabricationAndExpiry(t *testing.T) {
	tests := []struct {
		name   string
		change func(*AnalysisPacket, *Decision)
	}{
		{"wrong_asset", func(_ *AnalysisPacket, d *Decision) { d.AssetKey = "different" }},
		{"wrong_revision", func(_ *AnalysisPacket, d *Decision) { d.CandidateRevision = "old" }},
		{"wrong_policy", func(_ *AnalysisPacket, d *Decision) { d.PolicyHash = "larger-risk" }},
		{"fabricated_quote", func(_ *AnalysisPacket, d *Decision) { d.Citations[0].Quote = "guaranteed profit" }},
		{"unknown_source", func(_ *AnalysisPacket, d *Decision) { d.Citations[0].EvidenceID = "not-supplied" }},
		{"held", func(p *AnalysisPacket, _ *Decision) { p.Verification.Status = "held" }},
		{"consumed", func(p *AnalysisPacket, _ *Decision) {
			p.Context.ConsumedCandidates = []string{p.Verification.Candidate.ID}
		}},
		{"cash_empty", func(p *AnalysisPacket, _ *Decision) { p.Context.AvailableUSDT = decimal.Zero }},
		{"expired", func(p *AnalysisPacket, _ *Decision) { p.At = testNow.Add(-301 * time.Second) }},
		{"daily_halt", func(p *AnalysisPacket, _ *Decision) {
			p.Context.EquityUSDT = decimal.NewFromInt(980)
			p.Context.AvailableUSDT = p.Context.EquityUSDT
		}},
		{"wrong_lifecycle", func(_ *AnalysisPacket, d *Decision) { d.Action = "PROPOSE_CONTRIBUTE" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p, d := decisionFixture()
			test.change(&p, &d)
			if ValidateDecision(d, p, testNow) == nil {
				t.Fatal("invalid proposal accepted")
			}
		})
	}
	p, d := decisionFixture()
	if err := ValidateDecision(d, p, testNow); err != nil {
		t.Fatal(err)
	}
	d.Action = "WATCH"
	p.Verification.Status = "held"
	if err := ValidateDecision(d, p, testNow); err != nil {
		t.Fatal("held evidence must still support WATCH", err)
	}
}

func TestDecisionSchemaRejectsMissingDuplicateAndRiskFields(t *testing.T) {
	_, d := decisionFixture()
	raw, _ := json.Marshal(d)
	if _, err := DecodeDecision(raw); err != nil {
		t.Fatal(err)
	}
	withRisk := strings.TrimSuffix(string(raw), "}") + `,"position_size":"100000"}`
	if _, err := DecodeDecision([]byte(withRisk)); err == nil {
		t.Fatal("model set position size")
	}
	duplicate := strings.TrimSuffix(string(raw), "}") + `,"action":"PROPOSE_ENTRY"}`
	if _, err := DecodeDecision([]byte(duplicate)); err == nil {
		t.Fatal("duplicate field accepted")
	}
	var fields map[string]any
	_ = json.Unmarshal(raw, &fields)
	delete(fields, "position_id")
	missing, _ := json.Marshal(fields)
	if _, err := DecodeDecision(missing); err == nil {
		t.Fatal("missing field accepted")
	}
}

func TestLockedLifecycleAndOriginalThesisCannotBeChanged(t *testing.T) {
	p, d := decisionFixture()
	c := &p.Verification.Candidate
	c.Kind = "presale"
	c.EndsAt = testNow.Add(time.Hour)
	c.Seal()
	d.CandidateID = c.ID
	d.CandidateRevision = c.Revision
	d.Action = "HOLD"
	d.PositionID = "sale-position"
	p.Context.AvailableUSDT = decimal.NewFromInt(990)
	p.Context.Positions = []PaperExposure{{ID: d.PositionID, CandidateID: c.ID, AssetKey: c.Asset.Key(), Kind: "presale", NotionalUSDT: decimal.NewFromInt(10), LockedUSDT: decimal.NewFromInt(10), Thesis: d.Thesis, Invalidation: d.Invalidation}}
	if err := ValidateDecision(d, p, testNow); err != nil {
		t.Fatal(err)
	}
	d.Invalidation = "keep holding forever"
	if ValidateDecision(d, p, testNow) == nil {
		t.Fatal("HOLD replaced invalidation")
	}
	d.Invalidation = p.Context.Positions[0].Invalidation
	d.Action = "PROPOSE_EXIT"
	if ValidateDecision(d, p, testNow) == nil {
		t.Fatal("locked funds sold")
	}
	d.Action = "PROPOSE_CLAIM"
	if ValidateDecision(d, p, testNow) == nil {
		t.Fatal("claim invented")
	}
	p.Context.Positions[0].Claimable = true
	if err := ValidateDecision(d, p, testNow); err != nil {
		t.Fatal(err)
	}
	c.Kind = "dex"
	c.Seal()
	d.CandidateID = c.ID
	d.CandidateRevision = c.Revision
	p.Context.Positions[0].CandidateID = c.ID
	if ValidateDecision(d, p, testNow) == nil {
		t.Fatal("claim associated with a DEX pool")
	}
}

type completionFunc func(context.Context, int, string, map[string]any, []byte) (string, string, int, int, error)

func (f completionFunc) StructuredCompletion(ctx context.Context, n int, prompt string, format map[string]any, b []byte) (string, string, int, int, error) {
	return f(ctx, n, prompt, format, b)
}

func archivedAnalysisFixture(t *testing.T) (catalyst.Archive, AnalysisPacket) {
	t.Helper()
	a := catalyst.Archive{Dir: t.TempDir()}
	now := time.Now().UTC()
	id, err := a.Put("launch-raw", rawRecord{URL: "https://example.org/public", At: now, Status: 200, Body: "Public evidence says market is open."})
	if err != nil {
		t.Fatal(err)
	}
	c := sampleCEX()
	c.ObservedAt = now
	c.StartsAt = now.Add(-time.Hour)
	c.EvidenceIDs = []string{id}
	c.Seal()
	if _, err = a.Put("launch-batches", Batch{Version: Version, Kind: "cex", At: now, Candidates: []Candidate{c}}); err != nil {
		t.Fatal(err)
	}
	p := ApprovedPaperPolicy()
	packet, err := BuildAnalysisPacket(a, c.ID, p, InitialPaperContext(now, p), now)
	if err != nil {
		t.Fatal(err)
	}
	return a, packet
}

func TestInferenceIsArchivedDeduplicatedAndExpires(t *testing.T) {
	a, packet := archivedAnalysisFixture(t)
	calls := 0
	client := completionFunc(func(_ context.Context, tokens int, _ string, format map[string]any, input []byte) (string, string, int, int, error) {
		calls++
		if tokens != 2048 || format["type"] != "json_schema" {
			t.Fatal("missing schema or token cap")
		}
		var p AnalysisPacket
		if err := json.Unmarshal(input, &p); err != nil {
			t.Fatal(err)
		}
		d := Decision{CandidateID: p.Verification.Candidate.ID, CandidateRevision: p.Verification.Candidate.Revision, AssetKey: p.Verification.Candidate.Asset.Key(), PolicyHash: p.PolicyHash, Action: "WATCH", Reason: "more evidence needed", Citations: []catalyst.Citation{{EvidenceID: p.Evidence[0].ID, Quote: "market is open"}}}
		b, _ := json.Marshal(d)
		return string(b), "mock-served", 100, 100, nil
	})
	r, err := Analyze(context.Background(), a, "mock-model", client, packet)
	if err != nil || !r.Valid || r.EntryAllowed {
		t.Fatalf("analysis: %+v %v", r, err)
	}
	if _, err = Analyze(context.Background(), a, "mock-model", client, packet); !errors.Is(err, catalyst.ErrDuplicateInference) || calls != 1 {
		t.Fatal("duplicate inference spent a second call")
	}
	report, err := ReadReport(a, r.ExpiresAt.Add(time.Second))
	if err != nil || len(report.Analyses) != 1 || report.Analyses[0].Valid {
		t.Fatal("expired analysis stayed valid")
	}
}

func TestDailyQuotaSurvivesFailuresAndWIBBoundary(t *testing.T) {
	boundaryArchive := catalyst.Archive{Dir: t.TempDir()}
	beforeMidnight := time.Date(2026, 10, 29, 16, 59, 59, 0, time.UTC)
	for range 12 {
		if err := boundaryArchive.ReserveResearchInference(beforeMidnight); err != nil {
			t.Fatal(err)
		}
	}
	if err := boundaryArchive.ReserveResearchInference(beforeMidnight.Add(time.Second)); err != nil {
		t.Fatal("quota did not reset at WIB midnight")
	}
	a, packet := archivedAnalysisFixture(t)
	now := time.Now().UTC()
	for range packet.Policy.CallsPerWIBDay {
		if err := a.ReserveResearchInference(now); err != nil {
			t.Fatal(err)
		}
	}
	calls := 0
	client := completionFunc(func(context.Context, int, string, map[string]any, []byte) (string, string, int, int, error) {
		calls++
		return "", "", 0, 0, errors.New("outage")
	})
	if _, err := Analyze(context.Background(), a, "mock-model", client, packet); err == nil || calls != 0 {
		t.Fatal("daily quota did not stop transport")
	}
	if err := a.ReserveResearchInference(now.Add(24 * time.Hour)); err != nil {
		t.Fatal("WIB quota did not reset")
	}
	failedArchive, failedPacket := archivedAnalysisFixture(t)
	if r, err := Analyze(context.Background(), failedArchive, "mock-model", client, failedPacket); err == nil || r.Valid {
		t.Fatal("provider outage considered valid")
	}
	for i := 1; i < 12; i++ {
		if err := failedArchive.ReserveResearchInference(time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if err := failedArchive.ReserveResearchInference(time.Now()); err == nil {
		t.Fatal(fmt.Sprint("failed call refunded a quota slot"))
	}
}
