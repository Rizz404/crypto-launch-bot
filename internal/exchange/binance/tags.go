package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
)

// productsPath is the public product listing of the Binance website. It is
// not part of the documented API, so a failure falls back to a snapshot.
const productsPath = "/bapi/asset/v2/public/asset-service/product/get-products?includeEtf=true"

// AssetTags implements exchange.AssetTagger: the category tags of every
// listed base asset. If the listing cannot be read, the tokenized stocks
// known on 2026-10-18 are returned instead (with a warning), so exclusions
// based on "bStocks" keep holding.
func (c *Client) AssetTags(ctx context.Context) (map[string][]string, error) {
	tags, err := c.fetchAssetTags(ctx)
	if err == nil {
		return tags, nil
	}
	c.log.Warn("asset tags unavailable, using the bStocks snapshot", "err", err)
	tags = make(map[string][]string, len(bStocksSnapshot))
	for _, b := range bStocksSnapshot {
		tags[b] = []string{"bStocks"}
	}
	return tags, nil
}

func (c *Client) fetchAssetTags(ctx context.Context) (map[string][]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.ep.Web+productsPath, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("products: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("products: http %d", resp.StatusCode)
	}
	var body struct {
		Data []struct {
			Base string   `json:"b"`
			Tags []string `json:"tags"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&body); err != nil {
		return nil, fmt.Errorf("products: decode: %w", err)
	}
	if len(body.Data) == 0 {
		return nil, fmt.Errorf("products: empty listing")
	}
	tags := map[string][]string{}
	for _, p := range body.Data {
		for _, t := range p.Tags {
			if !slices.Contains(tags[p.Base], t) {
				tags[p.Base] = append(tags[p.Base], t)
			}
		}
	}
	return tags, nil
}
