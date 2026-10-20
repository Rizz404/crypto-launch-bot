// Package managed owns virtual capital and strategy inventories in one physical
// spot wallet. All mutations are serialized and journaled before network I/O.
package managed

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"time"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/risk"
	"github.com/rizz404/trading-bot/internal/strategy"
	"github.com/shopspring/decimal"
)

var validID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)
var ErrFunds = errors.New("insufficient unreserved capital or inventory")
var ErrAttention = errors.New("wallet needs reconciliation")

type Spec struct {
	ID        string          `koanf:"id"`
	Exchange  string          `koanf:"exchange"`
	Mode      string          `koanf:"-"`
	Quote     string          `koanf:"quote"`
	Capital   decimal.Decimal `koanf:"capital"`
	Risk      risk.Config     `koanf:"risk"`
	Instances []InstanceSpec  `koanf:"instances"`
}
type InstanceSpec struct {
	ID         string          `koanf:"id"`
	Name       string          `koanf:"name"`
	Allocation decimal.Decimal `koanf:"allocation"`
	Risk       risk.Config     `koanf:"risk"`
	Notify     string          `koanf:"notify"`
	Strategies []BindingSpec   `koanf:"strategies"`
}
type BindingSpec struct {
	ID      string          `koanf:"id"`
	Name    string          `koanf:"name"`
	Symbols []string        `koanf:"symbols"`
	Params  map[string]any  `koanf:"params"`
	Cap     decimal.Decimal `koanf:"cap"`
	Risk    risk.Config     `koanf:"risk"`
}

func (b BindingSpec) Config(exchange, owner string) strategy.Config {
	return strategy.Config{ID: owner, Name: b.Name, Exchange: exchange, Market: domain.Spot, Symbols: slices.Clone(b.Symbols), Leverage: 1, Params: b.Params}
}
func (s Spec) Validate() error {
	if !validID.MatchString(s.ID) || s.Exchange == "" || s.Quote != "USDT" || !s.Capital.IsPositive() {
		return errors.New("managed wallet needs a valid id, exchange, USDT quote and positive capital")
	}
	if !slices.Contains([]string{"paper", "testnet", "live", "backtest"}, s.Mode) {
		return errors.New("invalid managed wallet mode")
	}
	if err := validateRisk(s.Risk); err != nil {
		return err
	}
	ids := map[string]bool{}
	total := decimal.Zero
	for _, i := range s.Instances {
		if !validID.MatchString(i.ID) || i.ID == "account" || i.ID == "reserve" || ids[i.ID] || i.Allocation.IsNegative() {
			return fmt.Errorf("invalid or duplicate instance %q", i.ID)
		}
		ids[i.ID] = true
		total = total.Add(i.Allocation)
		if !slices.Contains([]string{"", "info", "warn", "error", "off"}, i.Notify) {
			return fmt.Errorf("invalid notification level for %s", i.ID)
		}
		if err := validateRisk(i.Risk); err != nil {
			return err
		}
		bindings := map[string]bool{}
		for _, b := range i.Strategies {
			if !validID.MatchString(b.ID) || bindings[b.ID] || b.Name == "" || len(b.Symbols) == 0 || b.Cap.IsNegative() {
				return fmt.Errorf("invalid binding in %s", i.ID)
			}
			bindings[b.ID] = true
			if err := validateRisk(b.Risk); err != nil {
				return err
			}
			for _, sym := range b.Symbols {
				if len(sym) <= len(s.Quote) || !regexp.MustCompile(`^[A-Z0-9]+USDT$`).MatchString(sym) {
					return fmt.Errorf("invalid USDT symbol %q", sym)
				}
			}
		}
	}
	if total.GreaterThan(s.Capital) {
		return ErrFunds
	}
	return nil
}
func (s Spec) Key() string { return s.Mode + ":" + s.ID }
func Hash(v any) string    { b, _ := json.Marshal(v); return fmt.Sprintf("%x", sha256.Sum256(b)) }

