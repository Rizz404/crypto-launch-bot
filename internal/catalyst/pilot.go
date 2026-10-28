package catalyst

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/rizz404/trading-bot/internal/domain"
	"github.com/rizz404/trading-bot/internal/engine"
	"github.com/rizz404/trading-bot/internal/managed"
	"github.com/shopspring/decimal"
)

type PilotStatus struct {
	StartedAt, TickAt, ResearchAt             time.Time
	Prices                                    map[string]time.Time
	SourceGaps                                []string
	DecisionID, Action, Symbol, Reason, Error string
}

// Pilot separates research latency from the quote/protection and timer loops.
// Runtime is always constructed by NewPaper; it has no exchange order client.
type Pilot struct {
	Policy     Policy
	Archive    Archive
	Collector  *Collector
	Market     *Market
	Analyst    *Analyst
	Runtime    *managed.Runtime
	Inbox      string
	CheckLease func(context.Context) error
	Notify     func(string)
	Log        *slog.Logger
	mu         sync.Mutex
	status     PilotStatus
}

func (p *Pilot) Status() PilotStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	s := p.status
	s.Prices, s.SourceGaps = maps.Clone(s.Prices), slices.Clone(s.SourceGaps)
	return s
}

func (s PilotStatus) Healthy(p Policy, now time.Time) bool {
	if s.TickAt.IsZero() || s.TickAt.After(now) || now.Sub(s.TickAt) > 10*time.Second {
		return false
	}
	for _, symbol := range p.Symbols {
		at := s.Prices[symbol]
		if at.IsZero() || at.After(now) || now.Sub(at) > 15*time.Second {
			return false
		}
	}
	return true
}

func (p *Pilot) Run(ctx context.Context) error {
	if err := p.Policy.Validate(); err != nil {
		return err
	}
	if p.Collector == nil || p.Market == nil || p.Analyst == nil || p.Runtime == nil || p.CheckLease == nil || !filepath.IsAbs(p.Inbox) {
		return errors.New("pilot dependencies, lease and absolute inbox required")
	}
	if p.Log == nil {
		p.Log = slog.Default()
	}
	p.mu.Lock()
	p.status = PilotStatus{StartedAt: time.Now().UTC(), Prices: map[string]time.Time{}}
	p.mu.Unlock()
	if err := os.MkdirAll(p.Inbox, 0700); err != nil {
		return err
	}
	if err := p.Runtime.Start(ctx); err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	failures := make(chan error, 3)
	start := func(work func(context.Context) error) {
		wg.Go(func() {
			if err := work(runCtx); err != nil && runCtx.Err() == nil {
				select {
				case failures <- err:
				default:
				}
			}
		})
	}
	start(p.feed)
	start(p.timers)
	start(p.research)
	var err error
	select {
	case <-ctx.Done():
	case err = <-failures:
	}
	cancel()
	wg.Wait()
	endCtx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	status := "stopped"
	if err != nil {
		status = "interrupted"
	}
	endErr := p.Runtime.End(endCtx, status)
	return errors.Join(err, endErr)
}

func (p *Pilot) timers(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case at := <-ticker.C:
			stepCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := p.CheckLease(stepCtx)
			if err == nil {
				err = p.Runtime.Tick(stepCtx, at.UTC())
			}
			cancel()
			if err != nil {
				return errors.New("pilot timer/storage/ownership failed; entry stopped")
			}
			p.mu.Lock()
			p.status.TickAt = at.UTC()
			p.mu.Unlock()
			if at.Unix()%5 == 0 {
				if err = persistJSON(p.Archive.Dir, "health.json", p.Status()); err != nil {
					return errors.New("pilot health persistence failed")
				}
			}
		}
	}
}

