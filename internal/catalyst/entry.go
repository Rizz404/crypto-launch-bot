package catalyst

import (
	"errors"
	"slices"
	"time"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/strategy"
	"github.com/shopspring/decimal"
)

// EntryPolicy is separate from model output. These development limits are
// deliberately not connected to the currently running P2 wallet.
type EntryPolicy struct {
	RiskFraction, PositionFraction, ExposureFraction decimal.Decimal
	MemePositionFraction, MemeExposureFraction       decimal.Decimal
	FeePerSide, SlippageFraction                     decimal.Decimal
	MinTrail, MaxTrail, ATRMultiple, MaxSpread       decimal.Decimal
	MinQuoteVolume, MaxDeviation                     decimal.Decimal
	MaxPositions                                     int
	QuoteTTL, ATRTTL                                 time.Duration
}

func DevelopmentEntryPolicy() EntryPolicy {
	d := decimal.RequireFromString
	return EntryPolicy{
		RiskFraction: d("0.005"), PositionFraction: d("0.20"), ExposureFraction: d("0.40"),
		MemePositionFraction: d("0.02"), MemeExposureFraction: d("0.04"),
		FeePerSide: d("0.001"), SlippageFraction: d("0.0002"),
		MinTrail: d("0.01"), MaxTrail: d("0.15"), ATRMultiple: d("2"), MaxSpread: d("0.002"),
		MinQuoteVolume: d("10000000"), MaxDeviation: d("0.005"),
		MaxPositions: 2, QuoteTTL: 5 * time.Second, ATRTTL: 16 * time.Minute,
	}
}

type Quote struct {
	Symbol                        string
	Bid, Ask, BidQty, AskQty      decimal.Decimal
	QuoteVolume, ATR, EntryAnchor decimal.Decimal
	At, ATRAt                     time.Time
	Instrument                    domain.Instrument
}

type Budget struct {
	Equity, FreeCash, Exposure, MemeExposure decimal.Decimal
	Positions                                int
	Halted, Attention                        bool
}

// VerifiedEvent must come from a trusted verification path, never deserialized
// from the analyst's JSON. Structural citation checks alone cannot set Verified.
type VerifiedEvent struct {
	Key, Symbol, Revision string
	Verified              bool
	Revoked               bool
	Meme                  bool
	ValidUntil            time.Time
}

func (p EntryPolicy) Validate() error {
	for _, f := range []decimal.Decimal{p.RiskFraction, p.PositionFraction, p.ExposureFraction, p.MemePositionFraction, p.MemeExposureFraction, p.MinTrail, p.MaxTrail, p.MaxSpread, p.MaxDeviation} {
		if !f.IsPositive() || !f.LessThan(decimal.NewFromInt(1)) {
			return errors.New("entry policy fractions out of bounds")
		}
	}
	if p.MaxPositions < 1 || p.QuoteTTL <= 0 || p.ATRTTL <= 0 || !p.ATRMultiple.IsPositive() || !p.MinQuoteVolume.IsPositive() || p.MinTrail.GreaterThan(p.MaxTrail) || p.MemePositionFraction.GreaterThan(p.PositionFraction) || p.PositionFraction.GreaterThan(p.ExposureFraction) || p.MemeExposureFraction.GreaterThan(p.ExposureFraction) {
		return errors.New("inconsistent entry policy")
	}
	for _, f := range []decimal.Decimal{p.FeePerSide, p.SlippageFraction} {
		if f.IsNegative() || f.GreaterThan(decimal.RequireFromString("0.01")) {
			return errors.New("invalid trading cost assumption")
		}
	}
	return nil
}