type Instance struct {
	ID, Name, Status, Notify     string
	Cash, Funded, Peak, DayStart decimal.Decimal
	Day                          time.Time
	Risk                         risk.Config
	Halted                       bool
	Reason                       string
}
type Binding struct {
	ID, InstanceID, Status, Reason string
	LastNotice                     string
	Spec                           BindingSpec
	Realized, Fees, Peak, DayStart decimal.Decimal
	Day                            time.Time
	Halted                         bool
	Engine                         json.RawMessage
}
type Holding struct {
	Owner, Symbol  string
	Quantity, Cost decimal.Decimal
}
type Order struct {
	Deferred                    *domain.Order
	Owner                       string
	Order                       domain.Order
	Group                       string
	QuoteReserved, BaseReserved decimal.Decimal
	State                       string // submitting, open, canceling, terminal, unknown
}
type Audit struct {
	At                   time.Time
	Kind, Scope, Message string
}
type FeeCharge struct {
	Owner, Asset string
	Quantity     decimal.Decimal
}
type Execution struct {
	Owner    string
	Fill     domain.Fill
	Realized decimal.Decimal
}
type EquitySample struct {
	At        time.Time
	Account   decimal.Decimal
	Instances map[string]decimal.Decimal
}
type Run struct {
	ID, Mode, Status, ConfigurationHash string
	StartedAt, EndedAt                  time.Time
	Configuration                       json.RawMessage
}
type State struct {
	Runs           []Run
	Samples        []EquitySample
	PriceAt        map[string]time.Time
	ReconciledAt   time.Time
	Executions     []Execution
	FeeAssets      map[string]decimal.Decimal
	PendingFees    map[string]FeeCharge
	FeeQuote       decimal.Decimal
	FeeIssue       string
	Pending        []broker.Event
	Version        int
	Spec           Spec
	ConfigHash     string
	Reserve        decimal.Decimal
	Instances      map[string]*Instance
	Bindings       map[string]*Binding
	Holdings       map[string]*Holding
	Orders         map[string]*Order
	Seen           map[string]bool
	Prices         map[string]decimal.Decimal
	Baseline       map[string]decimal.Decimal
	Requests       map[string]string
	History        []Audit
	Attention      string
	Peak, DayStart decimal.Decimal
	Day            time.Time
	Halted         bool
	Reason         string
	StartedAt      time.Time
}

