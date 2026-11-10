package broker

import (
	"context"
	"sync"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/domain"
)

// Balance of one asset.
type Balance struct {
	Asset  string
	Free   decimal.Decimal
	Locked decimal.Decimal
	// Wallet is the total balance: for futures the wallet balance (realized
	// PnL included, unrealized excluded), for spot free + locked.
	Wallet decimal.Decimal
}

// AccountState is a snapshot of one wallet on an exchange.
type AccountState struct {
	Market     domain.Market
	Balances   map[string]Balance
	Positions  []domain.Position // futures only; spot holdings are balances
	OpenOrders []domain.Order    // regular and conditional
}

// Live is a broker connected to a real exchange account (testnet or
// mainnet). Exchange adapters implement it.
type Live interface {
	Broker
	BracketPlacer
	LeverageSetter
	// Start prepares the account and runs the user data streams until ctx
	// is cancelled; goroutines are added to wg.
	Start(ctx context.Context, wg *sync.WaitGroup) error
	// AccountState fetches balances, positions and open orders.
	AccountState(ctx context.Context, m domain.Market) (*AccountState, error)
	// SyncOrders emits missed fills and final states of tracked orders that
	// are no longer open on the exchange.
	SyncOrders(ctx context.Context, tracked, exchangeOpen []domain.Order)
	// Track registers orders placed in an earlier session.
	Track(orders []domain.Order)
	// Resync signals that events may have been missed (stream reconnect).
	Resync() <-chan struct{}
}
