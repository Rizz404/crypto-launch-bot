package launch

import (
	"context"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/rizz404/trading-bot/internal/catalyst"
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

func (c *Client) discoverPresales(ctx context.Context, binary string) Batch {
	b := Batch{Version: Version, Kind: "presale", At: time.Now().UTC()}
	source := catalyst.Source{ID: "p4-pinksale-bsc", URL: "https://www.pinksale.finance/launchpads",
		Primary: true, ArticlePattern: `^/launchpad/bsc/0x[0-9a-fA-F]{40}$`}
	e, err := catalyst.CollectDiscoveryWebsite(ctx, c.Archive, binary, source)
	if err != nil {
		b.Gaps = append(b.Gaps, err.Error())
		return b
	}
	id, err := c.Archive.Put("launch-web", e)
	if err != nil {
		b.Gaps = append(b.Gaps, "website_evidence_archive_failed")
		return b
	}
	block, chainProof, chainErr := c.finalized(ctx)
	b.EvidenceIDs = append([]string{id}, chainProof...)
	for _, doc := range e.Documents {
		candidate, ok := parseSale(doc, id, e.FirstSeenAt)
		if !ok {
			continue
		}
		candidate.EvidenceIDs = append(candidate.EvidenceIDs, chainProof...)
		if chainErr != nil {
			candidate.Gaps = append(candidate.Gaps, chainErr.Error())
		} else {
			c.inspectToken(ctx, &candidate, block)
			var code string
			proof, codeErr := c.rpc(ctx, "eth_getCode", []any{candidate.ExternalID, block.Number}, &code)
			candidate.EvidenceIDs = append(candidate.EvidenceIDs, proof)
			if codeErr != nil || len(code) <= 2 {
				candidate.Gaps = append(candidate.Gaps, "sale_contract_code_unavailable")
			} else {
				candidate.Facts["sale_code"] = "present_not_factory_verified"
			}
		}
		candidate.Seal()
		b.Candidates = append(b.Candidates, candidate)
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
	rootID, err := c.fetch(ctx, "https://data-api.binance.vision/api/v3/exchangeInfo?permissions=SPOT&showPermissionSets=false", nil, &info)
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
