package launch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rizz404/crypto-launch-bot/internal/catalyst"
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
			case "/api/v3/exchangeInfo":
				if req.URL.Query().Get("symbolStatus") != "TRADING" {
					t.Fatal("unfiltered exchangeInfo exceeds the response limit")
				}
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

func TestCEXOldRetainedTradeRejectsWithoutClaimingListingTime(t *testing.T) {
	c := sampleCEX()
	c.StartsAt, c.TimeBasis = time.Time{}, ""
	c.Gaps = []string{"first_trade_history_unavailable"}
	c.Facts["oldest_retained_trade_at"] = testNow.Add(-MaxAge - time.Hour).Format(time.RFC3339)
	c.Seal()
	if r := Verify(c, testNow, nil); r.Status != "rejected" || !slices.Contains(r.Reasons, "listing_older_than_30_days") {
		t.Fatalf("old retained trade not rejected: %+v", r)
	}
	c.Facts["oldest_retained_trade_at"] = testNow.Add(-time.Hour).Format(time.RFC3339)
	c.Seal()
	if r := Verify(c, testNow, nil); r.Status != "held" || slices.Contains(r.Reasons, "listing_older_than_30_days") {
		t.Fatalf("recent retained trade must stay held: %+v", r)
	}
}

func TestDEXRefreshesPrunedSnapshotBeforePoolReads(t *testing.T) {
	c := NewClient(catalyst.Archive{Dir: t.TempDir()})
	c.limiter = rate.NewLimiter(rate.Inf, 1)
	blocks := 0
	var callBlocks []string
	c.HTTP = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		reply := func(result string) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"jsonrpc":"2.0","id":1,"result":` + result + `}`))}, nil
		}
		if req.URL.Host != "bsc-dataseed.bnbchain.org" {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"pairs":null}`))}, nil
		}
		var body struct {
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(req.Body).Decode(&body)
		switch body.Method {
		case "eth_chainId":
			return reply(`"0x38"`)
		case "eth_getBlockByNumber":
			blocks++
			ts := time.Now().Unix()
			if blocks == 1 {
				ts = time.Now().Add(-time.Minute).Unix() // fresh enough to start, too old for pool reads
			}
			return reply(fmt.Sprintf(`{"number":"0x%x","hash":"0x%064x","timestamp":"0x%x"}`, 100+blocks, blocks, ts))
		case "eth_getCode":
			return reply(`"0x"`)
		}
		var call struct{ Data string }
		_ = json.Unmarshal(body.Params[0], &call)
		var block string
		_ = json.Unmarshal(body.Params[1], &block)
		switch {
		case call.Data == allPairsLength:
			return reply(`"0x` + fmt.Sprintf("%064x", 1) + `"`)
		case strings.HasPrefix(call.Data, allPairs):
			callBlocks = append(callBlocks, block)
			return reply(`"0x000000000000000000000000` + strings.Repeat("ab", 20) + `"`)
		case call.Data == token0:
			return reply(`"0x000000000000000000000000` + strings.Repeat("cd", 20) + `"`)
		}
		return reply(`"0x000000000000000000000000` + strings.Repeat("ef", 20) + `"`)
	})}
	b := c.discoverDEX(context.Background(), 1, nil)
	if blocks != 2 || len(callBlocks) != 1 || callBlocks[0] != "0x66" || len(b.Candidates) != 1 {
		t.Fatalf("stale snapshot reused: blocks=%d calls=%v batch=%+v", blocks, callBlocks, b)
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

func TestGoPlusHoldersSkipPoolLockedAndDeadButKeepHiddenTail(t *testing.T) {
	pool := "0x" + strings.Repeat("ab", 20)
	holder := func(addr, percent string, locked int) string {
		return fmt.Sprintf(`{"address":"%s","percent":"%s","is_locked":%d}`, addr, percent, locked)
	}
	list := []string{holder(pool, "0.40", 0), holder(deadAddress, "0.20", 1), holder("0x"+strings.Repeat("11", 20), "0.08", 0)}
	for i := range 7 {
		list = append(list, holder(fmt.Sprintf("0x%040x", i+1), "0.01", 0))
	}
	c := Candidate{Kind: "dex", ExternalID: pool, Facts: map[string]string{}}
	goplusHolders(&c, json.RawMessage("["+strings.Join(list, ",")+"]"))
	// 0.08 + 7×0.01 seen, plus two hidden holders of at most 0.01 replacing the pool and dead slots.
	if c.Facts["top_holder_fraction"] != "0.080000" || c.Facts["top10_holder_fraction"] != "0.170000" {
		t.Fatalf("bad concentration: %+v", c.Facts)
	}
	c = Candidate{Kind: "dex", ExternalID: pool, Facts: map[string]string{}}
	goplusHolders(&c, json.RawMessage(`[{"address":"`+pool+`","percent":"bad"}]`))
	if c.Facts["top_holder_fraction"] != "" {
		t.Fatal("malformed holder list produced concentration facts")
	}
}

func jsonReply(body string) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
}

