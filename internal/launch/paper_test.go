package launch

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rizz404/trading-bot/internal/catalyst"
	"github.com/shopspring/decimal"
)

func TestExampleFixtureRunsAllThreePathsWithoutNetwork(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "configs", "launch.paper.fixture.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture PaperFixture
	if err = catalyst.Decode(bytes.NewReader(raw), &fixture, 16<<20); err != nil {
		t.Fatal(err)
	}
	archive := catalyst.Archive{Dir: t.TempDir()}
	s, err := ReplayPaper(context.Background(), archive, fixture)
	if err != nil {
		t.Fatal(err)
	}
	if !s.FixtureOnly || !s.CashUSDT.Equal(dec("1008.961")) || !s.FeesUSDT.Equal(dec("0.539")) || !s.RealizedGrossUSDT.Equal(dec("9.5")) {
		t.Fatalf("example conservation: %+v", s)
	}
	again, err := ReplayPaper(context.Background(), archive, fixture)
	if err != nil || catalyst.Hash(again) != catalyst.Hash(s) {
		t.Fatal("replay reset or duplicated the synthetic book", err)
	}
}

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func fixtureIntent(kind, id string, at time.Time) PaperIntent {
	c := sampleCEX()
	c.Kind = kind
	c.ObservedAt = at
	c.StartsAt = at.Add(-time.Hour)
	if kind != "cex" {
		c.Asset = Asset{Chain: "eip155:56", Address: testAddress}
		c.Venue = "pancakeswap-v2"
		c.ExternalID = "0x2234567890123456789012345678901234567890"
	}
	if kind == "presale" {
		c.Venue = "pinksale"
		c.EndsAt = at.Add(24 * time.Hour)
		c.State = "live"
	}
	c.Seal()
	i := PaperIntent{ID: id, Candidate: c, Action: "buy", PositionID: "position-" + id, NotionalUSDT: dec("90"), Quantity: dec("0.9"), LimitPrice: dec("100"), MaxFeeUSDT: dec("0.27"),
		CostFraction: dec("0.003"), StopFraction: dec("0.05"), CostsKnown: true, ExpiresAt: at.Add(5 * time.Minute), Thesis: "fixture thesis", Invalidation: "fixture invalidation", AssumedVerified: true}
	if kind != "cex" {
		i.NotionalUSDT = dec("9")
		i.Quantity = dec("0.09")
		i.MaxFeeUSDT = dec("0.45")
		i.CostFraction = dec("0.05")
	}
	if kind == "presale" {
		i.Action = "contribute"
		i.Quantity = decimal.Zero
		i.LimitPrice = decimal.Zero
	}
	return i
}

func newTestPaperStore(t *testing.T) PaperStore {
	t.Helper()
	return PaperStore{Archive: catalyst.Archive{Dir: t.TempDir()}, RunID: "fixture-run", Policy: ApprovedPaperPolicy()}
}

func putStep(t *testing.T, store PaperStore, step PaperStep) PaperState {
	t.Helper()
	s, err := store.Step(context.Background(), step)
	if err != nil {
		t.Fatal(step.ID, err)
	}
	return s
}