// Entry returns an intent only; the existing engine/ledger remains responsible
// for atomic reservation, ownership and the final account-level risk checks.
func (p EntryPolicy) Entry(policy Policy, review Review, evidence []Evidence, event VerifiedEvent, quote Quote, budget Budget, now time.Time) (strategy.Intent, error) {
	deny := func(reason string) (strategy.Intent, error) { return strategy.Intent{}, errors.New(reason) }
	if err := p.Validate(); err != nil {
		return strategy.Intent{}, err
	}
	a := review.Analysis
	if !identifier.MatchString(review.ID) || !review.Valid || review.Error != "" || review.PolicyHash != Hash(policy) || review.At.IsZero() || review.At.After(now) || !now.Before(review.ExpiresAt) || review.ExpiresAt.After(review.At.Add(time.Duration(policy.DecisionTTL)*time.Second)) || a.Action != "PROPOSE_ENTRY" {
		return deny("review invalid, expired or not an entry proposal")
	}
	if err := policy.ValidateAnalysis(a, evidence, now); err != nil {
		return strategy.Intent{}, err
	}
	if !event.Verified || event.Revoked || event.Key != a.EventKey || event.Symbol != a.Symbol || event.Revision != Hash(a) || !now.Before(event.ValidUntil) {
		return deny("event lacks a current trusted attestation")
	}
	if event.Meme != slices.Contains(policy.MemeSymbols, a.Symbol) {
		return deny("event meme classification differs from policy")
	}
	i := quote.Instrument
	if quote.Symbol != a.Symbol || i.Exchange != "binance" || i.Market != domain.Spot || i.Symbol != a.Symbol || i.Quote != "USDT" || !i.Tradable || !i.TickSize.IsPositive() || !i.StepSize.IsPositive() {
		return deny("unsupported or invalid instrument")
	}
	if quote.At.IsZero() || quote.At.After(now) || now.Sub(quote.At) > p.QuoteTTL || quote.ATRAt.IsZero() || quote.ATRAt.After(now) || now.Sub(quote.ATRAt) > p.ATRTTL {
		return deny("market snapshot stale or from future")
	}
	if !quote.Bid.IsPositive() || !quote.Ask.IsPositive() || quote.Bid.GreaterThan(quote.Ask) || !quote.ATR.IsPositive() || !quote.EntryAnchor.IsPositive() || quote.QuoteVolume.LessThan(p.MinQuoteVolume) {
		return deny("invalid or illiquid market")
	}
	spread := quote.Ask.Sub(quote.Bid).Div(quote.Bid.Add(quote.Ask).Div(decimal.NewFromInt(2)))
	if spread.GreaterThan(p.MaxSpread) || quote.Ask.Sub(quote.EntryAnchor).Abs().Div(quote.EntryAnchor).GreaterThan(p.MaxDeviation) {
		return deny("spread or entry deviation too large")
	}
	if budget.Halted || budget.Attention || !budget.Equity.IsPositive() || !budget.FreeCash.IsPositive() || budget.Positions < 0 || budget.Positions >= p.MaxPositions || budget.Exposure.IsNegative() || budget.MemeExposure.IsNegative() {
		return deny("capital or account risk gate rejected")
	}
	distance := decimal.Max(p.MinTrail, quote.ATR.Mul(p.ATRMultiple).Div(quote.Ask))
	if distance.GreaterThan(p.MaxTrail) {
		return deny("required stop distance exceeds policy")
	}
	stop := i.RoundPrice(quote.Ask.Mul(decimal.NewFromInt(1).Sub(distance)))
	if !stop.IsPositive() || !stop.LessThan(quote.Bid) {
		return deny("rounded stop is invalid")
	}
	if quote.Ask.Sub(stop).Div(quote.Ask).GreaterThan(p.MaxTrail) {
		return deny("rounded stop distance exceeds policy")
	}
	cost := p.FeePerSide.Add(p.SlippageFraction).Mul(decimal.NewFromInt(2))
	riskPerUnit := quote.Ask.Sub(stop).Add(quote.Ask.Mul(cost))
	quantity := budget.Equity.Mul(p.RiskFraction).Div(riskPerUnit)
	cap := decimal.Min(budget.Equity.Mul(p.PositionFraction), budget.Equity.Mul(p.ExposureFraction).Sub(budget.Exposure))
	if event.Meme {
		cap = decimal.Min(cap, decimal.Min(budget.Equity.Mul(p.MemePositionFraction), budget.Equity.Mul(p.MemeExposureFraction).Sub(budget.MemeExposure)))
	}
	cap = decimal.Min(cap, budget.FreeCash).Div(decimal.NewFromInt(1).Add(p.FeePerSide).Add(p.SlippageFraction))
	quantity = i.RoundQty(decimal.Min(quantity, cap.Div(quote.Ask)))
	if !quantity.IsPositive() || quote.BidQty.LessThan(quantity.Mul(decimal.NewFromInt(10))) || quote.AskQty.LessThan(quantity.Mul(decimal.NewFromInt(10))) {
		return deny("insufficient balance or two-sided top-of-book depth")
	}
	if err := i.CheckOrder(quote.Ask, quantity); err != nil {
		return strategy.Intent{}, err
	}
	trail, _ := distance.Float64()
	return strategy.Intent{Kind: strategy.Enter, Exchange: "binance", Market: domain.Spot, Symbol: a.Symbol, Side: domain.Long, Quantity: quantity, StopLoss: stop, TrailingStop: trail, Reason: "catalyst " + review.ID}, nil
}
