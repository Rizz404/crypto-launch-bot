// Package manual is the strategy of operator orders from the trading screen.
// It never trades on its own; the engine still sizes, risk-checks and
// protects what the operator submits, inside the instance's capital.
package manual

import (
	"errors"

	"github.com/rizz404/crypto-launch-bot/internal/domain"
	"github.com/rizz404/crypto-launch-bot/internal/strategy"
)

func init() { strategy.Register("manual", func() strategy.Strategy { return &Strategy{} }) }

type Strategy struct{}

func (s *Strategy) Init(_ strategy.Context, cfg strategy.Config) error {
	if cfg.Market != domain.Spot || len(cfg.Symbols) == 0 {
		return errors.New("manual needs spot symbols")
	}
	return nil
}

// Timeframes keeps 1m prices flowing so market orders have a fresh reference.
func (s *Strategy) Timeframes() []domain.Timeframe { return []domain.Timeframe{"1m"} }

func (s *Strategy) OnCandle(domain.Candle) []strategy.Intent { return nil }