func abiWord(hexValue string) string { return strings.Repeat("0", 64-len(hexValue)) + hexValue }

func TestGeckoPoolFillsMarketOnlyWhenPairMatchesChain(t *testing.T) {
	pool, a0, a1 := "0x"+strings.Repeat("ab", 20), "0x"+strings.Repeat("cd", 20), "0x"+strings.Repeat("ef", 20)
	body := `{"data":{"attributes":{"address":"` + pool + `","pool_created_at":"2026-11-12T04:35:42Z","base_token_price_usd":"0.5","reserve_in_usd":"1234.5"},
"relationships":{"base_token":{"data":{"id":"bsc_` + a1 + `"}},"quote_token":{"data":{"id":"bsc_` + a0 + `"}},"dex":{"data":{"id":"pancakeswap_v2"}}}}}`
	c := NewClient(catalyst.Archive{Dir: t.TempDir()})
	c.limiter = rate.NewLimiter(rate.Inf, 1)
	c.HTTP = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "api.geckoterminal.com" || req.URL.Path != "/api/v2/networks/bsc/pools/"+pool {
			t.Fatalf("unexpected request %s", req.URL)
		}
		return jsonReply(body)
	})}
	candidate := Candidate{Facts: map[string]string{}}
	if _, ok := c.geckoPool(context.Background(), &candidate, pool, a0, a1); !ok || candidate.Asset.Address != a1 || candidate.Quote != a0 ||
		candidate.StartsAt.IsZero() || candidate.TimeBasis != "provider_pool_creation" || candidate.Facts["market_provider"] != "geckoterminal" || candidate.Facts["liquidity_usd"] != "1234.5" {
		t.Fatalf("matching pool not used: %+v", candidate)
	}
	other := Candidate{Facts: map[string]string{}}
	if _, ok := c.geckoPool(context.Background(), &other, pool, a0, testAddress); ok || !slices.Contains(other.Gaps, "provider_pair_identity_conflict") || !other.StartsAt.IsZero() {
		t.Fatalf("provider pair overrode chain identity: %+v", other)
	}
}

func TestSaleContractMustBeKnownPinkSaleClone(t *testing.T) {
	check := func(implementation, factory string) Candidate {
		c := NewClient(catalyst.Archive{Dir: t.TempDir()})
		c.limiter = rate.NewLimiter(rate.Inf, 1)
		c.HTTP = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
			var body struct{ Method string }
			_ = json.NewDecoder(req.Body).Decode(&body)
			if body.Method == "eth_getCode" {
				return jsonReply(`{"jsonrpc":"2.0","id":1,"result":"0x363d3d373d3d3d363d73` + implementation[2:] + `5af43d82803e903d91602b57fd5bf3"}`)
			}
			return jsonReply(`{"jsonrpc":"2.0","id":1,"result":"0x` + abiWord(factory[2:]) + `"}`)
		})}
		candidate := Candidate{ExternalID: testAddress, Facts: map[string]string{}}
		c.inspectSale(context.Background(), &candidate, chainSnapshot{Number: "0x1"})
		return candidate
	}
	const presale, fairlaunch = "0xd18b06e912575a388593ee07bf6f902bd6e30109", "0x07f6351491b694f3a04c5f1e47d151d4ad927d45"
	for implementation, kind := range map[string]string{presale: "presale", fairlaunch: "fairlaunch"} {
		if c := check(implementation, pinkFactory); c.Facts["sale_code"] != "known_pinksale_clone" || c.Facts["sale_type"] != kind || c.Facts["sale_factory"] != pinkFactory || len(c.Gaps) != 0 {
			t.Fatalf("known %s clone not recognised: %+v", kind, c)
		}
	}
	for name, c := range map[string]Candidate{"implementation": check(testAddress, pinkFactory), "factory": check(presale, testAddress)} {
		if c.Facts["sale_code"] != "present_not_factory_verified" || c.Facts["sale_type"] != "" || !slices.Contains(c.Gaps, "sale_contract_not_known_pinksale_clone") {
			t.Fatalf("foreign %s accepted: %+v", name, c)
		}
	}
}

