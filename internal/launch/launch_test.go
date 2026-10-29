package launch

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rizz404/trading-bot/internal/catalyst"
	"golang.org/x/time/rate"
)

var testNow = time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)

const testAddress = "0x1234567890123456789012345678901234567890"

func sampleCEX() Candidate {
	c := Candidate{Kind: "cex", Venue: "binance-spot", ExternalID: "OLDUSDT", Asset: Asset{VenueID: "binance:OLD", Symbol: "OLD"},
		StartsAt: testNow.Add(-MaxAge), TimeBasis: "first_trade_id_zero", State: "TRADING", ObservedAt: testNow,
		EvidenceIDs: []string{"proof"}, Facts: map[string]string{"bid": "1", "ask": "1.01", "bid_qty": "10", "ask_qty": "11"}}
	c.Seal()
	return c
}

func TestVerificationBoundaryAndUnknowns(t *testing.T) {
	tests := []struct {
		name           string
		change         func(*Candidate)
		status, reason string
	}{
		{"exact_30_days", func(*Candidate) {}, "accepted_for_research", ""},
		{"older", func(c *Candidate) { c.StartsAt = c.StartsAt.Add(-time.Nanosecond) }, "rejected", "listing_older_than_30_days"},
		{"future", func(c *Candidate) { c.StartsAt = testNow.Add(time.Minute) }, "held", "listing_not_open"},
		{"unknown_time", func(c *Candidate) { c.StartsAt = time.Time{} }, "held", "schedule_unknown"},
		{"pool_creation_not_trade", func(c *Candidate) { c.TimeBasis = "provider_pool_creation" }, "held", "trading_start_not_verified"},
		{"stale", func(c *Candidate) { c.ObservedAt = testNow.Add(-EvidenceTTL - time.Second) }, "held", "evidence_stale_or_future"},
		{"future_receipt", func(c *Candidate) { c.ObservedAt = testNow.Add(time.Second) }, "held", "evidence_stale_or_future"},
		{"missing_evidence", func(c *Candidate) { c.EvidenceIDs = nil }, "held", "evidence_missing"},
		{"halted", func(c *Candidate) { c.State = "HALT" }, "held", "market_not_trading"},
		{"crossed_book", func(c *Candidate) { c.Facts["bid"] = "100" }, "held", "book_invalid_or_empty"},
		{"zero_depth", func(c *Candidate) { c.Facts["ask_qty"] = "0" }, "held", "book_invalid_or_empty"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := sampleCEX()
			tc.change(&c)
			r := Verify(c, testNow, nil)
			if r.Status != tc.status || r.EntryAllowed || (tc.reason != "" && !slices.Contains(r.Reasons, tc.reason)) {
				t.Fatalf("unexpected review: %+v", r)
			}
		})
	}
	c := sampleCEX()
	if Verify(c, testNow, []string{"source_failed"}).Status != "held" {
		t.Fatal("source failure must invalidate cached candidate")
	}
}

func TestIdentityIsNotTickerAndRevisionsAreStable(t *testing.T) {
	a := Asset{Chain: "eip155:56", Address: testAddress, Symbol: "MEME"}
	b := Asset{Chain: "eip155:56", Address: "0x2234567890123456789012345678901234567890", Symbol: "MEME"}
	if a.Key() == b.Key() || a.Key() == (Asset{VenueID: "binance:MEME"}).Key() {
		t.Fatal("ticker merged identities")
	}
	if (Asset{Chain: "solana", Address: testAddress}).Key() != "" {
		t.Fatal("unsupported chain accepted")
	}
	c := sampleCEX()
	id, revision := c.ID, c.Revision
	c.ObservedAt = testNow.Add(time.Minute)
	c.EvidenceIDs = []string{"new-proof"}
	c.Seal()
	if c.ID != id || c.Revision != revision {
		t.Fatal("fetch timestamp changed semantic identity")
	}
	c.StartsAt = c.StartsAt.Add(time.Hour)
	c.Seal()
	if c.ID != id || c.Revision == revision {
		t.Fatal("schedule revision not tracked")
	}
	c.Venue = "other-venue"
	c.Seal()
	if c.ID == id {
		t.Fatal("different venues were merged")
	}
}

