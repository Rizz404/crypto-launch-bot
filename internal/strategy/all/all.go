// Package all registers every built-in strategy. Import it for side effects.
package all

import (
	_ "github.com/rizz404/crypto-launch-bot/internal/strategy/dca"
	_ "github.com/rizz404/crypto-launch-bot/internal/strategy/emacross"
	_ "github.com/rizz404/crypto-launch-bot/internal/strategy/funding"
	_ "github.com/rizz404/crypto-launch-bot/internal/strategy/grid"
	_ "github.com/rizz404/crypto-launch-bot/internal/strategy/manual"
	_ "github.com/rizz404/crypto-launch-bot/internal/strategy/mm"
	_ "github.com/rizz404/crypto-launch-bot/internal/strategy/trendspot"
	_ "github.com/rizz404/crypto-launch-bot/internal/strategy/xspread"
)