func TestCurveDiscoveryConfirmsLeadLogsThroughHelper(t *testing.T) {
	token := "0x" + strings.Repeat("ab", 20)
	discover := func(manager string) Batch {
		c := NewClient(catalyst.Archive{Dir: t.TempDir()})
		c.limiter = rate.NewLimiter(rate.Inf, 1)
		c.HTTP = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
			reply := func(result string) (*http.Response, error) {
				return jsonReply(`{"jsonrpc":"2.0","id":1,"result":` + result + `}`)
			}
			if req.URL.Host == "api.gopluslabs.io" {
				return jsonReply(`{"code":0}`)
			}
			if req.URL.Host == "data-api.binance.vision" {
				return jsonReply(`{"symbol":"BNBUSDT","askPrice":"600"}`)
			}
			var body struct {
				Method string
				Params []json.RawMessage
			}
			_ = json.NewDecoder(req.Body).Decode(&body)
			if req.URL.Host == "bsc-rpc.publicnode.com" {
				if body.Method != "eth_getLogs" {
					t.Fatalf("lead node asked for %s", body.Method)
				}
				if strings.Contains(string(body.Params[0]), transferTopic) {
					// Mint 1000 to the manager, which sells 60 to a buyer and 40 to a second buyer.
					transfer := func(from, to, amount string) string {
						return `{"address":"` + token + `","topics":["` + transferTopic + `","0x` + abiWord(from) + `","0x` + abiWord(to) + `"],"data":"0x` + abiWord(amount) + `","blockNumber":"0x10"}`
					}
					return reply(`[` + transfer("0", fourMemeManager[2:], "3e8") + `,` + transfer(fourMemeManager[2:], strings.Repeat("11", 20), "3c") + `,` + transfer(fourMemeManager[2:], strings.Repeat("22", 20), "28") + `]`)
				}
				data := "0x" + abiWord(testAddress[2:]) + abiWord(token[2:]) + strings.Repeat(abiWord("0"), 6)
				return reply(`[{"address":"` + testAddress + `","topics":["` + tokenCreateTopic + `"],"data":"` + data + `","blockNumber":"0x10"},
{"address":"` + fourMemeManager + `","topics":["` + tokenCreateTopic + `"],"data":"` + data + `","blockNumber":"0x10"}]`)
			}
			switch body.Method {
			case "eth_chainId":
				return reply(`"0x38"`)
			case "eth_getBlockByNumber":
				return reply(fmt.Sprintf(`{"number":"0x64","hash":"0x%064x","timestamp":"0x%x"}`, 1, time.Now().Unix()))
			case "eth_getCode":
				return reply(`"0x6000"`)
			case "eth_getLogs":
				t.Fatal("logs requested from the state node")
			case "eth_gasPrice":
				return reply(`"0x2faf080"`) // 0.05 gwei
			}
			call := ""
			if len(body.Params) > 0 {
				call = string(body.Params[0])
			}
			switch {
			case strings.Contains(call, tokenInfos):
				// TaxToken9: creator type 9 in template bits 10–15.
				return reply(`"0x` + abiWord(token[2:]) + abiWord("0") + abiWord("2400") + strings.Repeat(abiWord("0"), 10) + `"`)
			case strings.Contains(call, feeRateBuy):
				return reply(`"0x` + abiWord("2") + `"`)
			case strings.Contains(call, feeRateSell):
				return reply(`"0x` + abiWord("3") + `"`)
			}
			if len(body.Params) > 0 && strings.Contains(string(body.Params[0]), tryBuy) {
				// 1000 tokens for cost 99 + fee 1.
				return reply(`"0x` + abiWord(fourMemeManager[2:]) + abiWord("0") + abiWord("3e8") + abiWord("63") + abiWord("1") + strings.Repeat(abiWord("0"), 3) + `"`)
			}
			if len(body.Params) > 0 && strings.Contains(string(body.Params[0]), trySell) {
				if !strings.Contains(string(body.Params[0]), abiWord("3e8")) {
					t.Fatal("sell quote did not use the bought amount")
				}
				return reply(`"0x` + abiWord(fourMemeManager[2:]) + abiWord("0") + abiWord("60") + abiWord("1") + `"`)
			}
			info := abiWord("2") + abiWord(manager[2:]) + strings.Repeat(abiWord("0"), 10)
			return reply(`"0x` + info + `"`)
		})}
		return c.discoverCurve(context.Background(), 5)
	}
	b := discover(fourMemeManager)
	if len(b.Candidates) != 1 || !slices.Contains(b.Gaps, "curve_log_malformed") {
		t.Fatalf("foreign log not dropped: %+v", b)
	}
	c := b.Candidates[0]
	if c.Kind != "curve" || c.Venue != "four-meme" || c.ExternalID != token || c.Quote != "BNB" || c.State != "bonding_curve" || c.TimeBasis != "onchain_creation_block" || c.StartsAt.IsZero() || c.Asset.Key() == "" {
		t.Fatalf("bad curve candidate: %+v", c)
	}
	if c.Facts["curve_buy_tokens"] != "1000" || c.Facts["curve_buy_paid_wei"] != "100" || c.Facts["curve_sell_funds_wei"] != "96" || c.Facts["curve_round_trip_cost_fraction"] != "0.040000" {
		t.Fatalf("bad curve round-trip quote: %+v", c.Facts)
	}
	// 0.05 gwei × 460k gas × 600 USDT/BNB.
	if c.Facts["curve_creator_type"] != "9" || c.Facts["buy_tax"] != "0.0200" || c.Facts["sell_tax"] != "0.0300" || c.Facts["curve_round_trip_gas_usdt"] != "0.013800" {
		t.Fatalf("bad curve tax/gas facts: %+v", c.Facts)
	}
	if c.Facts["holder_count"] != "2" || c.Facts["top_holder_fraction"] != "0.060000" || c.Facts["top10_holder_fraction"] != "0.100000" || c.Facts["holder_source"] != "chain_transfers" {
		t.Fatalf("manager inventory counted as a holder: %+v", c.Facts)
	}
	r := Verify(c, time.Now().UTC(), nil)
	if r.Status != "held" || r.EntryAllowed || !slices.Contains(r.Reasons, "holder_count_below_policy") || slices.Contains(r.Reasons, "buy_tax_above_policy") ||
		slices.Contains(r.Reasons, "curve_manager_not_confirmed") || slices.Contains(r.Reasons, "trading_start_not_verified") || slices.Contains(r.Reasons, "cannot_sell_all_unknown") {
		t.Fatalf("unexpected curve review: %+v", r)
	}
	if r = Verify(discover(testAddress).Candidates[0], time.Now().UTC(), nil); !slices.Contains(r.Reasons, "curve_manager_not_confirmed") {
		t.Fatalf("foreign manager accepted: %+v", r)
	}
}

