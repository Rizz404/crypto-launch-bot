package risk

import (
	"log/slog"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

// Config holds the risk settings.
type Config struct {
	// Sizing
	RiskPerTrade   float64 `koanf:"risk_per_trade"`   // fraction of equity lost when the stop is hit
	PositionPct    float64 `koanf:"position_pct"`     // margin fraction per position when there is no stop
	MaxPositionPct float64 `koanf:"max_position_pct"` // cap: notional ≤ this × equity × leverage
	MaxLeverage    int     `koanf:"max_leverage"`

	// Limits
	MaxOpenPositions int     `koanf:"max_open_positions"`
	MaxExposurePct   float64 `koanf:"max_exposure_pct"` // total notional ≤ this × equity

	// Kill switch: halts new entries (and optionally closes everything)
	// when equity falls this far below the UTC day start or the peak.
	DailyLossLimit float64 `koanf:"daily_loss_limit"`
	MaxDrawdown    float64 `koanf:"max_drawdown"`
	CloseOnKill    bool    `koanf:"close_on_kill"`
}

// DefaultConfig is deliberately conservative.
func DefaultConfig() Config {
	return Config{
		RiskPerTrade:   0.01,
		PositionPct:    0.10,
		MaxPositionPct: 0.25,
		MaxLeverage:    3,

		MaxOpenPositions: 3,
		MaxExposurePct:   1.5,
		DailyLossLimit:   0.05,
		MaxDrawdown:      0.20,
		CloseOnKill:      true,
	}
}

// Manager sizes orders and enforces limits. It is safe for concurrent use.
type Manager struct {
	cfg Config
	log *slog.Logger

	mu         sync.Mutex
	day        time.Time
	dayStart   decimal.Decimal
	peak       decimal.Decimal
	halted     bool
	haltReason string
}

func NewManager(cfg Config, log *slog.Logger) *Manager {
	if log == nil {
		log = slog.Default()
	}
	return &Manager{cfg: cfg, log: log}
}

// Config returns the active configuration.
func (m *Manager) Config() Config { return m.cfg }
