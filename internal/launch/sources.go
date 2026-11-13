package launch

import (
	"context"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/rizz404/crypto-launch-bot/internal/catalyst"
)

var pinkPath = regexp.MustCompile(`^/launchpad/bsc/(0x[0-9a-fA-F]{40})$`)
var tokenLabel = regexp.MustCompile(`(?i)Token\s+Address\s*[:|]?\s*(0x[0-9a-fA-F]{40})\b`)
var saleDate = regexp.MustCompile(`(?i)(?:Presale\s+)?(Start|End)\s+[Tt]ime\s*[:|]?\s*(\d{4}[.-]\d{2}[.-]\d{2}\s+\d{2}:\d{2}(?::\d{2})?\s*\(?UTC\)?)`)
var saleState = regexp.MustCompile(`(?i)\bStatus\s*[:|]?\s*(Canceled|Cancelled|Ended|Filled|Inprogress|Upcoming|Live)\b`)

func parseSale(doc catalyst.Document, evidence string, now time.Time) (Candidate, bool) {
	u, err := url.Parse(doc.URL)
	if err != nil || u.Scheme != "https" || u.Host != "www.pinksale.finance" || u.RawQuery != "" || u.User != nil {
		return Candidate{}, false
	}
	path := pinkPath.FindStringSubmatch(u.Path)
	if len(path) != 2 {
		return Candidate{}, false
	}
	pool, err := address(path[1])
	if err != nil {
		return Candidate{}, false
	}
	c := Candidate{Kind: "presale", Venue: "pinksale", ExternalID: pool, URL: doc.URL,
		Asset: Asset{Chain: "eip155:56"}, ObservedAt: now, FirstSeenAt: now,
		EvidenceIDs: []string{evidence}, Facts: map[string]string{}, TimeBasis: "publisher_schedule"}
	addresses := tokenLabel.FindAllStringSubmatch(doc.Body, -1)
	for _, m := range addresses {
		a, _ := address(m[1])
		if c.Asset.Address != "" && a != c.Asset.Address {
			c.Gaps = append(c.Gaps, "token_identity_conflict")
		}
		c.Asset.Address = a
	}
	if c.Asset.Address == "" {
		c.Gaps = append(c.Gaps, "publisher_token_address_missing")
	}
	for _, m := range saleDate.FindAllStringSubmatch(doc.Body, -1) {
		text := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(m[2], ".", "-"), "(", ""), ")", "")
		text = strings.Join(strings.Fields(text), " ")
		var parsed time.Time
		for _, layout := range []string{"2006-01-02 15:04 MST", "2006-01-02 15:04:05 MST"} {
			if t, e := time.Parse(layout, text); e == nil {
				parsed = t.UTC()
				break
			}
		}
		target := &c.StartsAt
		if strings.EqualFold(m[1], "end") {
			target = &c.EndsAt
		}
		if !target.IsZero() && !parsed.Equal(*target) {
			c.Gaps = append(c.Gaps, "sale_schedule_conflict")
		}
		*target = parsed
	}
	for _, m := range saleState.FindAllStringSubmatch(doc.Body, -1) {
		state := strings.ToLower(m[1])
		if state == "cancelled" {
			state = "canceled"
		}
		if c.State != "" && c.State != state {
			c.Gaps = append(c.Gaps, "sale_state_conflict")
		}
		c.State = state
	}
	if c.State == "" {
		c.Gaps = append(c.Gaps, "sale_state_unknown")
	}
	c.Seal()
	return c, true
}

// PinkSale publishes no contract addresses. These were read from live BSC pools on
// 2026-11-12: every sampled pool is an EIP-1167 clone of one of three implementations
// and returns the same factory(). A pool that differs is flagged, never trusted.
// The third implementation backs fair launches (two sampled pools, no hard cap or rate).
const pinkFactory = "0x1ee7736987f2ebd0f519ec5858b2b3bc6fd0c6a0"
const factorySelector = "0xc45a0155"

// The sale type follows the implementation: a fair launch has no fixed rate or hard cap.
var pinkImplementations = map[string]string{
	"0xd18b06e912575a388593ee07bf6f902bd6e30109": "presale",
	"0xeed5194c2d5f16cd1d54f37ca84603be7c0bfd0d": "presale",
	"0x07f6351491b694f3a04c5f1e47d151d4ad927d45": "fairlaunch",
}
var minimalProxy = regexp.MustCompile(`^0x363d3d373d3d3d363d73([0-9a-f]{40})5af43d82803e903d91602b57fd5bf3$`)

