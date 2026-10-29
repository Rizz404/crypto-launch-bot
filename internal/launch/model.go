// Package launch discovers and verifies launch candidates. It has no signer,
// real-money ledger, Telegram client or live order submission path. AI analysis
// is explicit research; a separate journal supports synthetic paper replay.
package launch

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/rizz404/trading-bot/internal/catalyst"
	"github.com/shopspring/decimal"
)

const Version = "launch-discovery-v1"
const MaxAge = 30 * 24 * time.Hour
const EvidenceTTL = 15 * time.Minute

var addressPattern = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)
var symbolPattern = regexp.MustCompile(`^[A-Z0-9]{2,32}$`)

func address(s string) (string, error) {
	if !addressPattern.MatchString(s) || strings.EqualFold(s, "0x"+strings.Repeat("0", 40)) {
		return "", errors.New("invalid or zero EVM address")
	}
	return strings.ToLower(s), nil
}

type Asset struct {
	Chain   string `json:"chain,omitempty"`
	Address string `json:"address,omitempty"`
	VenueID string `json:"venue_asset_id,omitempty"`
	Symbol  string `json:"symbol,omitempty"`
}

func (a Asset) Key() string {
	if a.Chain == "eip155:56" {
		if addr, err := address(a.Address); err == nil && a.VenueID == "" {
			return a.Chain + ":" + addr
		}
	}
	if a.Chain == "" && a.Address == "" && strings.HasPrefix(a.VenueID, "binance:") && symbolPattern.MatchString(strings.TrimPrefix(a.VenueID, "binance:")) {
		return "cex:" + a.VenueID
	}
	return ""
}

type Candidate struct {
	ID          string            `json:"id"`
	Revision    string            `json:"revision"`
	Kind        string            `json:"kind"`
	Venue       string            `json:"venue"`
	ExternalID  string            `json:"external_id"`
	Asset       Asset             `json:"asset"`
	Quote       string            `json:"quote,omitempty"`
	URL         string            `json:"url"`
	StartsAt    time.Time         `json:"starts_at"`
	EndsAt      time.Time         `json:"ends_at"`
	TimeBasis   string            `json:"time_basis"`
	State       string            `json:"state"`
	ObservedAt  time.Time         `json:"observed_at"`
	FirstSeenAt time.Time         `json:"first_seen_at"`
	EvidenceIDs []string          `json:"evidence_ids"`
	Facts       map[string]string `json:"facts"`
	Gaps        []string          `json:"gaps"`
}

// Seal separates stable sale/listing identity from mutable information.
func (c *Candidate) Seal() {
	c.ID = catalyst.Hash([]string{c.Kind, c.Venue, c.ExternalID, c.Asset.Chain})
	c.Revision = catalyst.Hash([]any{Version, c.Kind, c.Venue, c.ExternalID,
		c.Asset, c.Quote, c.StartsAt, c.EndsAt, c.TimeBasis, c.State, c.Facts, c.Gaps})
	slices.Sort(c.EvidenceIDs)
	c.EvidenceIDs = slices.Compact(c.EvidenceIDs)
	c.EvidenceIDs = slices.DeleteFunc(c.EvidenceIDs, func(id string) bool { return id == "" })
}

type Batch struct {
	Version       string      `json:"version"`
	Kind          string      `json:"kind"`
	At            time.Time   `json:"at"`
	Candidates    []Candidate `json:"candidates"`
	Gaps          []string    `json:"gaps"`
	Scanned       []string    `json:"scanned,omitempty"`
	EvidenceIDs   []string    `json:"evidence_ids,omitempty"`
	NextPoolIndex *uint64     `json:"next_pool_index,omitempty"`
}

type Review struct {
	Candidate    Candidate `json:"candidate"`
	Status       string    `json:"status"`
	Reasons      []string  `json:"reasons"`
	EntryAllowed bool      `json:"entry_allowed"`
}

type Report struct {
	Version      string           `json:"version"`
	At           time.Time        `json:"at"`
	ResearchOnly bool             `json:"research_only"`
	Sources      map[string]Batch `json:"sources"`
	Reviews      []Review         `json:"reviews"`
	Analyses     []AIReview       `json:"analyses,omitempty"`
}