func TestPaperCEXPartialDepthRestartExitAndIdempotency(t *testing.T) {
	store := newTestPaperStore(t)
	i := fixtureIntent("cex", "buy", testNow)
	putStep(t, store, PaperStep{ID: "reserve", At: testNow, Op: "submit", Intent: &i})
	q := PaperBookQuote{At: testNow.Add(time.Second), CandidateID: i.Candidate.ID, Bid: dec("99.5"), Ask: dec("100"), BidQty: dec("10"), AskQty: dec("5"), LotStep: dec("0.1"), MinNotional: dec("1"), FeeFraction: dec("0.001"), CostsKnown: true}
	s := putStep(t, store, PaperStep{ID: "partial", At: q.At, Op: "match", OrderID: i.ID, Book: &q})
	if s.Orders[i.ID].Status != "partial" || !s.Positions[i.PositionID].Quantity.Equal(dec("0.5")) {
		t.Fatal("book did not cap partial fill to 10% depth")
	}
	s = putStep(t, store, PaperStep{ID: "same-book", At: q.At.Add(time.Second), Op: "match", OrderID: i.ID, Book: &q})
	if !s.Positions[i.PositionID].Quantity.Equal(dec("0.5")) {
		t.Fatal("same quote reused consumed liquidity")
	}
	q.At = q.At.Add(2 * time.Second)
	s = putStep(t, store, PaperStep{ID: "finish-buy", At: q.At, Op: "match", OrderID: i.ID, Book: &q})
	if s.Orders[i.ID].Status != "filled" {
		t.Fatal("remaining quantity not filled")
	}
	restored, err := store.Load()
	if err != nil || catalyst.Hash(restored) != catalyst.Hash(s) {
		t.Fatal("restart changed paper state", err)
	}
	exit := i
	exit.ID = "sell"
	exit.Action = "sell"
	exit.NotionalUSDT = decimal.Zero
	exit.MaxFeeUSDT = dec("0.18")
	putStep(t, store, PaperStep{ID: "reserve-exit", At: testNow.Add(4 * time.Second), Op: "submit", Intent: &exit})
	q.At = testNow.Add(5 * time.Second)
	q.Bid = dec("110")
	q.Ask = dec("111")
	q.BidQty = dec("20")
	last := PaperStep{ID: "fill-exit", At: q.At, Op: "match", OrderID: exit.ID, Book: &q}
	s = putStep(t, store, last)
	if !s.CashUSDT.Equal(dec("1008.811")) || !s.FeesUSDT.Equal(dec("0.189")) || s.Positions[i.PositionID].Quantity.IsPositive() {
		t.Fatalf("bad settlement: %+v", s)
	}
	again := putStep(t, store, last)
	if catalyst.Hash(again) != catalyst.Hash(s) {
		t.Fatal("duplicate request changed settlement")
	}
	last.Book = nil
	if _, err = store.Step(context.Background(), last); err == nil {
		t.Fatal("idempotency key accepted a changed payload")
	}
}

func presaleReceipt(id, tx string, fee string) PaperReceipt {
	return PaperReceipt{ID: "fixture:" + id, ExternalID: "fixture:" + tx, Status: "filled", FeeUSDT: dec(fee), Finalized: true, BlockNumber: 100, BlockHash: "fixture:final-block"}
}

func contributeFixture(t *testing.T, store PaperStore) PaperIntent {
	t.Helper()
	i := fixtureIntent("presale", "sale", testNow)
	putStep(t, store, PaperStep{ID: "sale-reserve", At: testNow, Op: "submit", Intent: &i})
	putStep(t, store, PaperStep{ID: "sale-broadcast", At: testNow.Add(time.Second), Op: "broadcast", OrderID: i.ID, ExternalID: "fixture:sale-tx"})
	r := presaleReceipt("sale-final", "sale-tx", "0.1")
	r.SpentUSDT = i.NotionalUSDT
	s := putStep(t, store, PaperStep{ID: "sale-final", At: testNow.Add(2 * time.Second), Op: "receipt", OrderID: i.ID, Receipt: &r})
	if !s.Positions[i.PositionID].Quantity.IsZero() || !s.Positions[i.PositionID].LockedUSDT.Equal(dec("9")) {
		t.Fatal("contribution invented tokens")
	}
	return i
}

