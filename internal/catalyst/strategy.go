package catalyst

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/strategy"
	"github.com/shopspring/decimal"
)

// Registration is intentionally confined to the separate catalyst CLI/tests.
// The P2 bot's strategy/all import does not include this package.
func init() { strategy.Register("catalyst", func() strategy.Strategy { return &Strategy{} }) }

// Input is written by trusted research/market adapters, not by the model.
// Verification and market-data adapters must be accepted before a real pilot.
type Input struct {
	Policy   Policy        `json:"policy"`
	Review   Review        `json:"review"`
	Evidence []Evidence    `json:"evidence"`
	Event    VerifiedEvent `json:"event"`
	Quote    Quote         `json:"quote"`
}

type PositionState struct {
	EventKey, ReviewID string
	Peak, Stop         decimal.Decimal
	Trail              decimal.Decimal
	ReviewBefore       time.Time
	ReviewedAt         time.Time
	EntryExpiresAt     time.Time
	Closing            bool
}

type Strategy struct {
	ctx    strategy.Context
	cfg    strategy.Config
	params struct {
		Inbox         string `mapstructure:"inbox"`
		PolicyHash    string `mapstructure:"policy_hash"`
		ExecutionHash string `mapstructure:"execution_hash"`
	}
	state struct {
		Version    int
		UsedEvents map[string]bool
		Positions  map[string]*PositionState
	}
}

func (s *Strategy) Init(ctx strategy.Context, cfg strategy.Config) error {
	if cfg.Exchange != "binance" || cfg.Market != domain.Spot || cfg.Leverage > 1 || len(cfg.Legs) != 0 || len(cfg.Symbols) != 1 {
		return errors.New("catalyst needs one unleveraged Binance Spot symbol per binding")
	}
	if err := strategy.DecodeParams(cfg.Params, &s.params); err != nil {
		return err
	}
	if s.params.Inbox == "" || !filepath.IsAbs(s.params.Inbox) || len(s.params.PolicyHash) != 64 {
		return errors.New("catalyst requires an absolute isolated inbox and frozen policy hash")
	}
	if s.params.ExecutionHash != Hash(DevelopmentEntryPolicy()) {
		return errors.New("catalyst execution policy changed; use a new experiment version")
	}
	s.ctx, s.cfg = ctx, cfg
	s.state.Version = 1
	s.state.UsedEvents = map[string]bool{}
	s.state.Positions = map[string]*PositionState{}
	return nil
}

func (s *Strategy) Timeframes() []domain.Timeframe             { return []domain.Timeframe{"1m"} }
func (s *Strategy) Interval() time.Duration                    { return time.Second }
func (s *Strategy) OnCandle(c domain.Candle) []strategy.Intent { return s.OnPrice(c) }

func (s *Strategy) OnPrice(c domain.Candle) []strategy.Intent {
	if c.Exchange != s.cfg.Exchange || c.Market != domain.Spot || c.Symbol != s.cfg.Symbols[0] || !c.Close.IsPositive() {
		return nil
	}
	now := s.ctx.Now()
	if c.OpenTime.After(now) || now.Sub(c.OpenTime) > 2*time.Minute {
		return nil
	}
	state := s.state.Positions[c.Symbol]
	_, held := s.ctx.Position(s.cfg.Exchange, domain.Spot, c.Symbol)
	if state == nil || !held {
		return nil
	}
	state.Peak = decimal.Max(state.Peak, c.Close)
	i, ok := s.ctx.Instrument(s.cfg.Exchange, domain.Spot, c.Symbol)
	if !ok {
		return nil
	}
	state.Stop = decimal.Max(state.Stop, i.RoundPrice(state.Peak.Mul(decimal.NewFromInt(1).Sub(state.Trail))))
	if state.Closing || c.Close.LessThanOrEqual(state.Stop) {
		state.Closing = true
		return []strategy.Intent{{Kind: strategy.Exit, Symbol: c.Symbol, Reason: "catalyst trailing full exit"}}
	}
	return nil
}

