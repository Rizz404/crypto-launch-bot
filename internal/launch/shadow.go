package launch

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strings"
	"time"

	"github.com/rizz404/crypto-launch-bot/internal/catalyst"
	"github.com/shopspring/decimal"
)

// ShadowVersion names the pilot parameters the user approved on 2026-11-14. The
// shadow book is a deterministic no-AI baseline: it records what policy would buy
// at live quotes and marks it with live sell quotes. It never signs or sends anything.
const ShadowVersion = "launch-pilot-v1"

const (
	shadowDays        = 14
	shadowMaxHold     = 72 * time.Hour
	shadowMaxDegraded = 72 * time.Hour
	shadowSourceAge   = 5 * time.Hour
)

var (
	shadowStop     = decimal.RequireFromString("-0.20")
	shadowTake     = decimal.RequireFromString("0.50")
	shadowDrawdown = decimal.RequireFromString("0.08")
	cexTakerFee    = decimal.RequireFromString("0.001")
	weiPerBNB      = decimal.New(1, 18)
	gweiStep       = decimal.New(1, 9)
)

type ShadowPosition struct {
	ID         string           `json:"id"`
	Candidate  Candidate        `json:"candidate"`
	OpenedAt   time.Time        `json:"opened_at"`
	CostUSDT   decimal.Decimal  `json:"cost_usdt"`
	Quantity   decimal.Decimal  `json:"quantity"`
	MarkUSDT   *decimal.Decimal `json:"mark_usdt,omitempty"`
	MarkAt     time.Time        `json:"mark_at,omitempty"`
	ClosedAt   time.Time        `json:"closed_at,omitempty"`
	ExitUSDT   decimal.Decimal  `json:"exit_usdt"`
	ExitReason string           `json:"exit_reason,omitempty"`
}

type ShadowEvent struct {
	At     time.Time `json:"at"`
	Kind   string    `json:"kind"`
	Ref    string    `json:"ref,omitempty"`
	Detail string    `json:"detail"`
}

type ShadowState struct {
	Version       string                    `json:"version"`
	PolicyHash    string                    `json:"policy_hash"`
	Sequence      uint64                    `json:"sequence"`
	Previous      string                    `json:"previous"`
	T0            time.Time                 `json:"t0"`
	At            time.Time                 `json:"at"`
	CashUSDT      decimal.Decimal           `json:"cash_usdt"`
	EquityUSDT    decimal.Decimal           `json:"equity_usdt"`
	PeakUSDT      decimal.Decimal           `json:"peak_usdt"`
	DayWIB        string                    `json:"day_wib"`
	DayStartUSDT  decimal.Decimal           `json:"day_start_usdt"`
	LastHealthyAt time.Time                 `json:"last_healthy_at"`
	Stopped       string                    `json:"stopped,omitempty"`
	Positions     map[string]ShadowPosition `json:"positions"`
	Consumed      map[string]bool           `json:"consumed"`
	Events        []ShadowEvent             `json:"events"`
}

func (s ShadowState) open() []ShadowPosition {
	var out []ShadowPosition
	for _, p := range s.Positions {
		if p.ClosedAt.IsZero() {
			out = append(out, p)
		}
	}
	slices.SortFunc(out, func(a, b ShadowPosition) int { return strings.Compare(a.ID, b.ID) })
	return out
}

func (s *ShadowState) event(now time.Time, kind, ref, detail string) {
	s.Events = append(s.Events, ShadowEvent{At: now, Kind: kind, Ref: ref, Detail: detail})
}

// LoadShadow returns the newest state, checking that the chain of records is unbroken.
func LoadShadow(archive catalyst.Archive) (ShadowState, string, error) {
	ids, err := archive.IDs("launch-shadow")
	if err != nil || len(ids) == 0 {
		return ShadowState{}, "", err
	}
	bySeq := map[uint64]string{}
	states := map[string]ShadowState{}
	for _, id := range ids {
		var s ShadowState
		if err = archive.Read("launch-shadow", id, &s); err != nil {
			return ShadowState{}, "", err
		}
		if s.Version != ShadowVersion || bySeq[s.Sequence] != "" {
			return ShadowState{}, "", errors.New("shadow journal has a foreign version or a fork")
		}
		bySeq[s.Sequence], states[id] = id, s
	}
	last := uint64(len(ids))
	for seq := uint64(1); seq <= last; seq++ {
		id := bySeq[seq]
		if id == "" || (seq > 1 && states[id].Previous != bySeq[seq-1]) {
			return ShadowState{}, "", errors.New("shadow journal has a gap")
		}
	}
	return states[bySeq[last]], bySeq[last], nil
}