// presaleLeads only supply pool links; sale identity and terms come from the
// pinksale.finance pages and the chain. PinkSale's own index is rendered by
// JavaScript. The tracking channel is not listed among PinkSale's official links.
var presaleLeads = []struct {
	source   catalyst.Source
	official bool
}{
	{catalyst.Source{ID: "p4-pinksale-telegram-ann", URL: "https://t.me/s/pinkecosystem_ann"}, true},
	{catalyst.Source{ID: "p4-pinksale-telegram-tracking", URL: "https://t.me/s/PinkSaleTracking"}, false},
}

// inspectSale checks the sale contract against the known PinkSale clone shape.
func (c *Client) inspectSale(ctx context.Context, candidate *Candidate, block chainSnapshot) {
	var code string
	proof, err := c.rpc(ctx, "eth_getCode", []any{candidate.ExternalID, block.Number}, &code)
	candidate.EvidenceIDs = append(candidate.EvidenceIDs, proof)
	if err != nil || len(code) <= 2 {
		candidate.Gaps = append(candidate.Gaps, "sale_contract_code_unavailable")
		return
	}
	candidate.Facts["sale_code"] = "present_not_factory_verified"
	clone := minimalProxy.FindStringSubmatch(strings.ToLower(code))
	value, proof, err := c.call(ctx, candidate.ExternalID, factorySelector, block.Number)
	candidate.EvidenceIDs = append(candidate.EvidenceIDs, proof)
	factory, parseErr := abiAddress(value)
	if err != nil || parseErr != nil || factory != pinkFactory || len(clone) != 2 || pinkImplementations["0x"+clone[1]] == "" {
		candidate.Gaps = append(candidate.Gaps, "sale_contract_not_known_pinksale_clone")
		return
	}
	candidate.Facts["sale_code"] = "known_pinksale_clone"
	candidate.Facts["sale_implementation"] = "0x" + clone[1]
	candidate.Facts["sale_type"] = pinkImplementations["0x"+clone[1]]
	candidate.Facts["sale_factory"] = factory
}

func (c *Client) discoverPresales(ctx context.Context, binary string) Batch {
	b := Batch{Version: Version, Kind: "presale", At: time.Now().UTC()}
	source := catalyst.Source{ID: "p4-pinksale-bsc", URL: "https://www.pinksale.finance/launchpads",
		Primary: true, ArticlePattern: `^/launchpad/bsc/0x[0-9a-fA-F]{40}$`}
	block, chainProof, chainErr := c.finalized(ctx)
	b.EvidenceIDs = slices.Clone(chainProof)
	seen := map[string]bool{}
	for _, lead := range presaleLeads {
		leadEvidence, e, err := catalyst.CollectDiscoveryLinked(ctx, c.Archive, binary, lead.source, source)
		if err != nil {
			b.Gaps = append(b.Gaps, lead.source.ID+": "+err.Error())
			continue
		}
		leadID, leadErr := c.Archive.Put("launch-web", leadEvidence)
		id, err := c.Archive.Put("launch-web", e)
		if err != nil || leadErr != nil {
			b.Gaps = append(b.Gaps, "website_evidence_archive_failed")
			continue
		}
		b.EvidenceIDs = append(b.EvidenceIDs, id, leadID)
		for _, doc := range e.Documents {
			candidate, ok := parseSale(doc, id, e.FirstSeenAt)
			if !ok || seen[candidate.ExternalID] {
				continue
			}
			seen[candidate.ExternalID] = true
			candidate.EvidenceIDs = append(candidate.EvidenceIDs, leadID)
			candidate.Facts["lead_source"] = lead.source.ID
			candidate.Facts["lead_official"] = map[bool]string{true: "1", false: "0"}[lead.official]
			if chainErr == nil {
				var refreshProof []string
				block, refreshProof, chainErr = c.refresh(ctx, block)
				chainProof = append(chainProof, refreshProof...)
			}
			candidate.EvidenceIDs = append(candidate.EvidenceIDs, chainProof...)
			if chainErr != nil {
				candidate.Gaps = append(candidate.Gaps, chainErr.Error())
			} else {
				c.inspectToken(ctx, &candidate, block)
				c.inspectSale(ctx, &candidate, block)
			}
			candidate.Seal()
			b.Candidates = append(b.Candidates, candidate)
		}
	}
	b.Gaps = append(b.Gaps, "bounded_website_coverage_not_complete")
	if len(b.Candidates) == 0 {
		b.Gaps = append(b.Gaps, "no_parseable_bsc_sale_pages_or_javascript_required")
	}
	return b
}