func TestPresaleRefundAllocationVestingAndClaim(t *testing.T) {
	store := newTestPaperStore(t)
	i := contributeFixture(t, store)
	a := PaperAllocation{PositionID: i.PositionID, TotalQuantity: dec("100"), UnlockedQuantity: dec("40"), RefundableUSDT: dec("1"), Finalized: true, Proof: "fixture:allocation"}
	putStep(t, store, PaperStep{ID: "allocation", At: testNow.Add(3 * time.Second), Op: "allocation", Allocation: &a})
	refund := i
	refund.ID = "refund"
	refund.Action = "refund"
	refund.NotionalUSDT = dec("1")
	refund.MaxFeeUSDT = dec("0.1")
	putStep(t, store, PaperStep{ID: "refund-reserve", At: testNow.Add(4 * time.Second), Op: "submit", Intent: &refund})
	putStep(t, store, PaperStep{ID: "refund-broadcast", At: testNow.Add(5 * time.Second), Op: "broadcast", OrderID: refund.ID, ExternalID: "fixture:refund-tx"})
	r := presaleReceipt("refund-final", "refund-tx", "0.05")
	r.ReceivedUSDT = dec("1")
	putStep(t, store, PaperStep{ID: "refund-final", At: testNow.Add(6 * time.Second), Op: "receipt", OrderID: refund.ID, Receipt: &r})
	claim := i
	claim.ID = "claim-one"
	claim.Action = "claim"
	claim.NotionalUSDT = decimal.Zero
	claim.Quantity = dec("40")
	claim.MaxFeeUSDT = dec("0.1")
	putStep(t, store, PaperStep{ID: "claim-reserve", At: testNow.Add(7 * time.Second), Op: "submit", Intent: &claim})
	putStep(t, store, PaperStep{ID: "claim-broadcast", At: testNow.Add(8 * time.Second), Op: "broadcast", OrderID: claim.ID, ExternalID: "fixture:claim-tx"})
	r = presaleReceipt("claim-final", "claim-tx", "0.05")
	r.Quantity = dec("40")
	s := putStep(t, store, PaperStep{ID: "claim-final", At: testNow.Add(9 * time.Second), Op: "receipt", OrderID: claim.ID, Receipt: &r})
	if !s.Positions[i.PositionID].CostUSDT.Equal(dec("3.2")) || !s.Positions[i.PositionID].LockedUSDT.Equal(dec("4.8")) {
		t.Fatal("claim duplicated cost basis")
	}
	if s.Context(s.At).Positions[0].MarkKnown {
		t.Fatal("unpriced/locked allocation marked as known")
	}
	a.UnlockedQuantity = dec("100")
	putStep(t, store, PaperStep{ID: "unlock", At: testNow.Add(10 * time.Second), Op: "allocation", Allocation: &a})
	claim.ID = "claim-two"
	claim.Quantity = dec("60")
	putStep(t, store, PaperStep{ID: "claim-two-reserve", At: testNow.Add(11 * time.Second), Op: "submit", Intent: &claim})
	putStep(t, store, PaperStep{ID: "claim-two-broadcast", At: testNow.Add(12 * time.Second), Op: "broadcast", OrderID: claim.ID, ExternalID: "fixture:claim-two-tx"})
	r = presaleReceipt("claim-two-final", "claim-two-tx", "0.05")
	r.Quantity = dec("60")
	s = putStep(t, store, PaperStep{ID: "claim-two-final", At: testNow.Add(13 * time.Second), Op: "receipt", OrderID: claim.ID, Receipt: &r})
	if !s.Positions[i.PositionID].CostUSDT.Equal(dec("8")) || !s.Positions[i.PositionID].LockedUSDT.IsZero() || !s.Positions[i.PositionID].Quantity.Equal(dec("100")) {
		t.Fatal("vesting release did not conserve basis/quantity")
	}
}

func TestUnknownFinalityReorgAndFailedGasDoNotFreeFundsEarly(t *testing.T) {
	store := newTestPaperStore(t)
	i := fixtureIntent("presale", "unknown-sale", testNow)
	s := putStep(t, store, PaperStep{ID: "reserve", At: testNow, Op: "submit", Intent: &i})
	cash := s.CashUSDT
	putStep(t, store, PaperStep{ID: "broadcast", At: testNow.Add(time.Second), Op: "broadcast", OrderID: i.ID, ExternalID: "fixture:tx"})
	putStep(t, store, PaperStep{ID: "timeout", At: testNow.Add(2 * time.Second), Op: "unknown", OrderID: i.ID})
	r := presaleReceipt("confirmation", "tx", "0.05")
	r.Status = "confirmed"
	r.Finalized = false
	r.SpentUSDT = dec("9")
	s = putStep(t, store, PaperStep{ID: "confirm", At: testNow.Add(3 * time.Second), Op: "receipt", OrderID: i.ID, Receipt: &r})
	if !s.CashUSDT.Equal(cash) || len(s.Positions) != 0 {
		t.Fatal("nonfinal receipt settled funds")
	}
	r.ID = "fixture:reorg"
	r.Status = "reorg"
	putStep(t, store, PaperStep{ID: "reorg", At: testNow.Add(4 * time.Second), Op: "receipt", OrderID: i.ID, Receipt: &r})
	r = presaleReceipt("failed", "tx", "0.05")
	r.Status = "failed"
	s = putStep(t, store, PaperStep{ID: "failed", At: testNow.Add(5 * time.Second), Op: "receipt", OrderID: i.ID, Receipt: &r})
	if !s.CashUSDT.Equal(dec("999.95")) {
		t.Fatal("failed transaction principal or gas incorrect")
	}
	s = putStep(t, store, PaperStep{ID: "duplicate-receipt", At: testNow.Add(6 * time.Second), Op: "receipt", OrderID: i.ID, Receipt: &r})
	if !s.CashUSDT.Equal(dec("999.95")) {
		t.Fatal("duplicate receipt charged gas twice")
	}
	r.ID = "fixture:late-finality-violation"
	r.Status = "reorg"
	s = putStep(t, store, PaperStep{ID: "late-reorg", At: testNow.Add(7 * time.Second), Op: "receipt", OrderID: i.ID, Receipt: &r})
	if !s.Attention || !s.CashUSDT.Equal(dec("999.95")) {
		t.Fatal("finality violation rewrote settled ledger")
	}
}

