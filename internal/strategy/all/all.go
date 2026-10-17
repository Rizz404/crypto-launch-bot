// Package all registers every built-in strategy. Import it for side effects.
package all

import (
	_ "github.com/rizz404/trading-bot/internal/strategy/dca"
	_ "github.com/rizz404/trading-bot/internal/strategy/emacross"
	_ "github.com/rizz404/trading-bot/internal/strategy/funding"
	_ "github.com/rizz404/trading-bot/internal/strategy/grid"
	_ "github.com/rizz404/trading-bot/internal/strategy/mm"
	_ "github.com/rizz404/trading-bot/internal/strategy/xspread"
)
