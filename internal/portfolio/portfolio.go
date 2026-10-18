// Package portfolio keeps the ledger of each trading account: balance,
// one-way positions, realized and unrealized PnL, fees and funding.
//
// An account is one wallet on one exchange and market (Binance spot and
// futures are separate wallets). All balances are in the quote currency.
package portfolio

import (
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/domain"
)

// AccountKey identifies one wallet.
type AccountKey struct {
	Exchange string
	Market   domain.Market
}

func (k AccountKey) String() string { return k.Exchange + ":" + string(k.Market) }

// FillMeta is context about a fill that the exchange does not provide.
type FillMeta struct {
	Strategy string
	Tag      string // order tag; becomes the trade exit reason
	Leverage int
}

// Account is the ledger of one wallet. It is safe for concurrent use.
type Account struct {
	Key AccountKey

	mu        sync.Mutex
	balance   decimal.Decimal // cash incl. realized PnL, fees and funding
	positions map[string]*domain.Position
	marks     map[string]decimal.Decimal
	onTrade   func(domain.Trade)
}

// NewAccount starts a ledger with the given cash balance. onTrade, if not
// nil, is called (with the lock released) for every closed round trip.
func NewAccount(key AccountKey, balance decimal.Decimal, onTrade func(domain.Trade)) *Account {
	return &Account{
		Key:       key,
		balance:   balance,
		positions: map[string]*domain.Position{},
		marks:     map[string]decimal.Decimal{},
		onTrade:   onTrade,
	}
}

// ApplyFill updates the position and balance for one execution.
func (a *Account) ApplyFill(f domain.Fill, meta FillMeta) {
	a.mu.Lock()
	trades := a.applyFill(f, meta)
	a.mu.Unlock()
	if a.onTrade != nil {
		for _, t := range trades {
			a.onTrade(t)
		}
	}
}

func (a *Account) applyFill(f domain.Fill, meta FillMeta) []domain.Trade {
	a.balance = a.balance.Sub(f.Fee)
	a.marks[f.Symbol] = f.Price
	pos := a.positions[f.Symbol]

	fillSide := domain.Long
	if f.Side == domain.Sell {
		fillSide = domain.Short
	}

	// Open or add to a position.
	if pos == nil || pos.Side == fillSide {
		if pos == nil {
			pos = &domain.Position{
				PositionKey: domain.PositionKey{Exchange: f.Exchange, Market: f.Market, Symbol: f.Symbol},
				Side:        fillSide,
				Leverage:    max(meta.Leverage, 1),
				OpenedAt:    f.Time,
				Strategy:    meta.Strategy,
			}
			a.positions[f.Symbol] = pos
		}
		// A commission paid in the base asset never reaches the account.
		// Its value is already charged through Fee, so the entry price stays
		// the fill price (equity = cash - fee + held × (mark - entry)).
		held := f.Quantity.Sub(f.BaseFee)
		total := pos.Quantity.Add(held)
		pos.EntryPrice = pos.EntryPrice.Mul(pos.Quantity).Add(f.Price.Mul(held)).Div(total)
		pos.Quantity = total
		pos.Fees = pos.Fees.Add(f.Fee)
		return nil
	}

	// Reduce, close or flip.
	closeQty := decimal.Min(pos.Quantity, f.Quantity)
	gross := f.Price.Sub(pos.EntryPrice).Mul(closeQty).Mul(pos.Side.Sign())
	a.balance = a.balance.Add(gross)

	// Attribute the fill fee to the closed part, pro rata.
	closeFee := f.Fee.Mul(closeQty).Div(f.Quantity)
	share := closeQty.Div(pos.Quantity)
	trade := domain.Trade{
		PositionKey: pos.PositionKey,
		Strategy:    pos.Strategy,
		Side:        pos.Side,
		Quantity:    closeQty,
		EntryPrice:  pos.EntryPrice,
		ExitPrice:   f.Price,
		OpenedAt:    pos.OpenedAt,
		ClosedAt:    f.Time,
		GrossPnL:    gross,
		Fees:        pos.Fees.Mul(share).Add(closeFee),
		Funding:     pos.Funding.Mul(share),
		ExitReason:  meta.Tag,
	}
	pos.Fees = pos.Fees.Sub(pos.Fees.Mul(share))
	pos.Funding = pos.Funding.Sub(trade.Funding)
	pos.Quantity = pos.Quantity.Sub(closeQty)

	if pos.Quantity.IsZero() {
		delete(a.positions, f.Symbol)
	}
	if rest := f.Quantity.Sub(closeQty); rest.IsPositive() {
		a.positions[f.Symbol] = &domain.Position{
			PositionKey: pos.PositionKey,
			Side:        fillSide,
			Quantity:    rest,
			EntryPrice:  f.Price,
			Leverage:    max(meta.Leverage, 1),
			OpenedAt:    f.Time,
			Fees:        f.Fee.Sub(closeFee),
			Strategy:    meta.Strategy,
		}
	}
	return []domain.Trade{trade}
}

