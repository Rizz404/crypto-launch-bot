package launch

import (
	"time"

	"github.com/shopspring/decimal"
)

const PaperFixtureVersion = "launch-paper-fixture-v1"
const PaperJournalVersion = "launch-paper-journal-v1"

// All execution inputs are synthetic scenarios, never venue acknowledgements.
// This package does not construct, sign or submit exchange/on-chain requests.
type PaperFixture struct {
	Version     string      `json:"version"`
	RunID       string      `json:"run_id"`
	FixtureOnly bool        `json:"fixture_only"`
	Policy      PaperPolicy `json:"policy"`
	Steps       []PaperStep `json:"steps"`
}

type PaperStep struct {
	ID         string           `json:"id"`
	At         time.Time        `json:"at"`
	Op         string           `json:"op"`
	OrderID    string           `json:"order_id,omitempty"`
	ExternalID string           `json:"external_id,omitempty"`
	Intent     *PaperIntent     `json:"intent,omitempty"`
	Book       *PaperBookQuote  `json:"book,omitempty"`
	Route      *PaperRouteQuote `json:"route,omitempty"`
	Receipt    *PaperReceipt    `json:"receipt,omitempty"`
	Allocation *PaperAllocation `json:"allocation,omitempty"`
	Mark       *PaperMark       `json:"mark,omitempty"`
}

type PaperIntent struct {
	ID           string          `json:"id"`
	Candidate    Candidate       `json:"candidate"`
	Action       string          `json:"action"` // buy, contribute, sell, claim, refund
	PositionID   string          `json:"position_id"`
	NotionalUSDT decimal.Decimal `json:"notional_usdt"`
	Quantity     decimal.Decimal `json:"quantity"`
	LimitPrice   decimal.Decimal `json:"limit_price"`
	MaxFeeUSDT   decimal.Decimal `json:"max_fee_usdt"`
	CostFraction decimal.Decimal `json:"cost_fraction"`
	StopFraction decimal.Decimal `json:"stop_fraction"`
	CostsKnown   bool            `json:"costs_known"`
	ExpiresAt    time.Time       `json:"expires_at"`
	Thesis       string          `json:"thesis"`
	Invalidation string          `json:"invalidation"`
	// This is an explicit fixture assumption, not a way to promote P4-2 gaps.
	AssumedVerified bool `json:"assumed_verified"`
}

type PaperBookQuote struct {
	At          time.Time       `json:"at"`
	CandidateID string          `json:"candidate_id"`
	Bid         decimal.Decimal `json:"bid"`
	Ask         decimal.Decimal `json:"ask"`
	BidQty      decimal.Decimal `json:"bid_qty"`
	AskQty      decimal.Decimal `json:"ask_qty"`
	LotStep     decimal.Decimal `json:"lot_step"`
	MinNotional decimal.Decimal `json:"min_notional"`
	FeeFraction decimal.Decimal `json:"fee_fraction"`
	CostsKnown  bool            `json:"costs_known"`
}

type PaperRouteQuote struct {
	ID                string          `json:"id"`
	At                time.Time       `json:"at"`
	ExpiresAt         time.Time       `json:"expires_at"`
	CandidateID       string          `json:"candidate_id"`
	AssetKey          string          `json:"asset_key"`
	InputUSDT         decimal.Decimal `json:"input_usdt"`
	Quantity          decimal.Decimal `json:"quantity"`
	OutputUSDT        decimal.Decimal `json:"output_usdt"`
	MinQuantity       decimal.Decimal `json:"min_quantity"`
	MinOutputUSDT     decimal.Decimal `json:"min_output_usdt"`
	FeeUSDT           decimal.Decimal `json:"fee_usdt"`
	GasUSDT           decimal.Decimal `json:"gas_usdt"`
	CostsKnown        bool            `json:"costs_known"`
	LiquidityVerified bool            `json:"liquidity_verified"`
	SellableVerified  bool            `json:"sellable_verified"`
}