func NewState(spec Spec) (*State, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	s := &State{Version: 1, PriceAt: map[string]time.Time{}, FeeAssets: map[string]decimal.Decimal{}, PendingFees: map[string]FeeCharge{}, Spec: spec, ConfigHash: Hash(spec), Reserve: spec.Capital, Instances: map[string]*Instance{}, Bindings: map[string]*Binding{}, Holdings: map[string]*Holding{}, Orders: map[string]*Order{}, Seen: map[string]bool{}, Prices: map[string]decimal.Decimal{}, Baseline: map[string]decimal.Decimal{}, Requests: map[string]string{}, StartedAt: time.Now().UTC()}
	for _, i := range spec.Instances {
		s.Instances[i.ID] = &Instance{ID: i.ID, Name: i.Name, Status: "running", Notify: i.Notify, Cash: i.Allocation, Funded: i.Allocation, Risk: i.Risk}
		s.Reserve = s.Reserve.Sub(i.Allocation)
		for _, b := range i.Strategies {
			id := i.ID + "/" + b.ID
			s.Bindings[id] = &Binding{ID: id, InstanceID: i.ID, Status: "running", Spec: b}
		}
	}
	return s, nil
}
func (s *State) Note(kind, scope, msg string) {
	s.History = append(s.History, Audit{At: time.Now().UTC(), Kind: kind, Scope: scope, Message: msg})
}
func (s *State) Holding(owner, symbol string) *Holding {
	k := owner + ":" + symbol
	p := s.Holdings[k]
	if p == nil {
		p = &Holding{Owner: owner, Symbol: symbol}
		s.Holdings[k] = p
	}
	return p
}
func (s *State) Mark(symbol string, price decimal.Decimal) {
	if price.IsPositive() {
		s.Prices[symbol] = price
		s.valueFees()
	}
}
func (s *State) Inventory(owner string) decimal.Decimal {
	v := decimal.Zero
	for _, p := range s.Holdings {
		if owner == "" || p.Owner == owner {
			price := s.Prices[p.Symbol]
			if !price.IsPositive() && p.Quantity.IsPositive() {
				price = p.Cost.Div(p.Quantity)
			}
			v = v.Add(p.Quantity.Mul(price))
		}
	}
	return v
}
func (s *State) InstanceEquity(id string) decimal.Decimal {
	i := s.Instances[id]
	if i == nil {
		return decimal.Zero
	}
	v := i.Cash
	for owner, b := range s.Bindings {
		if b.InstanceID == id {
			v = v.Add(s.Inventory(owner))
		}
	}
	return v
}
func (s *State) Equity() decimal.Decimal {
	v := s.Reserve
	for id := range s.Instances {
		v = v.Add(s.InstanceEquity(id))
	}
	return v
}
func (s *State) Reserved(instance, owner, symbol string) (decimal.Decimal, decimal.Decimal) {
	q, b := decimal.Zero, decimal.Zero
	groups := map[string]bool{}
	for id, o := range s.Orders {
		if o.State == "terminal" {
			continue
		}
		binding := s.Bindings[o.Owner]
		if binding == nil || (instance != "" && binding.InstanceID != instance) || (owner != "" && o.Owner != owner) || (symbol != "" && o.Order.Symbol != symbol) {
			continue
		}
		group := o.Group
		if group == "" {
			group = id
		}
		if groups[group] {
			continue
		}
		groups[group] = true
		q = q.Add(o.QuoteReserved)
		b = b.Add(o.BaseReserved)
	}
	return q, b
}
func (s *State) FreeCash(id string) decimal.Decimal {
	i := s.Instances[id]
	if i == nil {
		return decimal.Zero
	}
	q, _ := s.Reserved(id, "", "")
	return i.Cash.Sub(q)
}
func (s *State) Transfer(from, to string, amount decimal.Decimal) error {
	if !amount.IsPositive() || from == to {
		return errors.New("transfer requires positive amount and different endpoints")
	}
	if s.Attention != "" {
		return ErrAttention
	}
	for _, id := range []string{from, to} {
		if id != "reserve" {
			i := s.Instances[id]
			if i == nil || i.Status == "archived" {
				return errors.New("unknown or archived instance")
			}
		}
	}
	available := s.Reserve
	if from != "reserve" {
		available = s.FreeCash(from)
	}
	if available.LessThan(amount) {
		return ErrFunds
	}
	if from == "reserve" {
		s.Reserve = s.Reserve.Sub(amount)
	} else {
		i := s.Instances[from]
		i.Cash = i.Cash.Sub(amount)
		i.Funded = i.Funded.Sub(amount)
		i.Peak = i.Peak.Sub(amount)
		i.DayStart = i.DayStart.Sub(amount)
	}
	if to == "reserve" {
		s.Reserve = s.Reserve.Add(amount)
	} else {
		i := s.Instances[to]
		i.Cash = i.Cash.Add(amount)
		i.Funded = i.Funded.Add(amount)
		i.Peak = i.Peak.Add(amount)
		i.DayStart = i.DayStart.Add(amount)
	}
	s.Note("transfer", from+" → "+to, amount.String())
	return nil
}

func validateRisk(c risk.Config) error {
	if c == (risk.Config{}) {
		return nil
	}
	for _, v := range []float64{c.RiskPerTrade, c.PositionPct, c.MaxPositionPct, c.MaxExposurePct, c.DailyLossLimit, c.MaxDrawdown} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return errors.New("risk settings must be finite and nonnegative")
		}
	}
	if c.RiskPerTrade <= 0 || c.RiskPerTrade > 0.1 || c.MaxLeverage != 1 || c.PositionPct > 1 || c.MaxPositionPct > 1 || c.DailyLossLimit >= 1 || c.MaxDrawdown >= 1 {
		return errors.New("managed risk requires leverage 1, risk per trade <=10%, fractions below 100%")
	}
	return nil
}
