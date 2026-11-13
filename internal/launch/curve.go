package launch

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
)

// four.meme addresses, the TokenCreate topic and the getTokenInfo selector come
// from the protocol's published integration guide and lite ABIs
// (github.com/four-meme-community/fourmeme-docs), checked on BSC 2026-11-12.
const fourMemeManager = "0x5c952063c7fc8610ffdb798152d69f0b9550762b"
const fourMemeHelper = "0xf251f83e40a78868fcfa3fa4599dad6494e46034"
const tokenCreateTopic = "0x396d5e902b675b032348d3d2e9517ee8f0c4a926603fbc075d3d282ff00cad20"
const getTokenInfo = "0x1f69565f"

// tryBuy(address,uint256,uint256) and trySell(address,uint256) on the helper.
const tryBuy = "0xe21b103a"
const trySell = "0xc6f43e8c"

// curveProbeWei (0.01 BNB) sizes the round-trip estimate; it is not a position size.
const curveProbeWei = 10_000_000_000_000_000

// bsc-dataseed refuses eth_getLogs, so a second public node supplies the logs.
// It is a lead only: every token is confirmed through the helper on rpcURL.
const logsURL = "https://bsc-rpc.publicnode.com"
const curveLogBlocks = 2000

type curveLog struct {
	Address     string   `json:"address"`
	Topics      []string `json:"topics"`
	Data        string   `json:"data"`
	BlockNumber string   `json:"blockNumber"`
	Removed     bool     `json:"removed"`
}

// words splits ABI data into 32-byte words; nil when it is shorter than n words.
func words(data string, n int) []string {
	data = strings.TrimPrefix(data, "0x")
	if len(data) < n*64 {
		return nil
	}
	out := make([]string, n)
	for i := range out {
		out[i] = "0x" + data[i*64:(i+1)*64]
	}
	return out
}

func (c *Client) discoverCurve(ctx context.Context, limit int) Batch {
	b := Batch{Version: Version, Kind: "curve", At: time.Now().UTC()}
	block, proof, err := c.finalized(ctx)
	b.EvidenceIDs = proof
	if err != nil {
		b.Gaps = append(b.Gaps, err.Error())
		return b
	}
	head, _ := strconv.ParseUint(strings.TrimPrefix(block.Number, "0x"), 16, 64)
	var logs []curveLog
	id, err := c.rpcAt(ctx, logsURL, "eth_getLogs", []any{map[string]any{
		"address": fourMemeManager, "topics": []string{tokenCreateTopic},
		"fromBlock": fmt.Sprintf("0x%x", head-min(head, curveLogBlocks)), "toBlock": block.Number}}, &logs)
	proof = append(proof, id)
	b.EvidenceIDs = proof
	if err != nil {
		b.Gaps = append(b.Gaps, "curve_logs_unavailable")
		return b
	}
	b.Gaps = append(b.Gaps, "bounded_curve_tail_not_complete")
	// Logs arrive oldest first; the newest creations are inspected.
	logs = logs[len(logs)-min(len(logs), limit):]
	for _, entry := range logs {
		created := words(entry.Data, 8)
		if entry.Removed || !strings.EqualFold(entry.Address, fourMemeManager) || len(entry.Topics) != 1 || entry.Topics[0] != tokenCreateTopic || created == nil {
			b.Gaps = append(b.Gaps, "curve_log_malformed")
			continue
		}
		token, tokenErr := abiAddress(created[1])
		creator, creatorErr := abiAddress(created[0])
		if tokenErr != nil || creatorErr != nil {
			b.Gaps = append(b.Gaps, "curve_log_malformed")
			continue
		}
		fresh, refreshProof, refreshErr := c.refresh(ctx, block)
		proof = append(proof, refreshProof...)
		if refreshErr != nil {
			b.Gaps = append(b.Gaps, refreshErr.Error())
			break
		}
		block = fresh
		candidate := Candidate{Kind: "curve", Venue: "four-meme", ExternalID: token, URL: "https://four.meme/token/" + token,
			Asset: Asset{Chain: "eip155:56", Address: token}, Quote: "BNB", State: "bonding_curve", ObservedAt: time.Now().UTC(),
			Facts: map[string]string{"creator": creator, "created_block": entry.BlockNumber}, EvidenceIDs: append([]string{}, proof...)}
		value, callID, callErr := c.call(ctx, fourMemeHelper, getTokenInfo+created[1][2:], block.Number)
		candidate.EvidenceIDs = append(candidate.EvidenceIDs, callID)
		info := words(value, 12)
		if callErr != nil || info == nil {
			candidate.Gaps = append(candidate.Gaps, "curve_token_not_confirmed_by_helper")
		} else {
			num := func(i int) *big.Int {
				if n, e := abiUint(info[i]); e == nil {
					return n
				}
				return new(big.Int)
			}
			candidate.Facts["version"] = num(0).String()
			if manager, e := abiAddress(info[1]); e == nil {
				candidate.Facts["token_manager"] = manager
			}
			if quote, e := abiAddress(info[2]); e == nil {
				candidate.Quote = quote
			}
			candidate.Facts["offers"], candidate.Facts["max_offers"] = num(7).String(), num(8).String()
			candidate.Facts["funds_wei"], candidate.Facts["max_funds_wei"] = num(9).String(), num(10).String()
			if num(11).Sign() != 0 {
				candidate.State = "graduated"
			}
			if launch := num(6); launch.IsInt64() && launch.Sign() > 0 {
				candidate.StartsAt, candidate.TimeBasis = time.Unix(launch.Int64(), 0).UTC(), "onchain_launch_time"
			}
			if candidate.State == "bonding_curve" && candidate.Quote == "BNB" {
				c.curveQuote(ctx, &candidate, created[1][2:], block.Number)
			} else if candidate.State == "bonding_curve" {
				candidate.Gaps = append(candidate.Gaps, "curve_quote_asset_not_native")
			}
		}
		if candidate.StartsAt.IsZero() {
			// No scheduled launch: trading opened with the creation block.
			var header chainSnapshot
			headerID, headerErr := c.rpc(ctx, "eth_getBlockByNumber", []any{entry.BlockNumber, false}, &header)
			candidate.EvidenceIDs = append(candidate.EvidenceIDs, headerID)
			if ts, e := strconv.ParseInt(strings.TrimPrefix(header.Timestamp, "0x"), 16, 64); headerErr == nil && e == nil && ts > 0 {
				candidate.StartsAt, candidate.TimeBasis = time.Unix(ts, 0).UTC(), "onchain_creation_block"
			}
		}
		c.inspectToken(ctx, &candidate, block)
		candidate.FirstSeenAt = candidate.ObservedAt
		candidate.Seal()
		b.Candidates = append(b.Candidates, candidate)
	}
	b.EvidenceIDs = proof
	return b
}

