package launch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rizz404/crypto-launch-bot/internal/catalyst"
	"github.com/shopspring/decimal"
	"golang.org/x/time/rate"
)

// shadowMarket answers the shadow pass's quote requests; bid moves the CEX price
// and sellWei the curve sell quote.
type shadowMarket struct {
	bid     string
	sellWei string
	fail    bool
}

func (m *shadowMarket) client(t *testing.T, a catalyst.Archive) *Client {
	c := NewClient(a)
	c.limiter = rate.NewLimiter(rate.Inf, 1)
	c.HTTP = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		if m.fail {
			return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("down"))}, nil
		}
		if req.URL.Host == "data-api.binance.vision" {
			symbol := req.URL.Query().Get("symbol")
			bid := m.bid
			if symbol == "BNBUSDT" {
				bid = "600"
			}
			return jsonReply(fmt.Sprintf(`{"symbol":%q,"bidPrice":%q}`, symbol, bid))
		}
		var body struct{ Method string }
		_ = json.NewDecoder(req.Body).Decode(&body)
		switch body.Method {
		case "eth_chainId":
			return jsonReply(`{"jsonrpc":"2.0","id":1,"result":"0x38"}`)
		case "eth_getBlockByNumber":
			return jsonReply(fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"result":{"number":"0x64","hash":"0x%064x","timestamp":"0x%x"}}`, 1, time.Now().Unix()))
		}
		return jsonReply(`{"jsonrpc":"2.0","id":1,"result":"0x` + abiWord(fourMemeManager[2:]) + abiWord("0") + abiWord(m.sellWei) + abiWord("1") + `"}`)
	})}
	return c
}

func acceptedCurve(now time.Time) Candidate {
	c := Candidate{Kind: "curve", Venue: "four-meme", ExternalID: testAddress, Quote: "BNB", State: "bonding_curve",
		Asset: Asset{Chain: "eip155:56", Address: testAddress}, StartsAt: now.Add(-time.Hour), TimeBasis: "onchain_creation_block",
		ObservedAt: now, FirstSeenAt: now, EvidenceIDs: []string{"proof"},
		Facts: map[string]string{"chain_verified": "56", "contract_code": "present", "finalized_block": "0x1", "token_manager": fourMemeManager, "version": "2",
			"is_honeypot": "0", "is_blacklisted": "0", "is_proxy": "0", "is_mintable": "0", "owner_change_balance": "0", "slippage_modifiable": "0",
			"transfer_pausable": "0", "is_open_source": "1", "buy_tax": "0.0000", "sell_tax": "0.0000", "holder_count": "80",
			"top_holder_fraction": "0.050000", "top10_holder_fraction": "0.300000", "curve_round_trip_cost_fraction": "0.023000",
			"curve_round_trip_gas_usdt": "0.014000", "curve_sell_funds_wei": "9767118962427648",
			// 0.01 BNB buys 1e24 token units; BNB ask 600 USDT.
			"curve_buy_tokens": "1000000000000000000000000", "curve_buy_paid_wei": "10000000000000000", "bnb_usdt_ask": "600"}}
	c.Seal()
	return c
}

func shadowArchive(t *testing.T, now time.Time, candidates ...Candidate) catalyst.Archive {
	t.Helper()
	a := catalyst.Archive{Dir: t.TempDir()}
	id, err := a.Put("launch-raw", rawRecord{URL: "https://example.org/proof", At: now, Status: 200, Body: "proof"})
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range kinds {
		b := Batch{Version: Version, Kind: kind, At: now, Gaps: []string{"bounded_scan"}}
		for _, c := range candidates {
			if c.Kind == kind {
				c.EvidenceIDs = []string{id}
				c.Seal()
				b.Candidates = append(b.Candidates, c)
			}
		}
		if _, err = a.Put("launch-batches", b); err != nil {
			t.Fatal(err)
		}
	}
	return a
}

func TestShadowBuysOnlyAcceptedCandidatesAndExitsOnLiveQuotes(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	curve := acceptedCurve(now)
	held := acceptedCurve(now)
	held.ExternalID, held.Asset.Address = "0x"+strings.Repeat("22", 20), "0x"+strings.Repeat("22", 20)
	held.Facts["holder_count"] = "3"
	cex := sampleCEX()
	cex.StartsAt, cex.ObservedAt, cex.FirstSeenAt = now.Add(-time.Hour), now, now
	a := shadowArchive(t, now, curve, held, cex)
	// Entry spent ~9.79 USDT on the curve; a 0.0164 BNB sell quote is worth ~9.84 USDT.
	m := &shadowMarket{bid: "1", sellWei: "3a43f4ac2f4000"}
	p := ApprovedPaperPolicy()
	s, err := Shadow(context.Background(), m.client(t, a), p, now)
	if err != nil {
		t.Fatal(err)
	}
	open := s.open()
	if len(open) != 2 || s.Sequence != 1 || s.T0 != now {
		t.Fatalf("want curve and CEX entries only: %+v", s.Events)
	}
	for _, o := range open {
		if o.Candidate.ExternalID == held.ExternalID {
			t.Fatal("held candidate was bought")
		}
		if o.Candidate.Kind == "curve" && (o.CostUSDT.GreaterThan(decimal.NewFromInt(10)) || !o.Quantity.IsPositive()) {
			t.Fatalf("curve entry exceeds 1%% budget: %+v", o)
		}
	}
	if !s.CashUSDT.Add(open[0].CostUSDT).Add(open[1].CostUSDT).Equal(p.InitialUSDT) {
		t.Fatal("cash not conserved on entry")
	}

	// Next pass: CEX bid falls 25% (stop), curve marks fine. Same candidates are not rebought.
	m.bid = "0.75"
	s, err = Shadow(context.Background(), m.client(t, a), p, now.Add(4*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.open()) != 1 || s.open()[0].Candidate.Kind != "curve" || s.Sequence != 2 {
		t.Fatalf("CEX stop not taken or candidate rebought: %+v", s.Events)
	}
	stopped := s.Positions["shadow-"+cex.ID[:16]]
	if stopped.ExitReason != "stop_20pct" || !stopped.ExitUSDT.IsPositive() {
		t.Fatalf("bad stop exit: %+v", stopped)
	}

	// Quotes fail: the mark becomes unknown, then after 72h the curve is written off.
	m.fail = true
	s, err = Shadow(context.Background(), m.client(t, a), p, now.Add(8*time.Hour))
	if err != nil || len(s.open()) != 1 || s.open()[0].MarkUSDT != nil {
		t.Fatalf("unknown mark not recorded: %v %+v", err, s.open())
	}
	// Sources were last fresh at +4h, so +77h is 73h of degradation and past max hold.
	s, err = Shadow(context.Background(), m.client(t, a), p, now.Add(77*time.Hour))
	if err != nil || len(s.open()) != 0 {
		t.Fatalf("max hold not enforced: %v", err)
	}
	written := s.Positions["shadow-"+curve.ID[:16]]
	if written.ExitReason != "max_hold_72h_unpriced_written_off" || !written.ExitUSDT.IsZero() {
		t.Fatalf("unpriced position kept value: %+v", written)
	}
	if s.Stopped != "sources_degraded_72h" {
		t.Fatalf("72h of stale sources did not stop the pilot: %q", s.Stopped)
	}
	loaded, _, err := LoadShadow(a)
	if err != nil || loaded.Sequence != 4 {
		t.Fatalf("journal chain: %v %d", err, loaded.Sequence)
	}
}

func TestShadowStopsOnDrawdownAndRefusesPolicyChange(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	a := shadowArchive(t, now)
	m := &shadowMarket{bid: "1", sellWei: "1"}
	p := ApprovedPaperPolicy()
	s, err := Shadow(context.Background(), m.client(t, a), p, now)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a loss recorded in a later state to check the drawdown gate.
	s.Sequence, s.CashUSDT = 2, decimal.NewFromInt(900)
	if _, err = a.Put("launch-shadow", s); err != nil {
		t.Fatal(err)
	}
	if _, _, err = LoadShadow(a); err == nil {
		t.Fatal("record without its chain link accepted")
	}
	ids, _ := a.IDs("launch-shadow")
	first := ""
	for _, id := range ids {
		var x ShadowState
		_ = a.Read("launch-shadow", id, &x)
		if x.Sequence == 1 {
			first = id
		}
	}
	b := catalyst.Archive{Dir: t.TempDir()}
	s.Previous = first
	var one ShadowState
	_ = a.Read("launch-shadow", first, &one)
	_, _ = b.Put("launch-shadow", one)
	_, _ = b.Put("launch-shadow", s)
	for _, kind := range kinds {
		_, _ = b.Put("launch-batches", Batch{Version: Version, Kind: kind, At: now})
	}
	s, err = Shadow(context.Background(), m.client(t, b), p, now.Add(time.Hour))
	if err != nil || s.Stopped != "drawdown_8pct" {
		t.Fatalf("drawdown did not stop the pilot: %v %q", err, s.Stopped)
	}
	if !slices.ContainsFunc(s.Events, func(e ShadowEvent) bool { return e.Kind == "stop" }) {
		t.Fatal("stop not recorded")
	}
	changed := p
	changed.MinHolderCount++
	if _, err = Shadow(context.Background(), m.client(t, b), changed, now.Add(2*time.Hour)); err == nil {
		t.Fatal("unapproved policy accepted")
	}
}