// Receipt amounts are cumulative for one order; deltas settle exactly once.
type PaperReceipt struct {
	ID           string          `json:"id"`
	ExternalID   string          `json:"external_id"`
	Status       string          `json:"status"`
	Quantity     decimal.Decimal `json:"quantity"`
	SpentUSDT    decimal.Decimal `json:"spent_usdt"`
	ReceivedUSDT decimal.Decimal `json:"received_usdt"`
	FeeUSDT      decimal.Decimal `json:"fee_usdt"`
	Finalized    bool            `json:"finalized"`
	BlockNumber  uint64          `json:"block_number"`
	BlockHash    string          `json:"block_hash"`
}

type PaperAllocation struct {
	PositionID       string          `json:"position_id"`
	TotalQuantity    decimal.Decimal `json:"total_quantity"`
	UnlockedQuantity decimal.Decimal `json:"unlocked_quantity"`
	RefundableUSDT   decimal.Decimal `json:"refundable_usdt"`
	Finalized        bool            `json:"finalized"`
	Proof            string          `json:"proof"`
}

type PaperMark struct {
	PositionID string          `json:"position_id"`
	At         time.Time       `json:"at"`
	PriceUSDT  decimal.Decimal `json:"price_usdt"`
	Known      bool            `json:"known"`
	Proof      string          `json:"proof"`
}

type PaperOrder struct {
	Intent       PaperIntent      `json:"intent"`
	At           time.Time        `json:"at"`
	Status       string           `json:"status"`
	ExternalID   string           `json:"external_id"`
	ReserveUSDT  decimal.Decimal  `json:"reserve_usdt"`
	Quantity     decimal.Decimal  `json:"quantity"`
	SpentUSDT    decimal.Decimal  `json:"spent_usdt"`
	ReceivedUSDT decimal.Decimal  `json:"received_usdt"`
	FeeUSDT      decimal.Decimal  `json:"fee_usdt"`
	Route        *PaperRouteQuote `json:"route,omitempty"`
}

type PaperPosition struct {
	ID                string          `json:"id"`
	Candidate         Candidate       `json:"candidate"`
	Quantity          decimal.Decimal `json:"quantity"`
	CostUSDT          decimal.Decimal `json:"cost_usdt"`
	LockedUSDT        decimal.Decimal `json:"locked_usdt"`
	ContributedUSDT   decimal.Decimal `json:"contributed_usdt"`
	AllocatedQuantity decimal.Decimal `json:"allocated_quantity"`
	UnlockedQuantity  decimal.Decimal `json:"unlocked_quantity"`
	ClaimedQuantity   decimal.Decimal `json:"claimed_quantity"`
	RefundableUSDT    decimal.Decimal `json:"refundable_usdt"`
	RefundedUSDT      decimal.Decimal `json:"refunded_usdt"`
	MarkPrice         decimal.Decimal `json:"mark_price"`
	MarkAt            time.Time       `json:"mark_at"`
	StopPrice         decimal.Decimal `json:"stop_price"`
	ExitRequired      bool            `json:"exit_required"`
	FeeBudgetUSDT     decimal.Decimal `json:"fee_budget_usdt"`
	Thesis            string          `json:"thesis"`
	Invalidation      string          `json:"invalidation"`
}

type PaperState struct {
	Version           string                     `json:"version"`
	RunID             string                     `json:"run_id"`
	FixtureOnly       bool                       `json:"fixture_only"`
	PolicyHash        string                     `json:"policy_hash"`
	Sequence          uint64                     `json:"sequence"`
	LastHash          string                     `json:"last_hash"`
	At                time.Time                  `json:"at"`
	CashUSDT          decimal.Decimal            `json:"cash_usdt"`
	FeesUSDT          decimal.Decimal            `json:"fees_usdt"`
	RealizedGrossUSDT decimal.Decimal            `json:"realized_gross_usdt"`
	DayWIB            string                     `json:"day_wib"`
	DayStartUSDT      decimal.Decimal            `json:"day_start_usdt"`
	PeakUSDT          decimal.Decimal            `json:"peak_usdt"`
	Attention         bool                       `json:"attention"`
	Orders            map[string]PaperOrder      `json:"orders"`
	Positions         map[string]PaperPosition   `json:"positions"`
	Requests          map[string]string          `json:"requests"`
	Receipts          map[string]string          `json:"receipts"`
	Consumed          map[string]bool            `json:"consumed"`
	QuoteUsage        map[string]decimal.Decimal `json:"quote_usage"`
}