// curveQuote records the helper's estimate for buying with the probe and selling the
// result straight back. It is curve arithmetic without gas, not proof a holder can sell.
func (c *Client) curveQuote(ctx context.Context, candidate *Candidate, tokenWord, block string) {
	value, id, err := c.call(ctx, fourMemeHelper, tryBuy+tokenWord+fmt.Sprintf("%064x%064x", 0, curveProbeWei), block)
	candidate.EvidenceIDs = append(candidate.EvidenceIDs, id)
	buy := words(value, 8)
	if err != nil || buy == nil {
		candidate.Gaps = append(candidate.Gaps, "curve_quote_unavailable")
		return
	}
	manager, managerErr := abiAddress(buy[0])
	amount, amountErr := abiUint(buy[2])
	cost, costErr := abiUint(buy[3])
	fee, feeErr := abiUint(buy[4])
	if managerErr != nil || amountErr != nil || costErr != nil || feeErr != nil || manager != fourMemeManager || amount.Sign() <= 0 || cost.Sign() <= 0 {
		candidate.Gaps = append(candidate.Gaps, "curve_quote_unavailable")
		return
	}
	value, id, err = c.call(ctx, fourMemeHelper, trySell+tokenWord+buy[2][2:], block)
	candidate.EvidenceIDs = append(candidate.EvidenceIDs, id)
	sell := words(value, 4)
	if err != nil || sell == nil {
		candidate.Gaps = append(candidate.Gaps, "curve_sell_quote_unavailable")
		return
	}
	funds, fundsErr := abiUint(sell[2])
	paid := new(big.Int).Add(cost, fee)
	if fundsErr != nil || funds.Sign() <= 0 || funds.Cmp(paid) > 0 {
		candidate.Gaps = append(candidate.Gaps, "curve_sell_quote_unavailable")
		return
	}
	candidate.Facts["curve_probe_wei"] = strconv.FormatInt(curveProbeWei, 10)
	candidate.Facts["curve_buy_tokens"], candidate.Facts["curve_buy_paid_wei"] = amount.String(), paid.String()
	candidate.Facts["curve_sell_funds_wei"] = funds.String()
	candidate.Facts["curve_round_trip_cost_fraction"] = new(big.Rat).SetFrac(new(big.Int).Sub(paid, funds), paid).FloatString(6)
}
