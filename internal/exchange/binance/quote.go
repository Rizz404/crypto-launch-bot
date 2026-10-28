package binance

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/shopspring/decimal"
)

// SpotQuote is public top-of-book data; At conservatively precedes the request.
type SpotQuote struct {
	Symbol string          `json:"symbol"`
	Bid    decimal.Decimal `json:"bidPrice"`
	Ask    decimal.Decimal `json:"askPrice"`
	BidQty decimal.Decimal `json:"bidQty"`
	AskQty decimal.Decimal `json:"askQty"`
	At     time.Time       `json:"-"`
}

func (c *Client) SpotQuote(ctx context.Context, symbol string) (SpotQuote, error) {
	var q SpotQuote
	start := time.Now().UTC()
	err := c.get(ctx, domain.Spot, "/api/v3/ticker/bookTicker", url.Values{"symbol": {symbol}}, 2, &q)
	if err != nil {
		return q, err
	}
	if q.Symbol != symbol || !q.Bid.IsPositive() || !q.Ask.IsPositive() || q.Bid.GreaterThan(q.Ask) || q.BidQty.IsNegative() || q.AskQty.IsNegative() {
		return SpotQuote{}, errors.New("invalid public spot quote")
	}
	q.At = start
	return q, nil
}