// Shadow runs one pass over the latest report: mark and exit open positions with live
// sell quotes, then open baseline positions for accepted curve/CEX candidates.
func Shadow(ctx context.Context, client *Client, p PaperPolicy, now time.Time) (ShadowState, error) {
	archive := client.Archive
	if err := p.Validate(); err != nil {
		return ShadowState{}, err
	}
	s, previous, err := LoadShadow(archive)
	if err != nil {
		return s, err
	}
	if s.Version == "" {
		s = ShadowState{Version: ShadowVersion, PolicyHash: catalyst.Hash(p), T0: now, CashUSDT: p.InitialUSDT, PeakUSDT: p.InitialUSDT,
			DayStartUSDT: p.InitialUSDT, LastHealthyAt: now, Positions: map[string]ShadowPosition{}, Consumed: map[string]bool{}}
		s.event(now, "start", "", fmt.Sprintf("T0; %d days, stop %s, take %s, max hold %s", shadowDays, shadowStop, shadowTake, shadowMaxHold))
	}
	if s.PolicyHash != catalyst.Hash(p) {
		return s, errors.New("policy changed during the pilot; start a new pilot version")
	}
	if now.Before(s.At) {
		return s, errors.New("shadow clock regressed")
	}
	s.Sequence, s.Previous, s.At = s.Sequence+1, previous, now
	report, err := ReadReport(archive, now)
	if err != nil {
		return s, err
	}
	if sourcesHealthy(report, now) {
		s.LastHealthyAt = now
	}
	s.mark(ctx, client, now)
	s.value(now)
	if s.Stopped == "" && now.Sub(s.LastHealthyAt) >= shadowMaxDegraded {
		s.Stopped = "sources_degraded_72h"
		s.event(now, "stop", "", s.Stopped)
	}
	if s.Stopped == "" && s.EquityUSDT.LessThanOrEqual(s.PeakUSDT.Mul(decimal.NewFromInt(1).Sub(shadowDrawdown))) {
		s.Stopped = "drawdown_8pct"
		s.event(now, "stop", "", s.Stopped)
	}
	if s.Stopped == "" && now.Before(s.T0.AddDate(0, 0, shadowDays)) {
		s.enter(report, p, now)
	}
	s.value(now)
	if _, err = archive.Put("launch-shadow", s); err != nil {
		return s, err
	}
	return s, nil
}

// sourcesHealthy is true when every source produced a recent batch without a failure gap.
func sourcesHealthy(r Report, now time.Time) bool {
	for _, kind := range kinds {
		b := r.Sources[kind]
		if b.At.IsZero() || now.Sub(b.At) > shadowSourceAge || slices.ContainsFunc(b.Gaps, func(g string) bool { return !coverageGap(g) }) {
			return false
		}
	}
	return true
}

func (s *ShadowState) value(now time.Time) {
	equity := s.CashUSDT
	for _, p := range s.open() {
		if p.MarkUSDT != nil {
			equity = equity.Add(*p.MarkUSDT)
		}
	}
	day := now.In(time.FixedZone("WIB", 7*60*60)).Format("2006-01-02")
	if s.DayWIB != day {
		s.DayWIB, s.DayStartUSDT = day, equity
	}
	s.EquityUSDT = equity
	s.PeakUSDT = decimal.Max(s.PeakUSDT, equity)
}

func (s *ShadowState) mark(ctx context.Context, c *Client, now time.Time) {
	for _, p := range s.open() {
		value, err := c.shadowQuote(ctx, p)
		if err != nil {
			p.MarkUSDT = nil
			s.event(now, "mark_unknown", p.ID, err.Error())
		} else {
			p.MarkUSDT, p.MarkAt = &value, now
		}
		reason := ""
		switch {
		case p.MarkUSDT != nil && p.MarkUSDT.Sub(p.CostUSDT).Div(p.CostUSDT).LessThanOrEqual(shadowStop):
			reason = "stop_20pct"
		case p.MarkUSDT != nil && p.MarkUSDT.Sub(p.CostUSDT).Div(p.CostUSDT).GreaterThanOrEqual(shadowTake):
			reason = "take_50pct"
		case now.Sub(p.OpenedAt) >= shadowMaxHold && p.MarkUSDT != nil:
			reason = "max_hold_72h"
		case now.Sub(p.OpenedAt) >= shadowMaxHold:
			// No sell quote: the token cannot be shown to be sellable, so it is written off.
			reason, value = "max_hold_72h_unpriced_written_off", decimal.Zero
			p.MarkUSDT = &value
		}
		if reason != "" {
			p.ClosedAt, p.ExitUSDT, p.ExitReason = now, *p.MarkUSDT, reason
			s.CashUSDT = s.CashUSDT.Add(p.ExitUSDT)
			s.event(now, "exit", p.ID, fmt.Sprintf("%s value %s cost %s", reason, p.ExitUSDT.StringFixed(4), p.CostUSDT.StringFixed(4)))
		}
		s.Positions[p.ID] = p
	}
}