func (p *Pilot) feed(ctx context.Context) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		for _, symbol := range p.Policy.Symbols {
			if ctx.Err() != nil {
				return nil
			}
			quoteCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			book, err := p.Market.Client.SpotQuote(quoteCtx, symbol)
			if err == nil {
				err = p.CheckLease(quoteCtx)
				if err != nil {
					cancel()
					return errors.New("pilot quote ownership lost")
				}
			}
			cancel()
			if err != nil {
				p.Log.Warn("pilot quote unavailable", "symbol", symbol)
				continue
			}
			writeCtx, done := context.WithTimeout(ctx, 5*time.Second)
			err = p.Runtime.RecordBook(writeCtx, domain.BookQuote{Exchange: "binance", Market: domain.Spot, Symbol: symbol, Bid: book.Bid, Ask: book.Ask, BidQty: book.BidQty, AskQty: book.AskQty, At: book.At})
			done()
			if err != nil {
				return errors.New("pilot book settlement/storage failed")
			}
			p.mu.Lock()
			p.status.Prices[symbol] = book.At
			p.mu.Unlock()
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (p *Pilot) research(ctx context.Context) error {
	ticker := time.NewTicker(time.Duration(p.Policy.PollSeconds) * time.Second)
	defer ticker.Stop()
	for {
		cycleCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
		err := p.Cycle(cycleCtx)
		cancel()
		if err != nil && ctx.Err() == nil {
			p.mu.Lock()
			p.status.Error = err.Error()
			p.mu.Unlock()
			p.Log.Warn("pilot research paused", "reason", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (p *Pilot) Cycle(ctx context.Context) error {
	failed := map[string]bool{}
	for _, source := range p.Policy.Sources {
		if _, err := p.Collector.Collect(ctx, source); err != nil {
			failed[source.ID] = true
		}
	}
	evidence, gaps, err := p.Archive.AvailableEvidence(p.Policy)
	if err != nil {
		return err
	}
	for source := range failed {
		if !slices.Contains(gaps, source) {
			return errors.New("source failure could not be recorded; research stopped")
		}
	}
	p.mu.Lock()
	p.status.SourceGaps = gaps
	p.status.ResearchAt = time.Now().UTC()
	p.mu.Unlock()
	if len(evidence) == 0 {
		return errors.New("no current source evidence; no model call or entry")
	}
	state, err := p.Runtime.Snapshot(ctx)
	if err != nil {
		return errors.New("cannot read pilot ledger")
	}
	research := ResearchContext{SourceGaps: gaps}
	used := map[string]bool{}
	for _, binding := range state.Bindings {
		if len(binding.Engine) == 0 {
			continue
		}
		var checkpoint engine.Checkpoint
		if json.Unmarshal(binding.Engine, &checkpoint) != nil {
			return errors.New("cannot read event history checkpoint")
		}
		for _, raw := range checkpoint.Strategies {
			var strategy Strategy
			if strategy.LoadState(raw) != nil {
				return errors.New("cannot validate event history")
			}
			for key := range strategy.state.UsedEvents {
				used[key] = true
			}
		}
	}
	for key := range used {
		research.ConsumedEvents = append(research.ConsumedEvents, key)
	}
	slices.Sort(research.ConsumedEvents)
	for _, symbol := range p.Policy.Symbols {
		q, err := p.Market.Snapshot(ctx, p.Policy, symbol)
		if err != nil {
			book, bookErr := p.Market.Client.SpotQuote(ctx, symbol)
			if bookErr != nil {
				continue
			}
			q = Quote{Symbol: symbol, Instrument: p.Market.instruments[symbol], Bid: book.Bid, Ask: book.Ask, BidQty: book.BidQty, AskQty: book.AskQty, At: book.At, QuoteVolume: p.Market.volume[symbol]}
			research.SourceGaps = append(research.SourceGaps, "ATR:"+symbol)
		}
		research.Market = append(research.Market, q)
	}
	for _, holding := range state.Holdings {
		if !holding.Quantity.IsPositive() {
			continue
		}
		if time.Since(state.PriceAt[holding.Symbol]) > 10*time.Second {
			return errors.New("held asset feed stale; new research entry blocked")
		}
		input, err := readInput(p.Inbox, holding.Symbol)
		if err != nil {
			return errors.New("held position thesis inbox unavailable; no new analysis")
		}
		research.Positions = append(research.Positions, PositionContext{Symbol: holding.Symbol, Quantity: holding.Quantity.String(), EventKey: input.Event.Key, Thesis: input.Review.Analysis.Thesis, Invalidation: input.Review.Analysis.Invalidation})
	}
	slices.SortFunc(research.Positions, func(a, b PositionContext) int { return strings.Compare(a.Symbol, b.Symbol) })
	review, err := p.Analyst.AnalyzeMarket(ctx, p.Archive, p.Policy, evidence, time.Now().UTC(), research)
	if errors.Is(err, ErrDuplicateInference) {
		return nil
	}
	p.mu.Lock()
	p.status.DecisionID, p.status.Action, p.status.Symbol, p.status.Reason, p.status.Error = review.ID, review.Analysis.Action, review.Analysis.Symbol, review.Analysis.Reason, review.Error
	p.mu.Unlock()
	if err != nil {
		return err
	}
	submitErr := p.Submit(ctx, review, evidence)
	message := "Lihat ledger untuk status order/fill."
	if submitErr != nil {
		message = "Ditahan: " + submitErr.Error()
		p.mu.Lock()
		p.status.Error = submitErr.Error()
		p.mu.Unlock()
	}
	_, auditErr := p.Archive.Put("outcomes", map[string]any{"review_id": review.ID, "policy_hash": Hash(p.Policy), "action": review.Analysis.Action, "symbol": review.Analysis.Symbol, "at": time.Now().UTC(), "message": message})
	if p.Notify != nil {
		p.Notify(fmt.Sprintf("Usulan AI • P3 paper • %s %s\n%s\n%s", review.Analysis.Action, review.Analysis.Symbol, review.Analysis.Reason, message))
	}
	return errors.Join(submitErr, auditErr)
}

// Submit re-fetches execution prices after inference. AI market context cannot
// become an executable price or a trusted event attestation by itself.
func (p *Pilot) Submit(ctx context.Context, review Review, evidence []Evidence) error {
	a := review.Analysis
	if a.Action != "PROPOSE_ENTRY" && a.Action != "HOLD" && a.Action != "PROPOSE_EXIT" {
		return nil
	}
	if !review.Valid || review.PolicyHash != Hash(p.Policy) || !time.Now().Before(review.ExpiresAt) {
		return errors.New("invalid or expired pilot review")
	}
	if p.CheckLease == nil || p.CheckLease(ctx) != nil {
		return errors.New("pilot ownership required before submission")
	}
	state, err := p.Runtime.Snapshot(ctx)
	if err != nil {
		return errors.New("pilot ledger unavailable")
	}
	if state.Halted || state.Attention != "" {
		return errors.New("pilot account halted or needs attention")
	}
	var previous Input
	held := false
	for _, h := range state.Holdings {
		held = held || (h.Symbol == a.Symbol && h.Quantity.IsPositive())
	}
	if held {
		previous, err = readInput(p.Inbox, a.Symbol)
		if err != nil || a.EventKey != previous.Event.Key {
			return errors.New("review does not match the held event")
		}
		if a.Action == "PROPOSE_ENTRY" {
			for _, h := range state.Holdings {
				if h.Quantity.IsPositive() && time.Since(state.PriceAt[h.Symbol]) > 10*time.Second {
					return errors.New("held asset mark stale; new entry blocked")
				}
			}
			return errors.New("entry for an already held symbol rejected")
		}
		if a.Action == "HOLD" && (a.EventType != previous.Review.Analysis.EventType || !a.EventAt.Equal(previous.Review.Analysis.EventAt)) {
			return errors.New("HOLD cannot replace the original event schedule or type")
		}
		if a.Action == "HOLD" && (a.Thesis != previous.Review.Analysis.Thesis || a.Invalidation != previous.Review.Analysis.Invalidation) {
			return errors.New("HOLD cannot replace the original thesis or invalidation")
		}
	} else if a.Action != "PROPOSE_ENTRY" {
		return errors.New("hold/exit requires an existing position")
	}
	event := previous.Event
	if a.Action != "PROPOSE_EXIT" {
		event, err = p.Archive.VerifyEvent(p.Policy, a, evidence, time.Now().UTC())
		if err != nil {
			return err
		}
	}
	var q Quote
	if a.Action == "PROPOSE_ENTRY" {
		q, err = p.Market.Snapshot(ctx, p.Policy, a.Symbol)
	} else {
		book, bookErr := p.Market.Client.SpotQuote(ctx, a.Symbol)
		err = bookErr
		q = Quote{Symbol: a.Symbol, Instrument: p.Market.instruments[a.Symbol], Bid: book.Bid, Ask: book.Ask, BidQty: book.BidQty, AskQty: book.AskQty, At: book.At}
	}
	if err != nil {
		return err
	}
	if a.Action == "PROPOSE_ENTRY" {
		owner := "research/" + strings.ToLower(a.Symbol)
		binding := state.Bindings[owner]
		instance := state.Instances["research"]
		if binding == nil || instance == nil || binding.Status != "running" || instance.Status != "running" || binding.Halted || instance.Halted {
			return errors.New("pilot entry is paused or halted")
		}
		budget := Budget{Equity: state.InstanceEquity("research"), FreeCash: state.FreeCash("research"), Exposure: state.Inventory("")}
		positions := map[string]bool{}
		for _, h := range state.Holdings {
			if h.Quantity.IsPositive() {
				positions[h.Owner+":"+h.Symbol] = true
				if slices.Contains(p.Policy.MemeSymbols, h.Symbol) {
					budget.MemeExposure = budget.MemeExposure.Add(h.Quantity.Mul(state.Prices[h.Symbol]))
				}
			}
		}
		for _, o := range state.Orders {
			if o.State != "terminal" && o.Order.Side == domain.Buy {
				positions[o.Owner+":"+o.Order.Symbol] = true
				budget.Exposure = budget.Exposure.Add(o.QuoteReserved)
				if slices.Contains(p.Policy.MemeSymbols, o.Order.Symbol) {
					budget.MemeExposure = budget.MemeExposure.Add(o.QuoteReserved)
				}
			}
		}
		budget.Positions = len(positions)
		intent, gateErr := DevelopmentEntryPolicy().Entry(p.Policy, review, evidence, event, q, budget, time.Now().UTC())
		if gateErr != nil {
			return gateErr
		}
		if gateErr = state.CheckExposure(owner, a.Symbol, intent.Quantity.Mul(q.Ask).Mul(decimal.RequireFromString("1.0012"))); gateErr != nil {
			return gateErr
		}
	}
	input := Input{Policy: p.Policy, Review: review, Evidence: evidence, Event: event, Quote: q}
	if _, err = p.Archive.Put("execution-inputs", input); err != nil {
		return err
	}
	if err = writePilotInput(p.Inbox, a.Symbol, input); err != nil {
		return err
	}
	if err = p.Runtime.RecordBook(ctx, domain.BookQuote{Exchange: "binance", Market: domain.Spot, Symbol: a.Symbol, Bid: q.Bid, Ask: q.Ask, BidQty: q.BidQty, AskQty: q.AskQty, At: q.At}); err != nil {
		return err
	}
	return p.Runtime.Tick(ctx, time.Now().UTC())
}

func readInput(dir, symbol string) (Input, error) {
	var input Input
	if !symbolPattern.MatchString(symbol) {
		return input, errors.New("invalid inbox symbol")
	}
	f, err := os.Open(filepath.Join(dir, symbol+".json"))
	if err != nil {
		return input, err
	}
	defer func() { _ = f.Close() }()
	err = Decode(f, &input, maxArchiveBytes)
	return input, err
}

func writePilotInput(dir, symbol string, input Input) error {
	if !symbolPattern.MatchString(symbol) || input.Quote.Symbol != symbol {
		return errors.New("input symbol mismatch")
	}
	return persistJSON(dir, symbol+".json", input)
}

func persistJSON(dir, name string, value any) error {
	if filepath.Base(name) != name || name == "." || name == ".." {
		return errors.New("invalid JSON filename")
	}
	data, err := json.Marshal(value)
	if err != nil || len(data) > maxArchiveBytes {
		return errors.New("input exceeds size limit")
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".pending-")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(f.Name(), filepath.Join(dir, name)); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()
	return d.Sync()
}