func (s *Strategy) OnTimer(now time.Time) []strategy.Intent {
	symbol := s.cfg.Symbols[0]
	position := s.state.Positions[symbol]
	_, held := s.ctx.Position(s.cfg.Exchange, domain.Spot, symbol)
	if held && position == nil {
		return []strategy.Intent{{Kind: strategy.Exit, Symbol: symbol, Reason: "catalyst position missing thesis checkpoint"}}
	}
	if position != nil && held {
		if position.Closing || !now.Before(position.ReviewBefore) {
			position.Closing = true
			return []strategy.Intent{{Kind: strategy.Exit, Symbol: symbol, Reason: "catalyst thesis expired or closing"}}
		}
	}
	if position != nil && !held && !now.Before(position.EntryExpiresAt) {
		return []strategy.Intent{{Kind: strategy.Exit, Symbol: symbol, Reason: "catalyst pending entry expired"}}
	}
	f, err := os.Open(filepath.Join(s.params.Inbox, symbol+".json"))
	if err != nil {
		return nil
	}
	var input Input
	err = Decode(f, &input, MaxDocumentBytes*8)
	_ = f.Close()
	if err != nil || Hash(input.Policy) != s.params.PolicyHash || input.Review.PolicyHash != s.params.PolicyHash || input.Review.Analysis.Symbol != symbol {
		return nil
	}
	r := input.Review
	if held {
		if input.Event.Key != position.EventKey {
			return nil
		}
		if input.Event.Revoked {
			position.Closing = true
			return []strategy.Intent{{Kind: strategy.Exit, Symbol: symbol, Reason: "catalyst event revoked"}}
		}
		if !r.Valid || r.Error != "" || r.At.IsZero() || r.At.After(now) || !now.Before(r.ExpiresAt) || r.ExpiresAt.After(r.At.Add(time.Duration(input.Policy.DecisionTTL)*time.Second)) || !r.ReviewBefore.After(now) || r.ReviewBefore.After(r.At.Add(time.Duration(input.Policy.ReviewHours)*time.Hour)) || input.Policy.ValidateAnalysis(r.Analysis, input.Evidence, now) != nil || r.Analysis.EventKey != position.EventKey {
			return nil
		}
		if r.Analysis.Action == "PROPOSE_EXIT" {
			position.Closing = true
			return []strategy.Intent{{Kind: strategy.Exit, Symbol: symbol, Reason: "catalyst thesis invalidated"}}
		}
		if r.Analysis.Action == "HOLD" && r.ID != position.ReviewID && r.At.After(position.ReviewedAt) && input.Event.Verified && now.Before(input.Event.ValidUntil) && input.Event.Revision == Hash(r.Analysis) {
			if !r.Analysis.EventAt.After(now) {
				updated := false
				for _, e := range input.Evidence {
					for _, source := range input.Policy.Sources {
						if source.ID == e.SourceID && source.Primary && !e.FirstSeenAt.Before(r.Analysis.EventAt) {
							updated = true
						}
					}
				}
				if !updated {
					return nil
				}
			}
			position.ReviewID, position.ReviewBefore, position.ReviewedAt = r.ID, r.ReviewBefore, r.At
		}
		return nil
	}
	if s.state.UsedEvents[input.Event.Key] {
		return nil
	}
	ins, ok := s.ctx.Instrument(s.cfg.Exchange, domain.Spot, symbol)
	if !ok {
		return nil
	}
	input.Quote.Instrument = ins
	last, ok := s.ctx.LastPrice(s.cfg.Exchange, domain.Spot, symbol)
	if !ok || !last.IsPositive() || last.Sub(input.Quote.Bid).Abs().Div(last).GreaterThan(DevelopmentEntryPolicy().MaxDeviation) {
		return nil
	}
	account := s.ctx.Account(s.cfg.Exchange, domain.Spot)
	budget := Budget{Equity: account.Equity, FreeCash: account.FreeMargin()}
	intent, err := DevelopmentEntryPolicy().Entry(input.Policy, r, input.Evidence, input.Event, input.Quote, budget, now)
	if err != nil || !r.ReviewBefore.After(now) || r.ReviewBefore.After(r.At.Add(time.Duration(input.Policy.ReviewHours)*time.Hour)) {
		return nil
	}
	// Consuming before emitting is conservative: a rejected entry is not
	// retried blindly; the engine persists this state before broker I/O.
	s.state.UsedEvents[input.Event.Key] = true
	s.state.Positions[symbol] = &PositionState{EventKey: input.Event.Key, ReviewID: r.ID, Peak: input.Quote.Ask, Stop: intent.StopLoss, Trail: decimal.NewFromFloat(intent.TrailingStop), ReviewBefore: r.ReviewBefore, ReviewedAt: r.At, EntryExpiresAt: r.ExpiresAt}
	return []strategy.Intent{intent}
}

func (s *Strategy) SaveState() ([]byte, error) { return json.Marshal(s.state) }
func (s *Strategy) LoadState(raw []byte) error {
	if err := Decode(bytes.NewReader(raw), &s.state, MaxDocumentBytes); err != nil {
		return err
	}
	if s.state.Version != 1 || s.state.UsedEvents == nil || s.state.Positions == nil {
		return errors.New("invalid catalyst checkpoint")
	}
	for _, p := range s.state.Positions {
		if p == nil || !p.Stop.IsPositive() || !p.Peak.IsPositive() || p.Stop.GreaterThan(p.Peak) || !p.Trail.IsPositive() || !p.Trail.LessThan(decimal.NewFromInt(1)) || p.ReviewBefore.IsZero() || p.ReviewedAt.IsZero() || p.EntryExpiresAt.IsZero() || p.EventKey == "" {
			return errors.New("invalid catalyst position checkpoint")
		}
	}
	return nil
}
