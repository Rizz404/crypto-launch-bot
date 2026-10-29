package launch

import (
	"errors"
	"time"

	"github.com/rizz404/trading-bot/internal/catalyst"
	"github.com/shopspring/decimal"
)

const PaperPolicyVersion = "launch-paper-v1"
const EvaluationVersion = "launch-evaluation-v1"

// PaperPolicy is approved configuration, never model output or live authority.
type PaperPolicy struct {
	Version               string          `json:"version"`
	EvaluationVersion     string          `json:"evaluation_version"`
	InitialUSDT           decimal.Decimal `json:"initial_usdt"`
	MaxExposures          int             `json:"max_exposures"`
	PresaleFraction       decimal.Decimal `json:"presale_fraction"`
	DEXFraction           decimal.Decimal `json:"dex_fraction"`
	CEXFraction           decimal.Decimal `json:"cex_fraction"`
	CEXRiskFraction       decimal.Decimal `json:"cex_risk_fraction"`
	DailyHaltFraction     decimal.Decimal `json:"daily_halt_fraction"`
	DrawdownHaltFraction  decimal.Decimal `json:"drawdown_halt_fraction"`
	CallsPerWIBDay        int             `json:"calls_per_wib_day"`
	MaxOutputTokens       int             `json:"max_output_tokens"`
	DecisionTTLSeconds    int             `json:"decision_ttl_seconds"`
	ReviewIntervalSeconds int             `json:"review_interval_seconds"`
}

func ApprovedPaperPolicy() PaperPolicy {
	return PaperPolicy{Version: PaperPolicyVersion, EvaluationVersion: EvaluationVersion,
		InitialUSDT: decimal.NewFromInt(1000), MaxExposures: 2,
		PresaleFraction: decimal.RequireFromString("0.01"), DEXFraction: decimal.RequireFromString("0.01"),
		CEXFraction: decimal.RequireFromString("0.10"), CEXRiskFraction: decimal.RequireFromString("0.005"),
		DailyHaltFraction: decimal.RequireFromString("0.02"), DrawdownHaltFraction: decimal.RequireFromString("0.08"),
		CallsPerWIBDay: 12, MaxOutputTokens: 2048, DecisionTTLSeconds: 300, ReviewIntervalSeconds: 300}
}

func (p PaperPolicy) Validate() error {
	if catalyst.Hash(p) != catalyst.Hash(ApprovedPaperPolicy()) {
		return errors.New("policy differs from approved launch-paper-v1; approve and version changes separately")
	}
	return nil
}

// PaperExposure counts locked, reserved and tradable capital once. LockedUSDT
// and ReservedUSDT are subsets of NotionalUSDT, not additional positions.
type PaperExposure struct {
	ID           string           `json:"id"`
	CandidateID  string           `json:"candidate_id"`
	AssetKey     string           `json:"asset_key"`
	Kind         string           `json:"kind"`
	NotionalUSDT decimal.Decimal  `json:"notional_usdt"`
	MarkUSDT     *decimal.Decimal `json:"mark_usdt,omitempty"`
	MarkKnown    bool             `json:"mark_known"`
	LockedUSDT   decimal.Decimal  `json:"locked_usdt"`
	ReservedUSDT decimal.Decimal  `json:"reserved_usdt"`
	Tradable     bool             `json:"tradable"`
	Claimable    bool             `json:"claimable"`
	Refundable   bool             `json:"refundable"`
	Thesis       string           `json:"thesis"`
	Invalidation string           `json:"invalidation"`
}

// PaperContext is explicit research input, not a verified execution ledger.
// P4-4 must source it from its own durable ledger before permitting any action.
type PaperContext struct {
	At                 time.Time       `json:"at"`
	DayWIB             string          `json:"day_wib"`
	EquityUSDT         decimal.Decimal `json:"equity_usdt"`
	DayStartUSDT       decimal.Decimal `json:"day_start_usdt"`
	PeakUSDT           decimal.Decimal `json:"peak_usdt"`
	AvailableUSDT      decimal.Decimal `json:"available_usdt"`
	Positions          []PaperExposure `json:"positions"`
	ConsumedCandidates []string        `json:"consumed_candidates"`
}

func InitialPaperContext(now time.Time, p PaperPolicy) PaperContext {
	return PaperContext{At: now.UTC(), DayWIB: now.In(time.FixedZone("WIB", 7*60*60)).Format("2006-01-02"), EquityUSDT: p.InitialUSDT, DayStartUSDT: p.InitialUSDT, PeakUSDT: p.InitialUSDT, AvailableUSDT: p.InitialUSDT}
}

