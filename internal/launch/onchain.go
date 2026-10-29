package launch

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"time"
)

const pancakeFactory = "0xca143ce32fe78f1f7019d7d551a6402fc5350c73"

// Selectors belong to the published PancakeSwap V2 factory/pair ABI.
const allPairsLength = "0x574f2ba3"
const allPairs = "0x1e3dd18b"
const token0 = "0x0dfe1681"
const token1 = "0xd21220a7"

type chainSnapshot struct{ Number, Hash, Timestamp string }

func (c *Client) finalized(ctx context.Context) (chainSnapshot, []string, error) {
	var chain string
	id, err := c.rpc(ctx, "eth_chainId", []any{}, &chain)
	ids := []string{id}
	if err != nil || chain != "0x38" {
		return chainSnapshot{}, ids, errors.New("BSC chain identity unavailable or mismatched")
	}
	var block chainSnapshot
	id, err = c.rpc(ctx, "eth_getBlockByNumber", []any{"finalized", false}, &block)
	ids = append(ids, id)
	ts, parseErr := strconv.ParseInt(strings.TrimPrefix(block.Timestamp, "0x"), 16, 64)
	_, numberErr := strconv.ParseUint(strings.TrimPrefix(block.Number, "0x"), 16, 64)
	_, hashErr := hex.DecodeString(strings.TrimPrefix(block.Hash, "0x"))
	if err != nil || parseErr != nil || numberErr != nil || hashErr != nil || !strings.HasPrefix(block.Number, "0x") || !strings.HasPrefix(block.Hash, "0x") || len(block.Hash) != 66 || ts <= 0 || time.Since(time.Unix(ts, 0)) > 2*time.Minute || time.Unix(ts, 0).After(time.Now().Add(5*time.Second)) {
		return block, ids, errors.New("fresh finalized BSC block unavailable")
	}
	return block, ids, nil
}

func (c *Client) call(ctx context.Context, to, data, block string) (string, string, error) {
	var value string
	id, err := c.rpc(ctx, "eth_call", []any{map[string]string{"to": to, "data": data}, block}, &value)
	return value, id, err
}

func abiAddress(value string) (string, error) {
	if len(value) != 66 || !strings.HasPrefix(value, "0x"+strings.Repeat("0", 24)) {
		return "", errors.New("invalid ABI address word")
	}
	return address("0x" + value[26:])
}

func abiUint(value string) (*big.Int, error) {
	if len(value) != 66 || !strings.HasPrefix(value, "0x") {
		return nil, errors.New("invalid ABI uint word")
	}
	n, ok := new(big.Int).SetString(value[2:], 16)
	if !ok {
		return nil, errors.New("invalid ABI uint")
	}
	return n, nil
}

func (c *Client) inspectToken(ctx context.Context, candidate *Candidate, block chainSnapshot) {
	addr, err := address(candidate.Asset.Address)
	if err != nil {
		candidate.Gaps = append(candidate.Gaps, "token_address_unknown")
		return
	}
	if candidate.Facts == nil {
		candidate.Facts = map[string]string{}
	}
	candidate.Facts["chain_verified"] = "56"
	candidate.Facts["finalized_block"] = block.Number
	candidate.Facts["finalized_hash"] = block.Hash
	var code string
	id, err := c.rpc(ctx, "eth_getCode", []any{addr, block.Number}, &code)
	candidate.EvidenceIDs = append(candidate.EvidenceIDs, id)
	codeBytes, codeErr := hex.DecodeString(strings.TrimPrefix(code, "0x"))
	if err != nil || codeErr != nil || !strings.HasPrefix(code, "0x") || len(codeBytes) == 0 {
		candidate.Gaps = append(candidate.Gaps, "token_code_missing")
		return
	}
	candidate.Facts["contract_code"] = "present"
	var security struct {
		Code   int                                   `json:"code"`
		Result map[string]map[string]json.RawMessage `json:"result"`
	}
	id, err = c.fetch(ctx, "https://api.gopluslabs.io/api/v1/token_security/56?contract_addresses="+addr, nil, &security)
	candidate.EvidenceIDs = append(candidate.EvidenceIDs, id)
	if err != nil || security.Code != 1 || len(security.Result[addr]) == 0 {
		candidate.Gaps = append(candidate.Gaps, "security_data_unknown")
		return
	}
	for key, raw := range security.Result[addr] {
		if !slices.Contains([]string{"is_honeypot", "cannot_sell_all", "is_blacklisted", "is_proxy", "is_mintable", "owner_change_balance", "slippage_modifiable", "transfer_pausable", "is_open_source", "buy_tax", "sell_tax", "owner_address", "holder_count"}, key) {
			continue
		}
		var value string
		if json.Unmarshal(raw, &value) == nil {
			candidate.Facts[key] = value
		}
	}
	// GoPlus is a corroborating scanner; its findings do not prove sellability.
	candidate.Facts["security_provider"] = "goplus"
}

func poolWindow(count uint64, limit int, next *uint64) (uint64, uint64, error) {
	if limit < 1 {
		return 0, 0, errors.New("invalid pool scan limit")
	}
	start := count - min(count, uint64(limit))
	if next != nil {
		if *next > count {
			return 0, 0, errors.New("factory_registry_regressed")
		}
		if *next < count {
			start = *next
		}
	}
	return start, start + min(count-start, uint64(limit)), nil
}