func TestDEXRequiresRouteAndFinalMinOut(t *testing.T) {
	store := newTestPaperStore(t)
	i := fixtureIntent("dex", "swap", testNow)
	putStep(t, store, PaperStep{ID: "reserve", At: testNow, Op: "submit", Intent: &i})
	q := PaperRouteQuote{ID: "fixture:route", At: testNow.Add(time.Second), ExpiresAt: testNow.Add(5 * time.Second), CandidateID: i.Candidate.ID, AssetKey: i.Candidate.Asset.Key(), InputUSDT: dec("9"), Quantity: dec("0.09"), MinQuantity: dec("0.085"), FeeUSDT: dec("0.05"), GasUSDT: dec("0.05"), CostsKnown: true, LiquidityVerified: true, SellableVerified: true}
	putStep(t, store, PaperStep{ID: "broadcast", At: q.At, Op: "broadcast", OrderID: i.ID, ExternalID: "fixture:swap-tx", Route: &q})
	r := presaleReceipt("bad-fill", "swap-tx", "0.1")
	r.SpentUSDT = dec("9")
	r.Quantity = dec("0.08")
	if _, err := store.Step(context.Background(), PaperStep{ID: "bad-min-out", At: testNow.Add(6 * time.Second), Op: "receipt", OrderID: i.ID, Receipt: &r}); err == nil {
		t.Fatal("DEX min-out violation settled")
	}
	r.ID = "fixture:good-fill"
	r.Quantity = dec("0.09")
	s := putStep(t, store, PaperStep{ID: "finalized-later", At: testNow.Add(10 * time.Second), Op: "receipt", OrderID: i.ID, Receipt: &r})
	if !s.Positions[i.PositionID].Quantity.Equal(dec("0.09")) {
		t.Fatal("valid broadcast failed merely because quote expired before finality")
	}
}

func TestDEXActualOutputMaySlipOnlyWithinMinOutAndPriceLimit(t *testing.T) {
	store := newTestPaperStore(t)
	i := fixtureIntent("dex", "slippage", testNow)
	i.LimitPrice = dec("106")
	putStep(t, store, PaperStep{ID: "reserve", At: testNow, Op: "submit", Intent: &i})
	q := PaperRouteQuote{ID: "fixture:route", At: testNow.Add(time.Second), ExpiresAt: testNow.Add(5 * time.Second), CandidateID: i.Candidate.ID, AssetKey: i.Candidate.Asset.Key(), InputUSDT: dec("9"), Quantity: dec("0.09"), MinQuantity: dec("0.085"), FeeUSDT: dec("0.05"), GasUSDT: dec("0.05"), CostsKnown: true, LiquidityVerified: true, SellableVerified: true}
	putStep(t, store, PaperStep{ID: "broadcast", At: q.At, Op: "broadcast", OrderID: i.ID, ExternalID: "fixture:tx", Route: &q})
	r := presaleReceipt("final", "tx", "0.1")
	r.SpentUSDT = dec("9")
	r.Quantity = dec("0.086")
	s := putStep(t, store, PaperStep{ID: "final", At: testNow.Add(2 * time.Second), Op: "receipt", OrderID: i.ID, Receipt: &r})
	if !s.Positions[i.PositionID].Quantity.Equal(dec("0.086")) || s.Orders[i.ID].Status != "filled" {
		t.Fatal("atomic DEX swap did not honor bounded actual output")
	}
}

