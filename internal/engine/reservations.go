package engine

import (
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/shopspring/decimal"
)

// Called under the engine lock: orders stay reserved until their fills or
// cancellation acknowledgements have actually been processed.
func (e *Engine) pendingRiskOrders() []domain.Order {
	var out []domain.Order
	for _, order := range e.orders {
		if order.ReduceOnly || !order.Status.Open() || !order.Remaining().IsPositive() {
			continue
		}
		o := *order
		if !o.Price.IsPositive() {
			o.Price = e.lastPrice[instKey{o.Exchange, o.Market, o.Symbol}]
			if o.Type.IsConditional() {
				o.Price = decimal.Max(o.Price, o.StopPrice)
			}
		}
		out = append(out, o)
	}
	return out
}

func (e *Engine) pendingMargin(exchange string, market domain.Market) decimal.Decimal {
	margin := decimal.Zero
	for _, o := range e.pendingRiskOrders() {
		if o.Exchange != exchange || o.Market != market {
			continue
		}
		key := domain.PositionKey{Exchange: exchange, Market: market, Symbol: o.Symbol}
		lev := decimal.NewFromInt(int64(max(e.leverage[key], 1)))
		margin = margin.Add(o.Remaining().Mul(o.Price).Div(lev))
	}
	return margin
}