func (c *Client) discoverDEX(ctx context.Context, limit int, next *uint64) Batch {
	b := Batch{Version: Version, Kind: "dex", At: time.Now().UTC()}
	block, proof, err := c.finalized(ctx)
	if err != nil {
		b.Gaps = append(b.Gaps, err.Error())
		return b
	}
	value, id, err := c.call(ctx, pancakeFactory, allPairsLength, block.Number)
	proof = append(proof, id)
	b.EvidenceIDs = proof
	count, parseErr := abiUint(value)
	if err != nil || parseErr != nil || !count.IsUint64() {
		b.Gaps = append(b.Gaps, "factory_registry_unavailable")
		return b
	}
	n := count.Uint64()
	start, end, err := poolWindow(n, limit, next)
	if err != nil {
		b.Gaps = append(b.Gaps, err.Error())
		return b
	}
	cursor := start
	b.NextPoolIndex = &cursor
	b.Gaps = append(b.Gaps, "bounded_factory_tail_not_full_30_day_history")
	if end < n {
		b.Gaps = append(b.Gaps, "factory_backlog_pending")
	}
	for index := start; index < end; index++ {
		value, id, err = c.call(ctx, pancakeFactory, allPairs+fmt.Sprintf("%064x", index), block.Number)
		pool, parseErr := abiAddress(value)
		if err != nil || parseErr != nil {
			b.Gaps = append(b.Gaps, "factory_pair_read_failed")
			break
		}
		candidate := Candidate{Kind: "dex", Venue: "pancakeswap-v2", ExternalID: pool,
			URL: "https://dexscreener.com/bsc/" + pool, ObservedAt: time.Now().UTC(),
			Facts: map[string]string{"factory": pancakeFactory, "factory_index": strconv.FormatUint(index, 10)}, EvidenceIDs: append(append([]string{}, proof...), id)}
		v0, id0, e0 := c.call(ctx, pool, token0, block.Number)
		v1, id1, e1 := c.call(ctx, pool, token1, block.Number)
		a0, p0 := abiAddress(v0)
		a1, p1 := abiAddress(v1)
		candidate.EvidenceIDs = append(candidate.EvidenceIDs, id0, id1)
		if e0 != nil || e1 != nil || p0 != nil || p1 != nil || a0 == a1 {
			b.Gaps = append(b.Gaps, "pair_tokens_unavailable")
			break
		}
		candidate.Asset = Asset{Chain: "eip155:56", Address: a0}
		candidate.Quote = a1
		var data struct {
			Pairs []struct {
				ChainID       string                           `json:"chainId"`
				PairAddress   string                           `json:"pairAddress"`
				DexID         string                           `json:"dexId"`
				BaseToken     struct{ Address, Symbol string } `json:"baseToken"`
				QuoteToken    struct{ Address, Symbol string } `json:"quoteToken"`
				PairCreatedAt int64                            `json:"pairCreatedAt"`
				PriceUSD      string                           `json:"priceUsd"`
				Liquidity     struct {
					USD *float64 `json:"usd"`
				} `json:"liquidity"`
			} `json:"pairs"`
		}
		id, err = c.fetch(ctx, "https://api.dexscreener.com/latest/dex/pairs/bsc/"+pool, nil, &data)
		candidate.EvidenceIDs = append(candidate.EvidenceIDs, id)
		matched := false
		if err == nil {
			for _, pair := range data.Pairs {
				if pair.ChainID != "bsc" || !strings.EqualFold(pair.PairAddress, pool) || pair.DexID != "pancakeswap" {
					continue
				}
				base, be := address(pair.BaseToken.Address)
				quote, qe := address(pair.QuoteToken.Address)
				if be != nil || qe != nil || !((base == a0 && quote == a1) || (base == a1 && quote == a0)) {
					candidate.Gaps = append(candidate.Gaps, "provider_pair_identity_conflict")
					continue
				}
				if matched {
					candidate.Gaps = append(candidate.Gaps, "provider_pair_duplicate")
					continue
				}
				matched = true
				candidate.Asset = Asset{Chain: "eip155:56", Address: base, Symbol: pair.BaseToken.Symbol}
				candidate.Quote = quote
				if pair.PairCreatedAt > 0 {
					candidate.StartsAt = time.UnixMilli(pair.PairCreatedAt).UTC()
				}
				candidate.TimeBasis = "provider_pool_creation"
				candidate.Facts["price_usd"] = pair.PriceUSD
				if pair.Liquidity.USD != nil {
					candidate.Facts["liquidity_usd"] = strconv.FormatFloat(*pair.Liquidity.USD, 'f', -1, 64)
				}
			}
		}
		if !matched {
			candidate.Gaps = append(candidate.Gaps, "market_metadata_unavailable")
		}
		c.inspectToken(ctx, &candidate, block)
		candidate.FirstSeenAt = candidate.ObservedAt
		candidate.Seal()
		b.Candidates = append(b.Candidates, candidate)
		cursor = index + 1
	}
	return b
}
