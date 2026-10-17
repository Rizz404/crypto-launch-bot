package risk

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/portfolio"
)

// CheckOrder enforces account-level limits before an order that increases
// exposure is sent. price is the expected execution price.
func (m *Manager) CheckOrder(o domain.Order, price decimal.Decimal, book *portfolio.Book) error {
	m.mu.Lock()
	halted, reason := m.halted, m.haltReason
	m.mu.Unlock()
	if halted {
		return fmt.Errorf("kill switch active: %s", reason)
	}

	total := book.Total()
	if m.cfg.MaxOpenPositions > 0 && o.Tag == domain.TagEntry && total.OpenPositions >= m.cfg.MaxOpenPositions {
		if acct := book.Account(portfolio.AccountKey{Exchange: o.Exchange, Market: o.Market}); acct != nil {
			if _, has := acct.Position(o.Symbol); !has {
				return fmt.Errorf("max open positions reached (%d)", m.cfg.MaxOpenPositions)
			}
		}
	}
	if m.cfg.MaxExposurePct > 0 && total.Equity.IsPositive() {
		after := total.Exposure.Add(o.Quantity.Mul(price))
		limit := total.Equity.Mul(decimal.NewFromFloat(m.cfg.MaxExposurePct))
		if after.GreaterThan(limit) {
			return fmt.Errorf("exposure %s would exceed limit %s (%.0f%% of equity)",
				after.StringFixed(2), limit.StringFixed(2), m.cfg.MaxExposurePct*100)
		}
	}
	return nil
}

// Observe feeds the current total equity to the kill switch. It returns
// true exactly once, when the switch trips.
func (m *Manager) Observe(now time.Time, equity decimal.Decimal) (tripped bool, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	day := now.UTC().Truncate(24 * time.Hour)
	if !day.Equal(m.day) {
		m.day, m.dayStart = day, equity
	}
	if equity.GreaterThan(m.peak) {
		m.peak = equity
	}
	if m.halted || !equity.IsPositive() && m.peak.IsZero() {
		return false, ""
	}

	if m.cfg.DailyLossLimit > 0 && m.dayStart.IsPositive() {
		loss := m.dayStart.Sub(equity).Div(m.dayStart)
		if loss.GreaterThanOrEqual(decimal.NewFromFloat(m.cfg.DailyLossLimit)) {
			reason = fmt.Sprintf("daily loss %s%% reached limit %.1f%%", loss.Shift(2).StringFixed(2), m.cfg.DailyLossLimit*100)
		}
	}
	if reason == "" && m.cfg.MaxDrawdown > 0 && m.peak.IsPositive() {
		dd := m.peak.Sub(equity).Div(m.peak)
		if dd.GreaterThanOrEqual(decimal.NewFromFloat(m.cfg.MaxDrawdown)) {
			reason = fmt.Sprintf("drawdown %s%% reached limit %.1f%%", dd.Shift(2).StringFixed(2), m.cfg.MaxDrawdown*100)
		}
	}
	if reason == "" {
		return false, ""
	}
	m.halted, m.haltReason = true, reason
	m.log.Error("kill switch tripped", "reason", reason, "equity", equity, "peak", m.peak, "day_start", m.dayStart)
	return true, reason
}

// Halt trips the kill switch manually.
func (m *Manager) Halt(reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.halted, m.haltReason = true, reason
}

// Reset clears the kill switch and restarts drawdown tracking from equity.
func (m *Manager) Reset(equity decimal.Decimal) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.halted, m.haltReason = false, ""
	m.peak, m.dayStart = equity, equity
}

// Halted reports whether the kill switch is active and why.
func (m *Manager) Halted() (bool, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.halted, m.haltReason
}