func TestPresaleParserRejectsTestnetAndPreservesAmbiguity(t *testing.T) {
	doc := catalyst.Document{URL: "https://www.pinksale.finance/launchpad/bsc/" + testAddress,
		Body: "Token Address " + testAddress + " Start time 2026.10.29 12:00 (UTC) End time 2026.10.30 12:00 (UTC) Status Upcoming"}
	c, ok := parseSale(doc, "proof", testNow)
	if !ok || c.Asset.Address != testAddress || c.StartsAt.Hour() != 12 || c.State != "upcoming" {
		t.Fatalf("bad sale: %+v", c)
	}
	if Verify(c, testNow, nil).Status != "held" {
		t.Fatal("publisher statement is not verified sale terms")
	}
	doc.Body += " Token Address 0x2234567890123456789012345678901234567890 Start time 2026.10.29 14:00 (UTC)"
	c, _ = parseSale(doc, "proof", testNow)
	if !slices.Contains(c.Gaps, "token_identity_conflict") || !slices.Contains(c.Gaps, "sale_schedule_conflict") {
		t.Fatal("ambiguity lost")
	}
	doc.URL = strings.Replace(doc.URL, "/bsc/", "/bsc-test/", 1)
	if _, ok = parseSale(doc, "proof", testNow); ok {
		t.Fatal("testnet accepted as mainnet")
	}
}

func TestHistoryPreservesFirstSeenAndHoldsConflict(t *testing.T) {
	c := sampleCEX()
	c.ObservedAt = testNow.Add(-time.Minute)
	c.Seal()
	later := c
	later.ObservedAt = testNow
	later.StartsAt = c.StartsAt.Add(time.Hour)
	later.Seal()
	batches := []Batch{{Version: Version, Kind: "cex", At: c.ObservedAt, Candidates: []Candidate{c}}, {Version: Version, Kind: "cex", At: testNow, Candidates: []Candidate{later}}}
	r, err := summarize(batches, testNow, func(string) error { return nil })
	if err != nil || len(r.Reviews) != 1 || !r.Reviews[0].Candidate.FirstSeenAt.Equal(c.ObservedAt) || r.Reviews[0].Candidate.Revision != later.Revision {
		t.Fatalf("history: %+v %v", r, err)
	}
	later.Asset = Asset{VenueID: "binance:DIFFERENT"}
	later.Seal()
	batches[1].Candidates = []Candidate{later}
	r, err = summarize(batches, testNow, func(string) error { return nil })
	if err != nil || !slices.Contains(r.Reviews[0].Reasons, "candidate_identity_or_revision_conflict") {
		t.Fatal("identity mutation not held")
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAPIFailureIsArchivedWithoutRetry(t *testing.T) {
	a := catalyst.Archive{Dir: t.TempDir()}
	c := NewClient(a)
	c.limiter = rate.NewLimiter(rate.Inf, 1)
	requests := 0
	c.HTTP = &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
		requests++
		return &http.Response{StatusCode: 429, Body: io.NopCloser(strings.NewReader(`{"error":"rate limit"}`))}, nil
	})}
	var out any
	id, err := c.fetch(context.Background(), "https://api.gopluslabs.io/api/v1/token_security/56", nil, &out)
	if err == nil || id == "" || requests != 1 {
		t.Fatal("failed request not preserved")
	}
	var raw rawRecord
	if err = a.Read("launch-raw", id, &raw); err != nil || raw.Status != 429 {
		t.Fatal("missing raw evidence")
	}
	if _, err = c.fetch(context.Background(), "https://127.0.0.1/private", nil, &out); err == nil || requests != 1 {
		t.Fatal("untrusted origin contacted")
	}
	if err = os.WriteFile(filepath.Join(a.Dir, "launch-raw", id+".json"), []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	raw = rawRecord{}
	if err = a.Read("launch-raw", id, &raw); err == nil {
		t.Fatal("archive corruption accepted")
	}
}