func TestCurveTaxByCreatorType(t *testing.T) {
	token := "0x" + strings.Repeat("ab", 20)
	for template, want := range map[string]string{"0": "0.0000/0.0000", "1400": "0.0150/0.0150", "3f": "0.0000/0.0000", "1c00": "unrecognised"} {
		c := NewClient(catalyst.Archive{Dir: t.TempDir()})
		c.limiter = rate.NewLimiter(rate.Inf, 1)
		c.HTTP = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
			body, _ := io.ReadAll(req.Body)
			result := abiWord(token[2:]) + abiWord("0") + abiWord(template) + strings.Repeat(abiWord("0"), 10)
			if strings.Contains(string(body), feeRate) {
				result = abiWord("96") // 150 basis points
			}
			return jsonReply(`{"jsonrpc":"2.0","id":1,"result":"0x` + result + `"}`)
		})}
		candidate := Candidate{Facts: map[string]string{}}
		c.curveTax(context.Background(), &candidate, token, abiWord(token[2:]), "0x1")
		got := candidate.Facts["buy_tax"] + "/" + candidate.Facts["sell_tax"]
		if slices.Contains(candidate.Gaps, "curve_creator_type_unrecognised") {
			got = "unrecognised"
		}
		if got != want {
			t.Fatalf("template %s: got %s, want %s", template, got, want)
		}
	}
}
