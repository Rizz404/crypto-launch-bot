package risk

import "log/slog"

// Config holds the risk settings.
type Config struct {
	// Sizing
	RiskPerTrade   float64 `koanf:"risk_per_trade"`   // fraction of equity lost when the stop is hit
	PositionPct    float64 `koanf:"position_pct"`     // margin fraction per position when there is no stop
	MaxPositionPct float64 `koanf:"max_position_pct"` // cap: notional ≤ this × equity × leverage
	MaxLeverage    int     `koanf:"max_leverage"`
}

// DefaultConfig is deliberately conservative.
func DefaultConfig() Config {
	return Config{
		RiskPerTrade:   0.01,
		PositionPct:    0.10,
		MaxPositionPct: 0.25,
		MaxLeverage:    3,
	}
}

// Manager sizes orders and enforces limits.
type Manager struct {
	cfg Config
	log *slog.Logger
}

func NewManager(cfg Config, log *slog.Logger) *Manager {
	if log == nil {
		log = slog.Default()
	}
	return &Manager{cfg: cfg, log: log}
}

// Config returns the active configuration.
func (m *Manager) Config() Config { return m.cfg }
