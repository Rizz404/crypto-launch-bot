package catalyst

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/managed"
	"github.com/shopspring/decimal"
)

type ReplayStep struct {
	At      time.Time       `json:"at"`
	Symbol  string          `json:"symbol"`
	Price   decimal.Decimal `json:"price"`
	Input   *Input          `json:"input,omitempty"`
	Restart bool            `json:"restart,omitempty"`
}

// Replay is explicitly a synthetic fixture, with an isolated temporary inbox
// and the existing managed MemoryStore. It never opens a DB or network client.
type Replay struct {
	Policy      Policy              `json:"policy"`
	Instruments []domain.Instrument `json:"instruments"`
	Steps       []ReplayStep        `json:"steps"`
}

type ReplayResult struct {
	FixtureOnly bool                `json:"fixture_only"`
	PolicyHash  string              `json:"policy_hash"`
	Steps       int                 `json:"steps"`
	Executions  []managed.Execution `json:"executions"`
	Equity      decimal.Decimal     `json:"equity"`
	Cash        decimal.Decimal     `json:"cash"`
	Inventory   decimal.Decimal     `json:"inventory"`
	Reserved    decimal.Decimal     `json:"reserved"`
	Halted      bool                `json:"halted"`
	Attention   string              `json:"attention"`
}

func (f Replay) Run(ctx context.Context) (ReplayResult, error) {
	var result ReplayResult
	if len(f.Steps) == 0 || len(f.Steps) > 10000 {
		return result, errors.New("fixture needs 1–10000 steps")
	}
	inbox, err := os.MkdirTemp("", "catalyst-fixture-")
	if err != nil {
		return result, err
	}
	defer func() { _ = os.RemoveAll(inbox) }()
	spec, err := PaperSpec(f.Policy, inbox)
	if err != nil {
		return result, err
	}
	repo, err := managed.NewMemory(spec)
	if err != nil {
		return result, err
	}
	now := f.Steps[0].At
	runtime, err := NewPaper(ctx, repo, f.Policy, inbox, f.Instruments, func() time.Time { return now })
	if err != nil {
		return result, err
	}
	if err = runtime.Start(ctx); err != nil {
		return result, err
	}
	for n, step := range f.Steps {
		if step.At.IsZero() || (n > 0 && !step.At.After(now)) || !step.Price.IsPositive() || !slices.Contains(f.Policy.Symbols, step.Symbol) {
			return result, errors.New("invalid or unordered fixture step")
		}
		now = step.At
		if step.Input != nil {
			if Hash(step.Input.Policy) != Hash(f.Policy) || step.Input.Quote.Symbol != step.Symbol {
				return result, errors.New("fixture input policy/symbol mismatch")
			}
			data, err := json.Marshal(step.Input)
			if err != nil {
				return result, err
			}
			if err = os.WriteFile(filepath.Join(inbox, step.Symbol+".json"), data, 0600); err != nil {
				return result, err
			}
		}
		if step.Restart {
			if err = runtime.End(ctx, "stopped"); err != nil {
				return result, err
			}
			runtime, err = NewPaper(ctx, repo, f.Policy, inbox, f.Instruments, func() time.Time { return now })
			if err != nil {
				return result, err
			}
			if err = runtime.Start(ctx); err != nil {
				return result, err
			}
		}
		candle := domain.Candle{Exchange: "binance", Market: domain.Spot, Symbol: step.Symbol, Timeframe: "1m", OpenTime: now.Truncate(time.Minute), Open: step.Price, High: step.Price, Low: step.Price, Close: step.Price}
		if err = runtime.Record(ctx, candle, false); err != nil {
			return result, err
		}
		if err = runtime.Tick(ctx, now); err != nil {
			return result, err
		}
	}
	if err = runtime.End(ctx, "stopped"); err != nil {
		return result, err
	}
	state, err := repo.Load(ctx)
	if err != nil {
		return result, err
	}
	reserved, _ := state.Reserved("", "", "")
	return ReplayResult{FixtureOnly: true, PolicyHash: Hash(f.Policy), Steps: len(f.Steps), Executions: state.Executions, Equity: state.Equity(), Cash: state.Instances["research"].Cash, Inventory: state.Inventory(""), Reserved: reserved, Halted: state.Halted, Attention: state.Attention}, nil
}
