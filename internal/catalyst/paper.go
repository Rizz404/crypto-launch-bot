package catalyst

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/managed"
	"github.com/rizz404/trading-bot/internal/risk"
	"github.com/shopspring/decimal"
)

// PaperSpec is a development contract, never an update to a P2 wallet.
func PaperSpec(policy Policy, inbox string) (managed.Spec, error) {
	if err := policy.Validate(); err != nil {
		return managed.Spec{}, err
	}
	if !filepath.IsAbs(inbox) {
		return managed.Spec{}, errors.New("isolated absolute inbox required")
	}
	rc := risk.Config{RiskPerTrade: 0.005, PositionPct: 0.20, MaxPositionPct: 0.20, MaxExposurePct: 0.40, MaxOpenPositions: 2, MaxLeverage: 1, DailyLossLimit: 0.02, MaxDrawdown: 0.08, CloseOnKill: true}
	spec := managed.Spec{ID: "catalyst-" + policy.Version, Exchange: "binance", Mode: "paper", Quote: "USDT", Capital: decimal.NewFromInt(1000), Risk: rc}
	instance := managed.InstanceSpec{ID: "research", Name: "Catalyst paper development", Allocation: spec.Capital, Risk: rc, Notify: "info"}
	for _, symbol := range policy.Symbols {
		cap := decimal.NewFromInt(200)
		if slices.Contains(policy.MemeSymbols, symbol) {
			cap = decimal.NewFromInt(20)
		}
		instance.Strategies = append(instance.Strategies, managed.BindingSpec{ID: strings.ToLower(symbol), Name: "catalyst", Symbols: []string{symbol}, Cap: cap, Risk: rc, Params: map[string]any{"inbox": inbox, "policy_hash": Hash(policy), "execution_hash": Hash(DevelopmentEntryPolicy())}})
	}
	spec.Instances = []managed.InstanceSpec{instance}
	return spec, spec.Validate()
}

// NewPaper always constructs a Sim broker. There is no live-broker parameter,
// credential, order API client or mode switch in the catalyst execution path.
func NewPaper(ctx context.Context, repo managed.Repository, policy Policy, inbox string, instruments []domain.Instrument, clock func() time.Time) (*managed.Runtime, error) {
	want, err := PaperSpec(policy, inbox)
	if err != nil {
		return nil, err
	}
	state, err := repo.Load(ctx)
	if err != nil {
		return nil, err
	}
	if managed.Hash(want) != state.ConfigHash || state.Spec.Mode != "paper" || !strings.HasPrefix(state.Spec.ID, "catalyst-") {
		return nil, errors.New("catalyst requires its own matching paper wallet; P2 state rejected")
	}
	for _, sym := range policy.Symbols {
		if !slices.ContainsFunc(instruments, func(i domain.Instrument) bool {
			return i.Exchange == "binance" && i.Market == domain.Spot && i.Symbol == sym && i.Tradable && i.Quote == "USDT"
		}) {
			return nil, errors.New("missing tradable Spot instrument")
		}
	}
	sim := broker.NewSim(broker.SimConfig{Exchange: "binance", Fees: map[domain.Market]broker.Fees{domain.Spot: {Maker: 0.001, Taker: 0.001}}, SlippageBps: 2, Clock: clock})
	return managed.NewRuntime(ctx, repo, sim, instruments, decimal.RequireFromString("0.001"), clock, nil)
}