func TestJournalRejectsForeignRunAndFork(t *testing.T) {
	store := newTestPaperStore(t)
	i := fixtureIntent("cex", "one", testNow)
	s := putStep(t, store, PaperStep{ID: "reserve", At: testNow, Op: "submit", Intent: &i})
	foreign := store
	foreign.RunID = "other"
	if _, err := foreign.Load(); err == nil {
		t.Fatal("new run reset existing balance")
	}
	_, err := store.Archive.Put("paper-journal", paperRecord{Version: PaperJournalVersion, RunID: store.RunID, PolicyHash: s.PolicyHash, Sequence: 5, Previous: s.LastHash, Step: PaperStep{ID: "fork", At: testNow}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Load(); err == nil {
		t.Fatal("journal gap accepted")
	}
}

func TestPresaleCancellationFullRefundAndMissingProofHold(t *testing.T) {
	store := newTestPaperStore(t)
	i := contributeFixture(t, store)
	claim := i
	claim.ID = "unverified-claim"
	claim.Action = "claim"
	claim.NotionalUSDT = decimal.Zero
	claim.Quantity = dec("1")
	claim.MaxFeeUSDT = dec("0.1")
	if _, err := store.Step(context.Background(), PaperStep{ID: "invented-claim", At: testNow.Add(3 * time.Second), Op: "submit", Intent: &claim}); err == nil {
		t.Fatal("claim without allocation accepted")
	}
	a := PaperAllocation{PositionID: i.PositionID, RefundableUSDT: dec("9"), Finalized: true, Proof: "fixture:cancellation"}
	putStep(t, store, PaperStep{ID: "cancel-sale", At: testNow.Add(3 * time.Second), Op: "allocation", Allocation: &a})
	refund := i
	refund.ID = "full-refund"
	refund.Action = "refund"
	refund.MaxFeeUSDT = dec("0.1")
	putStep(t, store, PaperStep{ID: "refund-reserve", At: testNow.Add(4 * time.Second), Op: "submit", Intent: &refund})
	putStep(t, store, PaperStep{ID: "refund-broadcast", At: testNow.Add(5 * time.Second), Op: "broadcast", OrderID: refund.ID, ExternalID: "fixture:refund"})
	r := presaleReceipt("refund-receipt", "refund", "0.05")
	r.ReceivedUSDT = dec("9")
	s := putStep(t, store, PaperStep{ID: "refund-final", At: testNow.Add(6 * time.Second), Op: "receipt", OrderID: refund.ID, Receipt: &r})
	if !s.CashUSDT.Equal(dec("999.85")) || positionOpen(s.Positions[i.PositionID]) {
		t.Fatal("full refund created profit or retained principal")
	}
}

func TestPaperUnknownExpiryAndUnsafeAdmissionDoNotReleaseReservations(t *testing.T) {
	store := newTestPaperStore(t)
	i := fixtureIntent("cex", "expired", testNow)
	putStep(t, store, PaperStep{ID: "reserve", At: testNow, Op: "submit", Intent: &i})
	s := putStep(t, store, PaperStep{ID: "unknown", At: testNow.Add(time.Second), Op: "unknown", OrderID: i.ID})
	if !s.Orders[i.ID].ReserveUSDT.IsPositive() {
		t.Fatal("timeout released capital")
	}
	other := fixtureIntent("cex", "second", testNow.Add(2*time.Second))
	other.Candidate.Asset.VenueID = "binance:OTHER"
	other.Candidate.ExternalID = "OTHERUSDT"
	other.Candidate.Seal()
	if _, err := store.Step(context.Background(), PaperStep{ID: "unsafe-entry", At: testNow.Add(2 * time.Second), Op: "submit", Intent: &other}); err == nil {
		t.Fatal("unknown submission did not block new exposure")
	}
	q := PaperBookQuote{At: i.ExpiresAt, CandidateID: i.Candidate.ID, Bid: dec("99.5"), Ask: dec("100"), BidQty: dec("100"), AskQty: dec("100"), LotStep: dec("0.1"), FeeFraction: dec("0.001"), CostsKnown: true}
	if _, err := store.Step(context.Background(), PaperStep{ID: "expired-fill", At: i.ExpiresAt, Op: "match", OrderID: i.ID, Book: &q}); err == nil {
		t.Fatal("expired/unknown order filled automatically")
	}
	s, err := store.Load()
	if err != nil || !s.Orders[i.ID].ReserveUSDT.IsPositive() {
		t.Fatal("failed matching freed reservation")
	}
	r := PaperReceipt{ID: "fixture:cancel", ExternalID: "fixture:expired", Status: "canceled", Finalized: true}
	s = putStep(t, store, PaperStep{ID: "cancel-ack", At: i.ExpiresAt.Add(time.Second), Op: "receipt", OrderID: i.ID, Receipt: &r})
	if !s.CashUSDT.Equal(dec("1000")) {
		t.Fatal("terminal acknowledgment did not release unused capital")
	}
}
