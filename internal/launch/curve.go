package launch

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
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

// _tokenInfos(address) on TokenManager2; creator type is (template >> 10) & 0x3F.
// Tax token getters: feeRate() in basis points (type 5), feeRateBuy()/feeRateSell()
// in percent (types 8 and 9). Selectors derived via web3_sha3 on 2026-11-14.
const tokenInfos = "0xe684626b"
const feeRate = "0x978bbdb9"
const feeRateBuy = "0x509e0ec1"
const feeRateSell = "0x34fb976a"
const transferTopic = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
const deadAddress = "0x000000000000000000000000000000000000dead"
const zeroAddress = "0x0000000000000000000000000000000000000000"

// curveRoundTripGas covers buy, ERC20 approve and sell, from BSC receipts on
// 2026-11-14 (buy ~210k, sell ~193k, approve ~46k).
const curveRoundTripGas = 460_000

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
	gas, gasProof, gasErr := c.curveGas(ctx)
	proof = append(proof, gasProof...)
	if gasErr != nil {
		b.Gaps = append(b.Gaps, gasErr.Error())
	}
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
		// Chain facts replace GoPlus tax/holder fields, which stay empty until DEX listing.
		c.curveTax(ctx, &candidate, token, created[1][2:], block.Number)
		c.curveHolders(ctx, &candidate, token, entry.BlockNumber, block.Number)
		for key, value := range gas {
			candidate.Facts[key] = value
		}
		candidate.EvidenceIDs = append(candidate.EvidenceIDs, gasProof...)
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

// curveGas prices one buy+approve+sell round trip in USDT from the node's gas price
// and the Binance BNBUSDT ask. It is shared by every token in the batch.
func (c *Client) curveGas(ctx context.Context) (map[string]string, []string, error) {
	var price string
	id, err := c.rpc(ctx, "eth_gasPrice", []any{}, &price)
	proof := []string{id}
	wei, ok := new(big.Int).SetString(strings.TrimPrefix(price, "0x"), 16)
	if err != nil || !strings.HasPrefix(price, "0x") || !ok || wei.Sign() <= 0 {
		return nil, proof, errors.New("curve_gas_price_unavailable")
	}
	var book struct{ Symbol, AskPrice string }
	id, err = c.fetch(ctx, "https://data-api.binance.vision/api/v3/ticker/bookTicker?symbol=BNBUSDT", nil, &book)
	proof = append(proof, id)
	ask, askErr := decimal.NewFromString(book.AskPrice)
	if err != nil || book.Symbol != "BNBUSDT" || askErr != nil || !ask.IsPositive() {
		return nil, proof, errors.New("bnb_usdt_price_unavailable")
	}
	gas := decimal.NewFromBigInt(wei, -18).Mul(decimal.NewFromInt(curveRoundTripGas)).Mul(ask)
	return map[string]string{"gas_price_wei": wei.String(), "bnb_usdt_ask": ask.String(),
		"curve_round_trip_gas_usdt": gas.StringFixed(6)}, proof, nil
}

// curveTax reads the token's own buy/sell tax as a 0–1 fraction. Unrecognised
// creator types stay unknown instead of being read as untaxed.
func (c *Client) curveTax(ctx context.Context, candidate *Candidate, token, tokenWord, block string) {
	value, id, err := c.call(ctx, fourMemeManager, tokenInfos+tokenWord, block)
	candidate.EvidenceIDs = append(candidate.EvidenceIDs, id)
	info := words(value, 13)
	if err != nil || info == nil {
		candidate.Gaps = append(candidate.Gaps, "curve_tax_unknown")
		return
	}
	base, baseErr := abiAddress(info[0])
	template, templateErr := abiUint(info[2])
	if baseErr != nil || templateErr != nil || base != token {
		candidate.Gaps = append(candidate.Gaps, "curve_tax_unknown")
		return
	}
	creatorType := new(big.Int).Rsh(template, 10).Uint64() & 0x3f
	candidate.Facts["curve_creator_type"] = strconv.FormatUint(creatorType, 10)
	rate := func(selector string, scale int64) (string, bool) {
		value, id, err := c.call(ctx, token, selector, block)
		candidate.EvidenceIDs = append(candidate.EvidenceIDs, id)
		n, parseErr := abiUint(value)
		if err != nil || parseErr != nil || n.Cmp(big.NewInt(scale)) > 0 {
			return "", false
		}
		return new(big.Rat).SetFrac(n, big.NewInt(scale)).FloatString(4), true
	}
	var buy, sell string
	var buyOK, sellOK bool
	switch creatorType {
	case 0:
		// Plain token: on-chain buys on 2026-11-14 paid exactly curve cost plus protocol fee.
		buy, sell, buyOK, sellOK = "0.0000", "0.0000", true, true
	case 5:
		buy, buyOK = rate(feeRate, 10000)
		sell, sellOK = buy, buyOK
	case 8, 9:
		buy, buyOK = rate(feeRateBuy, 100)
		sell, sellOK = rate(feeRateSell, 100)
	default:
		candidate.Gaps = append(candidate.Gaps, "curve_creator_type_unrecognised")
		return
	}
	if !buyOK || !sellOK {
		candidate.Gaps = append(candidate.Gaps, "curve_tax_unknown")
		return
	}
	candidate.Facts["buy_tax"], candidate.Facts["sell_tax"], candidate.Facts["tax_source"] = buy, sell, "chain"
}

