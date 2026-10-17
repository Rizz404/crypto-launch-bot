package risk

import (
	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/portfolio"
)

// CheckOrder enforces account-level limits before an order that increases
// exposure is sent. Fase 2 has no limits yet; see fase 3.
func (m *Manager) CheckOrder(_ domain.Order, _ decimal.Decimal, _ *portfolio.Book) error {
	return nil
}
