package managed

import (
	"context"
	"errors"
	"fmt"
	"github.com/rizz404/trading-bot/internal/risk"
	"github.com/rizz404/trading-bot/internal/strategy"
	"github.com/shopspring/decimal"
	"slices"
	"time"
)

type Command struct {
	Risk      *risk.Config    `json:"risk,omitempty"`
	ID        string          `json:"id"`
	Operation string          `json:"operation"`
	Scope     string          `json:"scope"`
	From      string          `json:"from"`
	To        string          `json:"to"`
	Amount    decimal.Decimal `json:"amount"`
	Name      string          `json:"name"`
	Policy    string          `json:"policy"`
	Instance  *InstanceSpec   `json:"instance,omitempty"`
	Binding   *BindingSpec    `json:"binding,omitempty"`
	Confirm   bool            `json:"confirm"`
}

func (s *State) owners(scope string) ([]string, error) {
	var out []string
	for _, id := range sortedBindings(s) {
		b := s.Bindings[id]
		if scope == "account" || scope == b.InstanceID || scope == id {
			out = append(out, id)
		}
	}
	if len(out) == 0 && scope != "account" && s.Instances[scope] == nil {
		return nil, errors.New("unknown scope")
	}
	return out, nil
}
func (s *State) flat(scope string) bool {
	owners, err := s.owners(scope)
	if err != nil {
		return false
	}
	for _, owner := range owners {
		for _, p := range s.Holdings {
			if p.Owner == owner && p.Quantity.IsPositive() {
				return false
			}
		}
		for _, o := range s.Orders {
			if o.Owner == owner && o.State != "terminal" {
				return false
			}
		}
	}
	return true
}
func (s *State) Apply(c Command) error {
	if c.Operation == "transfer" {
		return s.Transfer(c.From, c.To, c.Amount)
	}
	if c.Operation == "create" || c.Operation == "clone" {
		if c.Instance == nil {
			return errors.New("instance definition required")
		}
		spec := *c.Instance
		if c.Operation == "clone" {
			source := s.Instances[c.Scope]
			if source == nil {
				return errors.New("unknown template source")
			}
			spec.Strategies = nil
			for _, id := range sortedBindings(s) {
				b := s.Bindings[id]
				if b.InstanceID == source.ID {
					spec.Strategies = append(spec.Strategies, b.Spec)
				}
			}
			spec.Risk = source.Risk
		}
		if s.Instances[spec.ID] != nil {
			return errors.New("instance id exists")
		}
		spec.Allocation = decimal.Zero
		test := s.Spec
		test.Instances = []InstanceSpec{spec}
		if err := test.Validate(); err != nil {
			return err
		}
		for _, b := range spec.Strategies {
			st, err := strategy.New(b.Name)
			if err != nil {
				return err
			}
			if err = st.Init(nil, b.Config(s.Spec.Exchange, spec.ID+"/"+b.ID)); err != nil {
				return err
			}
		}
		s.Instances[spec.ID] = &Instance{ID: spec.ID, Name: spec.Name, Status: "paused", Notify: spec.Notify, Risk: spec.Risk}
		for _, b := range spec.Strategies {
			id := spec.ID + "/" + b.ID
			s.Bindings[id] = &Binding{ID: id, InstanceID: spec.ID, Status: "paused", Reason: "restart runtime to initialize subscriptions", Spec: b}
		}
		s.Note(c.Operation, spec.ID, "configuration copied; no capital or positions copied")
		return nil
	}

	if c.Operation == "bind" || c.Operation == "edit_binding" {
		if c.Binding == nil {
			return errors.New("strategy definition required")
		}
		instance := s.Instances[c.Scope]
		if instance == nil || instance.Status != "paused" {
			return errors.New("pause the instance before editing strategies")
		}
		spec := *c.Binding
		check := s.Spec
		check.Instances = []InstanceSpec{{ID: instance.ID, Strategies: []BindingSpec{spec}}}
		if err := check.Validate(); err != nil {
			return err
		}
		id := instance.ID + "/" + spec.ID
		old := s.Bindings[id]
		if old != nil {
			if c.Operation != "edit_binding" || !s.flat(id) {
				return errors.New("strategy exists or still has orders or inventory")
			}
		}
		st, err := strategy.New(spec.Name)
		if err != nil {
			return err
		}
		if err = st.Init(nil, spec.Config(s.Spec.Exchange, id)); err != nil {
			return err
		}
		s.Bindings[id] = &Binding{ID: id, InstanceID: instance.ID, Spec: spec, Status: "paused", Reason: "restart runtime to activate revised strategy"}
		s.Note(c.Operation, id, "strategy revision saved")
		return nil
	}
	owners, err := s.owners(c.Scope)
	if err != nil {
		return err
	}
	switch c.Operation {
	case "limits":
		if !c.Confirm {
			return errors.New("confirm risk or budget changes")
		}
		if c.Amount.IsNegative() {
			return errors.New("negative strategy cap")
		}
		if c.Risk != nil {
			if err := validateRisk(*c.Risk); err != nil {
				return err
			}
		}
		if c.Scope == "account" {
			if c.Risk == nil {
				return errors.New("risk settings required")
			}
			s.Spec.Risk = *c.Risk
		} else if i := s.Instances[c.Scope]; i != nil {
			if c.Risk == nil {
				return errors.New("risk settings required")
			}
			i.Risk = *c.Risk
		} else {
			b := s.Bindings[c.Scope]
			before := s.bindingCapital(b)
			b.Spec.Cap = c.Amount
			s.rebaseBindings(b.InstanceID, func(other *Binding) decimal.Decimal {
				if other != b {
					return decimal.Zero
				}
				return s.bindingCapital(b).Sub(before)
			})
			if c.Risk != nil {
				b.Spec.Risk = *c.Risk
			}
		}
	case "rename":
		i := s.Instances[c.Scope]
		if i == nil || len(c.Name) == 0 || len(c.Name) > 100 {
			return errors.New("instance name required")
		}
		i.Name = c.Name
	case "notify":
		i := s.Instances[c.Scope]
		if i == nil || !slices.Contains([]string{"info", "warn", "error", "off"}, c.Name) {
			return errors.New("invalid notification preference")
		}
		i.Notify = c.Name
	case "pause", "resume", "flatten", "finish", "kill", "reset", "archive", "cancel":
		destructive := slices.Contains([]string{"flatten", "finish", "kill", "reset", "archive", "cancel"}, c.Operation)
		if destructive && !c.Confirm {
			return errors.New("explicit confirmation required")
		}
		if c.Operation == "finish" && !slices.Contains([]string{"retain", "flatten"}, c.Policy) {
			return errors.New("choose retain or flatten inventory")
		}
		if c.Operation == "archive" {
			for id, i := range s.Instances {
				if (c.Scope == "account" || c.Scope == id) && !i.Cash.IsZero() {
					return errors.New("transfer instance cash to reserve before archiving")
				}
			}
		}
		if c.Operation == "archive" && !s.flat(c.Scope) {
			return errors.New("cannot archive open positions or orders, including dust")
		}
		if c.Operation == "resume" {
			if s.Attention != "" || s.Halted {
				return ErrAttention
			}
			for _, id := range owners {
				b := s.Bindings[id]
				i := s.Instances[b.InstanceID]
				if b.Status == "archived" && c.Scope == "account" {
					continue
				}
				if b.Halted || i.Halted || b.Status == "archived" || i.Status == "archived" {
					return errors.New("scope halted or archived; reset explicitly")
				}
			}
		}
		status := "paused"
		switch c.Operation {
		case "resume", "reset":
			status = "running"
		case "archive":
			status = "archived"
		case "flatten", "finish", "kill":
			status = "closing"
		case "cancel":
			status = "paused"
		}
		if c.Operation == "reset" && s.Attention != "" {
			return ErrAttention
		}
		if c.Scope == "account" && c.Operation == "kill" {
			s.Halted = true
			s.Reason = "manual account halt"
		}
		if c.Scope == "account" && c.Operation == "reset" {
			s.Halted = false
			s.Reason = ""
			s.Peak = s.Equity()
			s.DayStart = s.Peak
			s.Day = time.Now().UTC().Truncate(24 * time.Hour)
		}
		for id, i := range s.Instances {
			if (c.Scope == "account" || c.Scope == id) && i.Status != "archived" {
				i.Status = status
			}
		}
		for _, id := range owners {
			b := s.Bindings[id]
			if b.Status == "archived" {
				continue
			}
			i := s.Instances[b.InstanceID]
			b.Status = status
			if c.Scope == "account" || c.Scope == i.ID {
				i.Status = status
			}
			if c.Operation == "kill" {
				b.Halted = true
				b.Reason = "manual scoped halt"
				if c.Scope == "account" || c.Scope == i.ID {
					i.Halted = true
				}
			}
			if c.Operation == "reset" {
				b.Halted = false
				b.Reason = ""
				b.Peak = decimal.Zero
				b.DayStart = decimal.Zero
				b.Day = time.Time{}
				if c.Scope == "account" || c.Scope == i.ID {
					i.Halted = false
					i.Peak = s.InstanceEquity(i.ID)
					i.DayStart = i.Peak
				}
			}
			if c.Operation == "finish" {
				b.Reason = "finish:" + c.Policy
			}
		}
	default:
		return fmt.Errorf("unknown operation %q", c.Operation)
	}
	s.Note(c.Operation, c.Scope, "control applied")
	return nil
}
func (r *Runtime) Command(ctx context.Context, c Command) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c.Operation == "resume" || c.Operation == "reset" {
		st, err := r.repo.Load(ctx)
		if err != nil {
			return err
		}
		owners, err := st.owners(c.Scope)
		if err != nil {
			return err
		}
		for _, id := range owners {
			if r.engines[id] == nil || Hash(st.Bindings[id].Spec.Config(r.physical.Exchange(), id)) != r.revisions[id] {
				return errors.New("restart runtime before activating a newly created strategy")
			}
		}
	}
	if err := r.repo.Update(ctx, c.ID, "control", c, func(s *State) error { return s.Apply(c) }); err != nil {
		return err
	}
	s, err := r.repo.Load(ctx)
	if err != nil {
		return err
	}
	owners, err := s.owners(c.Scope)
	if err != nil && c.Operation != "transfer" && c.Operation != "create" && c.Operation != "clone" {
		return err
	}
	for _, owner := range owners {
		e := r.engines[owner]
		if e == nil {
			continue
		}
		if err = r.syncView(ctx, owner); err != nil {
			return err
		}
		switch c.Operation {
		case "limits":
			rc := effectiveRisk(s.Bindings[owner].Spec.Risk)
			rc.DailyLossLimit = 0
			rc.MaxDrawdown = 0
			rc.MaxLeverage = 1
			e.SetRiskConfig(rc)
		case "flatten", "kill":
			e.CloseAll(ctx, c.Operation)
		case "finish":
			if c.Policy == "flatten" {
				e.CloseAll(ctx, "finish experiment")
			} else {
				e.CancelAllOrders(ctx)
			}
		case "cancel":
			e.CancelPending(ctx)
		}
		if err = r.save(ctx, owner); err != nil {
			return err
		}
	}
	if err = r.drain(ctx); err != nil {
		return err
	}
	return r.finalize(ctx)
}
func (r *Runtime) finalize(ctx context.Context) error {
	return r.repo.Update(ctx, nonce(), "settle_controls", nil, func(s *State) error {
		for owner, b := range s.Bindings {
			if b.Status != "closing" {
				continue
			}
			orders := false
			for _, o := range s.Orders {
				if o.Owner == owner && o.State != "terminal" {
					orders = true
				}
			}
			if !orders && (s.positionFlat(owner) || b.Reason == "finish:retain") {
				b.Status = "stopped"
				b.Reason = "terminal exchange orders confirmed"
			}
		}
		for _, i := range s.Instances {
			if i.Status != "closing" {
				continue
			}
			done := true
			for _, b := range s.Bindings {
				if b.InstanceID == i.ID && b.Status == "closing" {
					done = false
				}
			}
			if done {
				i.Status = "stopped"
			}
		}
		return nil
	})
}

func (s *State) positionFlat(owner string) bool {
	for _, p := range s.Holdings {
		if p.Owner == owner && p.Quantity.IsPositive() {
			return false
		}
	}
	return true
}