// Verify never authorizes entry. Missing facts are not interpreted as safe.
func Verify(c Candidate, now time.Time, sourceGaps []string) Review {
	r := Review{Candidate: c, Status: "accepted_for_research"}
	hold := func(reason string) {
		r.Reasons = append(r.Reasons, reason)
		if r.Status != "rejected" {
			r.Status = "held"
		}
	}
	reject := func(reason string) { r.Reasons = append(r.Reasons, reason); r.Status = "rejected" }
	if c.Asset.Key() == "" {
		hold("asset_identity_unknown")
	}
	if c.ObservedAt.IsZero() || c.ObservedAt.After(now) || now.Sub(c.ObservedAt) > EvidenceTTL {
		hold("evidence_stale_or_future")
	}
	if len(c.EvidenceIDs) == 0 {
		hold("evidence_missing")
	}
	for _, gap := range append(slices.Clone(sourceGaps), c.Gaps...) {
		hold(gap)
	}
	if c.StartsAt.IsZero() {
		hold("schedule_unknown")
	} else if c.Kind != "presale" {
		if c.StartsAt.After(now) {
			hold("listing_not_open")
		} else if now.Sub(c.StartsAt) > MaxAge {
			reject("listing_older_than_30_days")
		}
		if c.TimeBasis != "first_trade_id_zero" {
			hold("trading_start_not_verified")
		}
	}
	if c.Kind == "presale" {
		if c.EndsAt.IsZero() || (!c.StartsAt.IsZero() && !c.EndsAt.After(c.StartsAt)) {
			hold("sale_schedule_invalid")
		}
		if slices.Contains([]string{"canceled", "ended", "filled"}, c.State) || (!c.EndsAt.IsZero() && !now.Before(c.EndsAt)) {
			reject("sale_closed")
		}
		if !slices.Contains([]string{"upcoming", "live", "inprogress", "canceled", "ended", "filled"}, c.State) {
			hold("sale_state_unknown")
		}
		hold("sale_contract_and_terms_not_verified")
	}
	if c.Kind != "presale" && c.Kind != "dex" && c.Kind != "cex" {
		reject("unsupported_kind")
	}
	if c.Kind == "cex" && c.State != "TRADING" {
		hold("market_not_trading")
	}
	if c.Kind == "cex" {
		bid, be := decimal.NewFromString(c.Facts["bid"])
		ask, ae := decimal.NewFromString(c.Facts["ask"])
		bq, bqe := decimal.NewFromString(c.Facts["bid_qty"])
		aq, aqe := decimal.NewFromString(c.Facts["ask_qty"])
		if be != nil || ae != nil || bqe != nil || aqe != nil || !bid.IsPositive() || !ask.IsPositive() || !bq.IsPositive() || !aq.IsPositive() || bid.GreaterThan(ask) {
			hold("book_invalid_or_empty")
		}
	}
	if c.Kind == "dex" || c.Kind == "presale" {
		for _, key := range []string{"chain_verified", "contract_code", "finalized_block"} {
			if c.Facts[key] == "" || c.Facts[key] == "unknown" {
				hold(key + "_unknown")
			}
		}
		for _, key := range []string{"is_honeypot", "cannot_sell_all", "is_blacklisted"} {
			if c.Facts[key] == "1" {
				reject(key)
			} else if c.Facts[key] != "0" {
				hold(key + "_unknown")
			}
		}
		for _, key := range []string{"is_proxy", "is_mintable", "owner_change_balance", "slippage_modifiable", "transfer_pausable"} {
			if c.Facts[key] != "0" {
				hold(key + "_not_cleared")
			}
		}
		if c.Facts["is_open_source"] != "1" {
			hold("contract_source_not_verified")
		}
		hold("holder_liquidity_and_exit_policy_pending")
	}
	r.Reasons = append(r.Reasons, "entry_disabled_until_p4_3_and_p4_4")
	slices.Sort(r.Reasons)
	r.Reasons = slices.Compact(r.Reasons)
	return r
}