func (s *ShadowState) context(now time.Time) PaperContext {
	c := PaperContext{At: now, DayWIB: s.DayWIB, EquityUSDT: s.EquityUSDT, DayStartUSDT: s.DayStartUSDT, PeakUSDT: s.PeakUSDT, AvailableUSDT: s.CashUSDT}
	for _, p := range s.open() {
		mark := decimal.Zero
		if p.MarkUSDT != nil {
			mark = *p.MarkUSDT
		}
		c.Positions = append(c.Positions, PaperExposure{ID: p.ID, CandidateID: p.Candidate.ID, AssetKey: p.Candidate.Asset.Key(), Kind: p.Candidate.Kind,
			NotionalUSDT: p.CostUSDT, MarkUSDT: &mark, MarkKnown: p.MarkUSDT != nil, Tradable: true})
	}
	return c
}

func (s *ShadowState) enter(r Report, p PaperPolicy, now time.Time) {
	reviews := slices.Clone(r.Reviews)
	// Baseline order: first seen, then ID.
	slices.SortFunc(reviews, func(a, b Review) int {
		return cmp.Or(a.Candidate.FirstSeenAt.Compare(b.Candidate.FirstSeenAt), strings.Compare(a.Candidate.ID, b.Candidate.ID))
	})
	for _, review := range reviews {
		c := review.Candidate
		if review.Status != "accepted_for_research" || (c.Kind != "curve" && c.Kind != "cex") || s.Consumed[c.ID] ||
			slices.ContainsFunc(s.open(), func(o ShadowPosition) bool { return o.Candidate.Asset.Key() == c.Asset.Key() }) {
			continue
		}
		position, err := shadowEntry(c, p, s.context(now), now)
		if err != nil {
			// Not consumed: the limit or halt that blocked it may clear on a later pass.
			s.event(now, "skip", c.ID, err.Error())
			continue
		}
		s.Consumed[c.ID] = true
		s.CashUSDT = s.CashUSDT.Sub(position.CostUSDT)
		s.Positions[position.ID] = position
		s.event(now, "entry", position.ID, fmt.Sprintf("%s %s cost %s", c.Kind, c.ExternalID, position.CostUSDT.StringFixed(4)))
		s.value(now)
	}
}

func fact(c Candidate, key string) (decimal.Decimal, error) {
	d, err := decimal.NewFromString(c.Facts[key])
	if err != nil || d.IsNegative() {
		return decimal.Zero, fmt.Errorf("%s missing or invalid", key)
	}
	return d, nil
}