func TestEvidenceFailureAndUnsafeFlags(t *testing.T) {
	c := sampleCEX()
	b := Batch{Version: Version, Kind: "cex", At: testNow, Candidates: []Candidate{c}}
	if _, err := summarize([]Batch{b}, testNow, func(string) error { return errors.New("missing proof") }); err == nil {
		t.Fatal("unreadable evidence accepted")
	}
	c.Kind = "dex"
	c.Asset = Asset{Chain: "eip155:56", Address: testAddress}
	c.Facts["is_honeypot"] = "1"
	if r := Verify(c, testNow, nil); r.Status != "rejected" || r.EntryAllowed {
		t.Fatal("honeypot was not rejected")
	}
}

func TestCEXHistoryRequiresTradeZeroAndKeepsVenueIdentity(t *testing.T) {
	for _, trade := range []string{`[{"a":0,"T":1793268000000}]`, `[{"a":9,"T":1793268000000}]`, `[{"T":1793268000000}]`} {
		c := NewClient(catalyst.Archive{Dir: t.TempDir()})
		c.limiter = rate.NewLimiter(rate.Inf, 1)
		c.HTTP = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
			body := `{"symbols":[{"symbol":"MEMEUSDT","baseAsset":"MEME","quoteAsset":"USDT","status":"TRADING"}]}`
			switch req.URL.Path {
			case "/api/v3/aggTrades":
				if req.URL.Query().Get("fromId") != "0" {
					t.Fatal("first trade query omitted")
				}
				body = trade
			case "/api/v3/ticker/bookTicker":
				body = `{"symbol":"MEMEUSDT","bidPrice":"1","askPrice":"1.01","bidQty":"10","askQty":"10"}`
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
		})}
		b := c.discoverCEX(context.Background(), 5, map[string]time.Time{})
		if len(b.Candidates) != 1 || b.Candidates[0].Asset.Key() != "cex:binance:MEME" {
			t.Fatalf("bad CEX batch: %+v", b)
		}
		if strings.Contains(trade, `"a":0`) != (b.Candidates[0].TimeBasis == "first_trade_id_zero") {
			t.Fatal("partial history treated as verified listing time")
		}
	}
}

func TestPoolCursorKeepsBacklogAndDetectsRegistryRegression(t *testing.T) {
	next := uint64(100)
	start, end, err := poolWindow(120, 5, &next)
	if err != nil || start != 100 || end != 105 {
		t.Fatal("new pool burst was skipped")
	}
	start, end, err = poolWindow(120, 5, nil)
	if err != nil || start != 115 || end != 120 {
		t.Fatal("bootstrap tail invalid")
	}
	if _, _, err = poolWindow(90, 5, &next); err == nil {
		t.Fatal("regressed registry accepted")
	}
	start, end, err = poolWindow(0, 5, nil)
	if err != nil || start != 0 || end != 0 {
		t.Fatal("empty registry underflow")
	}
}

func TestScannerCannotOverwriteOnchainProofAndRPCFailureStaysUnknown(t *testing.T) {
	c := NewClient(catalyst.Archive{Dir: t.TempDir()})
	c.limiter = rate.NewLimiter(rate.Inf, 1)
	c.HTTP = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		body := `{"jsonrpc":"2.0","id":1,"result":"0x6000"}`
		if req.URL.Host == "api.gopluslabs.io" {
			body = `{"code":1,"result":{"` + testAddress + `":{"chain_verified":"999","contract_code":"fake","is_honeypot":"1","is_mintable":"0"}}}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	candidate := Candidate{Asset: Asset{Chain: "eip155:56", Address: testAddress}}
	c.inspectToken(context.Background(), &candidate, chainSnapshot{Number: "0x1", Hash: "proof"})
	if candidate.Facts["chain_verified"] != "56" || candidate.Facts["contract_code"] != "present" || candidate.Facts["is_honeypot"] != "1" {
		t.Fatal("scanner crossed proof trust boundary")
	}
	c.HTTP = &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"jsonrpc":"2.0","id":1,"result":null,"error":{"code":-1}}`))}, nil
	})}
	var out json.RawMessage
	if _, err := c.rpc(context.Background(), "eth_getCode", []any{}, &out); err == nil {
		t.Fatal("RPC error accepted")
	}
}