func (s PaperContext) Validate(now time.Time) error {
	if s.DayWIB != now.In(time.FixedZone("WIB", 7*60*60)).Format("2006-01-02") {
		return errors.New("paper day-start belongs to a different WIB day")
	}
	if s.At.IsZero() || s.At.After(now) || now.Sub(s.At) > EvidenceTTL || !s.EquityUSDT.IsPositive() || !s.DayStartUSDT.IsPositive() || s.PeakUSDT.LessThan(s.EquityUSDT) || s.AvailableUSDT.IsNegative() || s.AvailableUSDT.GreaterThan(s.EquityUSDT) {
		return errors.New("invalid or stale paper context")
	}
	seen := map[string]bool{}
	total := s.AvailableUSDT
	for _, position := range s.Positions {
		if position.ID == "" || position.CandidateID == "" || position.AssetKey == "" || seen[position.ID] || !position.NotionalUSDT.IsPositive() || position.LockedUSDT.IsNegative() || position.ReservedUSDT.IsNegative() || position.LockedUSDT.Add(position.ReservedUSDT).GreaterThan(position.NotionalUSDT) {
			return errors.New("invalid or duplicate paper exposure")
		}
		if position.Kind != "presale" && position.Kind != "dex" && position.Kind != "cex" {
			return errors.New("unsupported exposure kind")
		}
		seen[position.ID] = true
		mark := position.NotionalUSDT
		if position.MarkUSDT != nil {
			if position.MarkUSDT.IsNegative() {
				return errors.New("negative paper mark")
			}
			mark = *position.MarkUSDT
		}
		total = total.Add(mark)
	}
	if total.GreaterThan(s.EquityUSDT) {
		return errors.New("paper capital is counted more than once")
	}
	return nil
}

func (p PaperPolicy) Halted(s PaperContext) bool {
	return s.EquityUSDT.LessThanOrEqual(s.DayStartUSDT.Mul(decimal.NewFromInt(1).Sub(p.DailyHaltFraction))) || s.EquityUSDT.LessThanOrEqual(s.PeakUSDT.Mul(decimal.NewFromInt(1).Sub(p.DrawdownHaltFraction)))
}

type PaperSize struct {
	PolicyHash      string          `json:"policy_hash"`
	NotionalUSDT    decimal.Decimal `json:"notional_usdt"`
	ModeledLossUSDT decimal.Decimal `json:"modeled_loss_usdt"`
	EntryAllowed    bool            `json:"entry_allowed"`
}

// PlanSize calculates a paper ceiling including a caller-verified round-trip
// cost fraction. Unknown fees/stop must not be silently replaced with zero.
// These are scenario amounts, never executable orders or guaranteed loss caps.
func (p PaperPolicy) PlanSize(kind string, s PaperContext, stopFraction, costFraction decimal.Decimal, costKnown bool, now time.Time) (PaperSize, error) {
	result := PaperSize{PolicyHash: catalyst.Hash(p)}
	if err := p.Validate(); err != nil {
		return result, err
	}
	if err := s.Validate(now); err != nil {
		return result, err
	}
	if p.Halted(s) || len(s.Positions) >= p.MaxExposures {
		return result, errors.New("paper halt or exposure limit")
	}
	if !marksKnown(s) {
		return result, errors.New("paper valuation unknown; no new exposure")
	}
	if !costKnown || costFraction.IsNegative() || costFraction.GreaterThanOrEqual(decimal.NewFromInt(1)) {
		return result, errors.New("round-trip cost unknown or invalid")
	}
	fraction := p.PresaleFraction
	switch kind {
	case "presale":
	case "dex":
		fraction = p.DEXFraction
	case "cex":
		fraction = p.CEXFraction
	default:
		return result, errors.New("unsupported paper kind")
	}
	budget := decimal.Min(s.EquityUSDT, p.InitialUSDT).Mul(fraction)
	lossRate := decimal.NewFromInt(1).Add(costFraction)
	if kind == "cex" {
		if !stopFraction.IsPositive() || stopFraction.GreaterThanOrEqual(decimal.NewFromInt(1)) {
			return result, errors.New("CEX stop fraction required")
		}
		lossRate = stopFraction.Add(costFraction)
		budget = decimal.Min(budget, decimal.Min(s.EquityUSDT, p.InitialUSDT).Mul(p.CEXRiskFraction).Div(lossRate))
	} else {
		// Locked/illiquid capital may be lost in full; reserve fee headroom too.
		budget = budget.Div(lossRate)
	}
	budget = decimal.Min(budget, s.AvailableUSDT.Div(decimal.NewFromInt(1).Add(costFraction))).Truncate(8)
	if !budget.IsPositive() {
		return result, errors.New("no available paper capital")
	}
	result.NotionalUSDT = budget
	result.ModeledLossUSDT = budget.Mul(lossRate)
	return result, nil
}