// shadowEntry sizes a baseline entry with the approved policy and the candidate's
// own quotes. Curve uses the 0.01 BNB probe ratio for a ~10 USDT buy.
// shortcut: probe ratio, not a quote for the exact size; add tryBuy at the real size if curve sizes grow.
func shadowEntry(c Candidate, p PaperPolicy, state PaperContext, now time.Time) (ShadowPosition, error) {
	position := ShadowPosition{ID: "shadow-" + c.ID[:16], Candidate: c, OpenedAt: now}
	switch c.Kind {
	case "curve":
		cost, ok := p.CurveRoundTripCost(c)
		if !ok {
			return position, errors.New("curve cost unknown")
		}
		size, err := p.PlanSize("curve", state, decimal.Zero, cost, true, now)
		if err != nil {
			return position, err
		}
		var values [5]decimal.Decimal
		for i, key := range []string{"curve_buy_tokens", "curve_buy_paid_wei", "bnb_usdt_ask", "curve_round_trip_gas_usdt", "buy_tax"} {
			if values[i], err = fact(c, key); err != nil {
				return position, err
			}
		}
		tokens, paid, ask, gas, tax := values[0], values[1], values[2], values[3], values[4]
		if !tokens.IsPositive() || !paid.IsPositive() || !ask.IsPositive() {
			return position, errors.New("curve quote empty")
		}
		spendWei := size.NotionalUSDT.Div(ask).Mul(weiPerBNB)
		qty := spendWei.Mul(tokens).Div(paid).Mul(decimal.NewFromInt(1).Sub(tax))
		position.Quantity = qty.Div(gweiStep).Floor().Mul(gweiStep)
		position.CostUSDT = size.NotionalUSDT.Add(gas.Div(decimal.NewFromInt(2)))
	case "cex":
		var values [3]decimal.Decimal
		var err error
		for i, key := range []string{"bid", "ask", "ask_qty"} {
			if values[i], err = fact(c, key); err != nil {
				return position, err
			}
		}
		bid, ask, depth := values[0], values[1], values[2]
		if !bid.IsPositive() || ask.LessThan(bid) {
			return position, errors.New("book invalid")
		}
		cost := cexTakerFee.Mul(decimal.NewFromInt(2)).Add(ask.Sub(bid).Div(ask))
		size, err := p.PlanSize("cex", state, shadowStop.Neg(), cost, true, now)
		if err != nil {
			return position, err
		}
		qty := decimal.Min(size.NotionalUSDT.Div(ask), depth).Truncate(8)
		position.Quantity = qty
		position.CostUSDT = qty.Mul(ask).Mul(decimal.NewFromInt(1).Add(cexTakerFee))
	default:
		return position, errors.New("no shadow entry path for " + c.Kind)
	}
	if !position.Quantity.IsPositive() || !position.CostUSDT.IsPositive() {
		return position, errors.New("entry rounds to zero")
	}
	// Valued at cost until the next pass fetches a live sell quote.
	mark := position.CostUSDT
	position.MarkUSDT, position.MarkAt = &mark, now
	return position, nil
}

// shadowQuote is the net USDT a seller would receive now for the whole position.
func (c *Client) shadowQuote(ctx context.Context, p ShadowPosition) (decimal.Decimal, error) {
	bnbBid := func() (decimal.Decimal, error) {
		var book struct{ Symbol, BidPrice string }
		_, err := c.fetch(ctx, "https://data-api.binance.vision/api/v3/ticker/bookTicker?symbol=BNBUSDT", nil, &book)
		bid, parseErr := decimal.NewFromString(book.BidPrice)
		if err != nil || parseErr != nil || book.Symbol != "BNBUSDT" || !bid.IsPositive() {
			return decimal.Zero, errors.New("bnb_usdt_price_unavailable")
		}
		return bid, nil
	}
	switch p.Candidate.Kind {
	case "curve":
		block, _, err := c.finalized(ctx)
		if err != nil {
			return decimal.Zero, err
		}
		token := strings.Repeat("0", 24) + strings.TrimPrefix(p.Candidate.Asset.Address, "0x")
		value, _, err := c.call(ctx, fourMemeHelper, trySell+token+fmt.Sprintf("%064x", p.Quantity.BigInt()), block.Number)
		sell := words(value, 4)
		if err != nil || sell == nil {
			return decimal.Zero, errors.New("curve_sell_quote_unavailable")
		}
		funds, err := abiUint(sell[2])
		if err != nil || funds.Sign() < 0 {
			return decimal.Zero, errors.New("curve_sell_quote_unavailable")
		}
		bid, err := bnbBid()
		if err != nil {
			return decimal.Zero, err
		}
		tax, taxErr := fact(p.Candidate, "sell_tax")
		gas, gasErr := fact(p.Candidate, "curve_round_trip_gas_usdt")
		if taxErr != nil || gasErr != nil {
			return decimal.Zero, errors.New("entry facts incomplete")
		}
		net := decimal.NewFromBigInt(new(big.Int).Set(funds), 0).Div(weiPerBNB).Mul(bid).Mul(decimal.NewFromInt(1).Sub(tax)).Sub(gas.Div(decimal.NewFromInt(2)))
		return decimal.Max(net, decimal.Zero), nil
	case "cex":
		var book struct{ Symbol, BidPrice string }
		_, err := c.fetch(ctx, "https://data-api.binance.vision/api/v3/ticker/bookTicker?symbol="+p.Candidate.ExternalID, nil, &book)
		bid, parseErr := decimal.NewFromString(book.BidPrice)
		if err != nil || parseErr != nil || book.Symbol != p.Candidate.ExternalID || !bid.IsPositive() {
			return decimal.Zero, errors.New("book_unavailable")
		}
		return p.Quantity.Mul(bid).Mul(decimal.NewFromInt(1).Sub(cexTakerFee)), nil
	}
	return decimal.Zero, errors.New("no shadow quote path")
}