// Mark updates the mark price of symbol for unrealized PnL.
func (a *Account) Mark(symbol string, price decimal.Decimal) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.marks[symbol] = price
}

// ApplyFunding charges funding at rate (e.g. 0.0001) on the position of
// symbol: longs pay positive rates, shorts receive them.
func (a *Account) ApplyFunding(symbol string, rate decimal.Decimal, _ time.Time) decimal.Decimal {
	a.mu.Lock()
	defer a.mu.Unlock()
	pos := a.positions[symbol]
	if pos == nil {
		return decimal.Zero
	}
	mark, ok := a.marks[symbol]
	if !ok {
		mark = pos.EntryPrice
	}
	paid := pos.Notional(mark).Mul(rate).Mul(pos.Side.Sign())
	pos.Funding = pos.Funding.Add(paid)
	a.balance = a.balance.Sub(paid)
	return paid
}

// SetBalance replaces the cash balance, e.g. after reconciling with the
// exchange.
func (a *Account) SetBalance(b decimal.Decimal) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.balance = b
}

// SetPosition replaces (or removes, when qty is zero) a position, e.g. after
// reconciling with the exchange.
func (a *Account) SetPosition(p domain.Position) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if p.Quantity.IsZero() {
		delete(a.positions, p.Symbol)
		return
	}
	cp := p
	a.positions[p.Symbol] = &cp
}

// Position returns a copy of the position on symbol.
func (a *Account) Position(symbol string) (domain.Position, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if p := a.positions[symbol]; p != nil {
		return *p, true
	}
	return domain.Position{}, false
}

// Positions returns copies of all open positions, sorted by symbol.
func (a *Account) Positions() []domain.Position {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]domain.Position, 0, len(a.positions))
	for _, p := range a.positions {
		out = append(out, *p)
	}
	slices.SortFunc(out, func(x, y domain.Position) int { return strings.Compare(x.Symbol, y.Symbol) })
	return out
}

// Snapshot is a point-in-time view of an account.
type Snapshot struct {
	Balance       decimal.Decimal
	Equity        decimal.Decimal
	UnrealizedPnL decimal.Decimal
	Exposure      decimal.Decimal // sum of position notionals at mark
	UsedMargin    decimal.Decimal // sum of notional / leverage
	OpenPositions int
}

// FreeMargin is what is left for new positions.
func (s Snapshot) FreeMargin() decimal.Decimal { return s.Equity.Sub(s.UsedMargin) }

func (a *Account) Snapshot() Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := Snapshot{Balance: a.balance, OpenPositions: len(a.positions)}
	for sym, p := range a.positions {
		mark, ok := a.marks[sym]
		if !ok {
			mark = p.EntryPrice
		}
		n := p.Notional(mark)
		s.UnrealizedPnL = s.UnrealizedPnL.Add(p.UnrealizedPnL(mark))
		s.Exposure = s.Exposure.Add(n)
		s.UsedMargin = s.UsedMargin.Add(n.Div(decimal.NewFromInt(int64(max(p.Leverage, 1)))))
	}
	s.Equity = s.Balance.Add(s.UnrealizedPnL)
	return s
}

// Book holds the accounts of all wallets the bot trades.
type Book struct {
	mu       sync.Mutex
	accounts map[AccountKey]*Account
}

func NewBook() *Book { return &Book{accounts: map[AccountKey]*Account{}} }

// Add registers an account.
func (b *Book) Add(a *Account) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.accounts[a.Key] = a
}

// Account returns the account for key, or nil.
func (b *Book) Account(key AccountKey) *Account {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.accounts[key]
}

// Accounts returns all accounts sorted by key.
func (b *Book) Accounts() []*Account {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]*Account, 0, len(b.accounts))
	for _, a := range b.accounts {
		out = append(out, a)
	}
	slices.SortFunc(out, func(x, y *Account) int { return strings.Compare(x.Key.String(), y.Key.String()) })
	return out
}

// Total sums the snapshots of all accounts.
func (b *Book) Total() Snapshot {
	var t Snapshot
	for _, a := range b.Accounts() {
		s := a.Snapshot()
		t.Balance = t.Balance.Add(s.Balance)
		t.Equity = t.Equity.Add(s.Equity)
		t.UnrealizedPnL = t.UnrealizedPnL.Add(s.UnrealizedPnL)
		t.Exposure = t.Exposure.Add(s.Exposure)
		t.UsedMargin = t.UsedMargin.Add(s.UsedMargin)
		t.OpenPositions += s.OpenPositions
	}
	return t
}