// curveHolders replays the token's Transfer logs from creation to the finalized
// block. The manager's unsold curve inventory, the token contract, zero and dead
// addresses are not holders who can sell.
func (c *Client) curveHolders(ctx context.Context, candidate *Candidate, token, fromBlock, toBlock string) {
	var logs []curveLog
	id, err := c.rpcAt(ctx, logsURL, "eth_getLogs", []any{map[string]any{
		"address": token, "topics": []string{transferTopic}, "fromBlock": fromBlock, "toBlock": toBlock}}, &logs)
	candidate.EvidenceIDs = append(candidate.EvidenceIDs, id)
	if err != nil {
		candidate.Gaps = append(candidate.Gaps, "curve_holders_unknown")
		return
	}
	balances := map[string]*big.Int{}
	supply := new(big.Int)
	for _, entry := range logs {
		if entry.Removed || !strings.EqualFold(entry.Address, token) || len(entry.Topics) != 3 || entry.Topics[0] != transferTopic {
			candidate.Gaps = append(candidate.Gaps, "curve_holders_unknown")
			return
		}
		from, fromErr := topicAddress(entry.Topics[1])
		to, toErr := topicAddress(entry.Topics[2])
		amount, amountErr := abiUint(entry.Data)
		if fromErr != nil || toErr != nil || amountErr != nil {
			candidate.Gaps = append(candidate.Gaps, "curve_holders_unknown")
			return
		}
		for _, side := range []struct {
			who  string
			sign int
		}{{from, -1}, {to, 1}} {
			delta := new(big.Int).Mul(amount, big.NewInt(int64(side.sign)))
			if side.who == zeroAddress {
				supply.Sub(supply, delta)
				continue
			}
			if balances[side.who] == nil {
				balances[side.who] = new(big.Int)
			}
			balances[side.who].Add(balances[side.who], delta)
		}
	}
	var shares []decimal.Decimal
	for who, balance := range balances {
		if balance.Sign() < 0 {
			// Logs before the window or a non-standard token: balances cannot be trusted.
			candidate.Gaps = append(candidate.Gaps, "curve_holders_unknown")
			return
		}
		if balance.Sign() > 0 && who != fourMemeManager && who != token && who != deadAddress {
			shares = append(shares, decimal.NewFromBigInt(balance, 0))
		}
	}
	if supply.Sign() <= 0 {
		candidate.Gaps = append(candidate.Gaps, "curve_holders_unknown")
		return
	}
	total := decimal.NewFromBigInt(supply, 0)
	for i := range shares {
		shares[i] = shares[i].Div(total)
	}
	holderFacts(candidate, shares, 0, decimal.Zero)
	candidate.Facts["holder_count"] = strconv.Itoa(len(shares))
	candidate.Facts["holder_source"] = "chain_transfers"
}

// topicAddress decodes an indexed address, keeping the zero address used by mint and burn.
func topicAddress(topic string) (string, error) {
	if topic == "0x"+strings.Repeat("0", 64) {
		return zeroAddress, nil
	}
	return abiAddress(topic)
}

// holderFacts records concentration among holders who can sell. unseen holders
// lie past the provider's top-10 list and replace excluded ones; each holds at
// most floor, the smallest share the provider showed.
func holderFacts(candidate *Candidate, shares []decimal.Decimal, unseen int, floor decimal.Decimal) {
	slices.SortFunc(shares, func(a, b decimal.Decimal) int { return b.Cmp(a) })
	top, top10 := decimal.Zero, decimal.Zero
	if len(shares) > 0 {
		top = shares[0]
	}
	for _, share := range shares[:min(len(shares), 10)] {
		top10 = top10.Add(share)
	}
	if unseen > 0 {
		top = decimal.Max(top, floor)
		top10 = top10.Add(floor.Mul(decimal.NewFromInt(int64(min(unseen, 10-min(len(shares), 10))))))
	}
	candidate.Facts["top_holder_fraction"] = top.StringFixed(6)
	candidate.Facts["top10_holder_fraction"] = top10.StringFixed(6)
}