type binanceSymbol struct{ Symbol, Status, BaseAsset, QuoteAsset string }

func (c *Client) discoverCEX(ctx context.Context, limit int, scanned map[string]time.Time) Batch {
	b := Batch{Version: Version, Kind: "cex", At: time.Now().UTC()}
	var info struct {
		Symbols []binanceSymbol `json:"symbols"`
	}
	// The unfiltered SPOT list (mostly BREAK symbols) exceeds maxBody; only TRADING pairs can be entered.
	rootID, err := c.fetch(ctx, "https://data-api.binance.vision/api/v3/exchangeInfo?permissions=SPOT&showPermissionSets=false&symbolStatus=TRADING", nil, &info)
	if rootID != "" {
		b.EvidenceIDs = []string{rootID}
	}
	if err != nil || len(info.Symbols) == 0 {
		b.Gaps = append(b.Gaps, "exchange_info_unavailable")
		return b
	}
	symbols := slices.DeleteFunc(info.Symbols, func(s binanceSymbol) bool {
		return s.QuoteAsset != "USDT" || !symbolPattern.MatchString(s.Symbol) || !symbolPattern.MatchString(s.BaseAsset)
	})
	// New symbols are inspected first, then least recently inspected symbols.
	slices.SortFunc(symbols, func(a, b binanceSymbol) int {
		if n := scanned[a.Symbol].Compare(scanned[b.Symbol]); n != 0 {
			return n
		}
		return strings.Compare(a.Symbol, b.Symbol)
	})
	if len(symbols) > limit {
		symbols = symbols[:limit]
		b.Gaps = append(b.Gaps, "bounded_cex_scan_not_complete")
	}
	for _, s := range symbols {
		candidate := Candidate{Kind: "cex", Venue: "binance-spot", ExternalID: s.Symbol,
			Asset: Asset{VenueID: "binance:" + s.BaseAsset, Symbol: s.BaseAsset}, Quote: s.QuoteAsset,
			URL:   "https://www.binance.com/en/trade/" + s.BaseAsset + "_" + s.QuoteAsset,
			State: s.Status, ObservedAt: time.Now().UTC(), Facts: map[string]string{}, EvidenceIDs: []string{rootID}}
		var trades []struct {
			ID *int64 `json:"a"`
			At int64  `json:"T"`
		}
		id, e := c.fetch(ctx, "https://data-api.binance.vision/api/v3/aggTrades?symbol="+s.Symbol+"&fromId=0&limit=1", nil, &trades)
		candidate.EvidenceIDs = append(candidate.EvidenceIDs, id)
		if e != nil || len(trades) != 1 || trades[0].ID == nil || *trades[0].ID != 0 || trades[0].At <= 0 {
			candidate.Gaps = append(candidate.Gaps, "first_trade_history_unavailable")
			if e == nil && len(trades) == 1 && trades[0].At > 0 {
				// Listing is no later than the oldest retained trade.
				candidate.Facts["oldest_retained_trade_at"] = time.UnixMilli(trades[0].At).UTC().Format(time.RFC3339)
			}
		} else {
			candidate.StartsAt = time.UnixMilli(trades[0].At).UTC()
			candidate.TimeBasis = "first_trade_id_zero"
		}
		var book struct{ Symbol, BidPrice, BidQty, AskPrice, AskQty string }
		id, e = c.fetch(ctx, "https://data-api.binance.vision/api/v3/ticker/bookTicker?symbol="+s.Symbol, nil, &book)
		candidate.EvidenceIDs = append(candidate.EvidenceIDs, id)
		if e != nil || book.Symbol != s.Symbol {
			candidate.Gaps = append(candidate.Gaps, "book_unavailable")
		} else {
			candidate.Facts["bid"] = book.BidPrice
			candidate.Facts["ask"] = book.AskPrice
			candidate.Facts["bid_qty"] = book.BidQty
			candidate.Facts["ask_qty"] = book.AskQty
		}
		candidate.FirstSeenAt = candidate.ObservedAt
		candidate.Seal()
		b.Candidates = append(b.Candidates, candidate)
		b.Scanned = append(b.Scanned, s.Symbol)
	}
	return b
}
