package launch

import (
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/rizz404/trading-bot/internal/catalyst"
	"github.com/shopspring/decimal"
)

func newPaperState(run string, p PaperPolicy) PaperState {
	return PaperState{Version: PaperJournalVersion, RunID: run, FixtureOnly: true, PolicyHash: catalyst.Hash(p), CashUSDT: p.InitialUSDT,
		PeakUSDT: p.InitialUSDT, DayStartUSDT: p.InitialUSDT, Orders: map[string]PaperOrder{}, Positions: map[string]PaperPosition{},
		Requests: map[string]string{}, Receipts: map[string]string{}, Consumed: map[string]bool{}, QuoteUsage: map[string]decimal.Decimal{}}
}

func orderOpen(o PaperOrder) bool {
	return !slices.Contains([]string{"filled", "canceled", "failed", "rejected"}, o.Status)
}
func positionOpen(p PaperPosition) bool {
	return p.Quantity.IsPositive() || p.CostUSDT.IsPositive() || p.LockedUSDT.IsPositive()
}
func (s PaperState) reserved(position string) decimal.Decimal {
	amount := decimal.Zero
	for _, o := range s.Orders {
		if orderOpen(o) && (position == "" || o.Intent.PositionID == position) {
			amount = amount.Add(o.ReserveUSDT)
		}
	}
	return amount
}
func (s PaperState) pending(position string) bool {
	for _, o := range s.Orders {
		if orderOpen(o) && o.Intent.PositionID == position {
			return true
		}
	}
	return false
}
func (s PaperState) uncertain() bool {
	if s.Attention {
		return true
	}
	for _, o := range s.Orders {
		if o.Status == "unknown" {
			return true
		}
	}
	for _, p := range s.Positions {
		if p.ExitRequired && positionOpen(p) {
			return true
		}
	}
	return false
}

// Context distinguishes cost-at-risk from a fresh mark. Unpriced/locked
// inventory has conservative mark zero and blocks new exposure, not exits.
func (s PaperState) Context(now time.Time) PaperContext {
	c := PaperContext{At: now.UTC(), DayWIB: now.In(time.FixedZone("WIB", 7*60*60)).Format("2006-01-02"),
		AvailableUSDT: s.CashUSDT, EquityUSDT: s.CashUSDT.Add(s.reserved("")), DayStartUSDT: s.DayStartUSDT, PeakUSDT: s.PeakUSDT}
	if s.DayWIB != "" {
		c.DayWIB = s.DayWIB
	}
	seen := map[string]bool{}
	for id, p := range s.Positions {
		reserve := s.reserved(id)
		if !positionOpen(p) && !reserve.IsPositive() {
			continue
		}
		known := !p.LockedUSDT.IsPositive()
		mark := decimal.Zero
		if p.Quantity.IsPositive() {
			fresh := !p.MarkAt.IsZero() && !p.MarkAt.After(now) && now.Sub(p.MarkAt) <= 5*time.Second && p.MarkPrice.IsPositive()
			known = known && fresh
			if fresh {
				mark = p.Quantity.Mul(p.MarkPrice)
			}
		}
		c.EquityUSDT = c.EquityUSDT.Add(mark)
		exposure := PaperExposure{ID: id, CandidateID: p.Candidate.ID, AssetKey: p.Candidate.Asset.Key(), Kind: p.Candidate.Kind,
			NotionalUSDT: p.CostUSDT.Add(p.LockedUSDT).Add(reserve), LockedUSDT: p.LockedUSDT, ReservedUSDT: reserve,
			MarkUSDT: decimalPointer(mark.Add(reserve)), MarkKnown: known, Tradable: p.Quantity.IsPositive() && !s.pending(id),
			Claimable: p.UnlockedQuantity.GreaterThan(p.ClaimedQuantity), Refundable: p.RefundableUSDT.GreaterThan(p.RefundedUSDT), Thesis: p.Thesis, Invalidation: p.Invalidation}
		c.Positions = append(c.Positions, exposure)
		seen[id] = true
	}
	for _, o := range s.Orders {
		if !orderOpen(o) || seen[o.Intent.PositionID] {
			continue
		}
		amount := s.reserved(o.Intent.PositionID)
		c.Positions = append(c.Positions, PaperExposure{ID: o.Intent.PositionID, CandidateID: o.Intent.Candidate.ID, AssetKey: o.Intent.Candidate.Asset.Key(),
			Kind: o.Intent.Candidate.Kind, NotionalUSDT: amount, ReservedUSDT: amount, MarkUSDT: decimalPointer(amount), MarkKnown: true})
		seen[o.Intent.PositionID] = true
	}
	for id, consumed := range s.Consumed {
		if consumed {
			c.ConsumedCandidates = append(c.ConsumedCandidates, id)
		}
	}
	slices.Sort(c.ConsumedCandidates)
	slices.SortFunc(c.Positions, func(a, b PaperExposure) int { return strings.Compare(a.ID, b.ID) })
	return c
}
func decimalPointer(d decimal.Decimal) *decimal.Decimal { return &d }

func (s *PaperState) apply(step PaperStep, p PaperPolicy) error {
	if step.ID == "" || step.At.IsZero() || (!s.At.IsZero() && step.At.Before(s.At)) {
		return errors.New("paper step ID/time missing or time regressed")
	}
	if err := paperStepShape(step); err != nil {
		return err
	}
	day := step.At.In(time.FixedZone("WIB", 7*60*60)).Format("2006-01-02")
	if s.DayWIB == "" {
		s.DayWIB = day
	}
	if s.DayWIB != day {
		context := s.Context(step.At)
		if !marksKnown(context) {
			s.Attention = true
		} else {
			s.DayStartUSDT = context.EquityUSDT
		}
		s.DayWIB = day
	}
	var err error
	switch step.Op {
	case "submit":
		if step.Intent == nil {
			return errors.New("submit needs an intent")
		}
		err = s.submit(*step.Intent, step.At, p)
	case "broadcast":
		err = s.broadcast(step)
	case "unknown":
		o, ok := s.Orders[step.OrderID]
		if !ok || !orderOpen(o) {
			return errors.New("unknown status needs an open order")
		}
		o.Status = "unknown"
		s.Orders[step.OrderID] = o
	case "receipt":
		if step.Receipt == nil {
			return errors.New("receipt missing")
		}
		err = s.receipt(step.OrderID, *step.Receipt, step.At)
	case "match":
		if step.Book == nil {
			return errors.New("match needs a book")
		}
		err = s.matchBook(step.OrderID, *step.Book, step.ID, step.At)
	case "allocation":
		if step.Allocation == nil {
			return errors.New("allocation missing")
		}
		err = s.allocation(*step.Allocation)
	case "mark":
		if step.Mark == nil {
			return errors.New("mark missing")
		}
		err = s.mark(*step.Mark, step.At)
	default:
		return errors.New("unsupported fixture step; no live operation exists")
	}
	if err != nil {
		return err
	}
	s.At = step.At
	context := s.Context(step.At)
	if marksKnown(context) && context.EquityUSDT.GreaterThan(s.PeakUSDT) {
		s.PeakUSDT = context.EquityUSDT
	}
	return s.invariants(p)
}

func marksKnown(c PaperContext) bool {
	for _, p := range c.Positions {
		if !p.MarkKnown {
			return false
		}
	}
	return true
}

func (s *PaperState) submit(i PaperIntent, now time.Time, p PaperPolicy) error {
	if i.ID == "" || i.PositionID == "" || !i.AssumedVerified || !i.CostsKnown || i.NotionalUSDT.IsNegative() || i.Quantity.IsNegative() || i.LimitPrice.IsNegative() || i.MaxFeeUSDT.IsNegative() || i.CostFraction.IsNegative() || i.CostFraction.GreaterThanOrEqual(decimal.NewFromInt(1)) || !i.ExpiresAt.After(now) || i.ExpiresAt.Sub(now) > time.Duration(p.DecisionTTLSeconds)*time.Second {
		return errors.New("invalid fixture admission, fees or expiry")
	}
	if _, ok := s.Orders[i.ID]; ok {
		return errors.New("order ID already exists")
	}
	c := i.Candidate
	sealed := c
	sealed.EvidenceIDs = slices.Clone(c.EvidenceIDs)
	sealed.Seal()
	if c.ID != sealed.ID || c.Revision != sealed.Revision || c.Asset.Key() == "" || !slices.Contains([]string{"presale", "dex", "cex"}, c.Kind) {
		return errors.New("candidate identity/revision invalid")
	}
	if (c.Kind == "presale" && (c.Venue != "pinksale" || c.Asset.Chain != "eip155:56")) || (c.Kind == "dex" && (c.Venue != "pancakeswap-v2" || c.Asset.Chain != "eip155:56")) || (c.Kind == "cex" && c.Venue != "binance-spot") {
		return errors.New("unsupported paper venue")
	}
	if s.pending(i.PositionID) {
		return errors.New("position already has an unresolved order")
	}
	reserve := i.MaxFeeUSDT
	opening := i.Action == "buy" || i.Action == "contribute"
	if opening {
		if _, exists := s.Positions[i.PositionID]; exists {
			return errors.New("position ID cannot be reused after settlement")
		}
		if s.uncertain() || !marksKnown(s.Context(now)) || s.Consumed[c.ID] {
			return errors.New("new exposure blocked by uncertainty or consumed candidate")
		}
		for _, position := range s.Positions {
			if positionOpen(position) && (position.ID == i.PositionID || position.Candidate.Asset.Key() == c.Asset.Key()) {
				return errors.New("asset already has paper exposure")
			}
		}
		for _, order := range s.Orders {
			if orderOpen(order) && order.Intent.Candidate.Asset.Key() == c.Asset.Key() {
				return errors.New("asset already reserved")
			}
		}
		if c.ObservedAt.IsZero() || c.ObservedAt.After(now) || now.Sub(c.ObservedAt) > EvidenceTTL || c.StartsAt.IsZero() || c.StartsAt.After(now) {
			return errors.New("candidate is stale or not open")
		}
		if c.Kind == "presale" {
			if i.Action != "contribute" || c.EndsAt.IsZero() || !now.Before(c.EndsAt) || !slices.Contains([]string{"live", "inprogress"}, c.State) {
				return errors.New("sale not active")
			}
		} else if i.Action != "buy" || now.Sub(c.StartsAt) > MaxAge {
			return errors.New("listing outside paper universe")
		}
		size, err := p.PlanSize(c.Kind, s.Context(now), i.StopFraction, i.CostFraction, true, now)
		if err != nil {
			return err
		}
		if !i.NotionalUSDT.IsPositive() || i.NotionalUSDT.GreaterThan(size.NotionalUSDT) || i.MaxFeeUSDT.GreaterThan(i.NotionalUSDT.Mul(i.CostFraction)) {
			return errors.New("intent exceeds paper size or modeled fee ceiling")
		}
		if i.Thesis == "" || i.Invalidation == "" {
			return errors.New("entry thesis/invalidation missing")
		}
		if c.Kind != "presale" && (!i.Quantity.IsPositive() || !i.LimitPrice.IsPositive() || (c.Kind == "cex" && i.Quantity.Mul(i.LimitPrice).GreaterThan(i.NotionalUSDT))) {
			return errors.New("invalid trade quantity/price ceiling")
		}
		if c.Kind == "presale" && !i.Quantity.IsZero() {
			return errors.New("presale contribution does not mint claimed inventory")
		}
		reserve = reserve.Add(i.NotionalUSDT)
	} else {
		position, ok := s.Positions[i.PositionID]
		if !ok || !positionOpen(position) || c.Asset.Key() != position.Candidate.Asset.Key() {
			return errors.New("position/asset not found")
		}
		if i.MaxFeeUSDT.GreaterThan(position.FeeBudgetUSDT) {
			return errors.New("position fee budget exhausted")
		}
		switch i.Action {
		case "sell":
			if !i.Quantity.IsPositive() || i.Quantity.GreaterThan(position.Quantity) || !i.LimitPrice.IsPositive() || c.Kind == "presale" {
				return errors.New("exit exceeds tradable inventory or lacks trading venue")
			}
		case "claim":
			if c.ID != position.Candidate.ID || c.Kind != "presale" || !i.Quantity.IsPositive() || i.Quantity.GreaterThan(position.UnlockedQuantity.Sub(position.ClaimedQuantity)) || position.RefundableUSDT.GreaterThan(position.RefundedUSDT) {
				return errors.New("claim not unlocked or refund must settle first")
			}
		case "refund":
			if c.ID != position.Candidate.ID || c.Kind != "presale" || !i.NotionalUSDT.IsPositive() || i.NotionalUSDT.GreaterThan(position.RefundableUSDT.Sub(position.RefundedUSDT)) || position.ClaimedQuantity.IsPositive() {
				return errors.New("refund not available for this sale lifecycle")
			}
		default:
			return errors.New("unsupported paper action")
		}
	}
	if reserve.GreaterThan(s.CashUSDT) {
		return errors.New("paper cash insufficient")
	}
	s.CashUSDT = s.CashUSDT.Sub(reserve)
	s.Orders[i.ID] = PaperOrder{Intent: i, At: now, Status: "reserved", ReserveUSDT: reserve}
	if opening {
		s.Consumed[c.ID] = true
	}
	return nil
}

func paperStepShape(step PaperStep) error {
	count := 0
	for _, present := range []bool{step.Intent != nil, step.Book != nil, step.Route != nil, step.Receipt != nil, step.Allocation != nil, step.Mark != nil} {
		if present {
			count++
		}
	}
	valid := false
	orderRequired := false
	switch step.Op {
	case "submit":
		valid = step.Intent != nil && count == 1
	case "match":
		valid = step.Book != nil && count == 1
		orderRequired = true
	case "receipt":
		valid = step.Receipt != nil && count == 1
		orderRequired = true
	case "allocation":
		valid = step.Allocation != nil && count == 1
	case "mark":
		valid = step.Mark != nil && count == 1
	case "broadcast":
		valid = count == 0 || (count == 1 && step.Route != nil)
		orderRequired = true
	case "unknown":
		valid = count == 0
		orderRequired = true
	}
	if !valid || (orderRequired && step.OrderID == "") || (!orderRequired && step.OrderID != "") || (step.Op != "broadcast" && step.ExternalID != "") {
		return errors.New("paper operation has missing or ambiguous payload")
	}
	return nil
}

func (s *PaperState) allocation(a PaperAllocation) error {
	p, ok := s.Positions[a.PositionID]
	if !ok || p.Candidate.Kind != "presale" || !a.Finalized || !strings.HasPrefix(a.Proof, "fixture:") || s.pending(a.PositionID) {
		return errors.New("allocation lacks finalized fixture proof or position is busy")
	}
	if a.TotalQuantity.IsNegative() || a.UnlockedQuantity.IsNegative() || a.RefundableUSDT.IsNegative() || a.UnlockedQuantity.GreaterThan(a.TotalQuantity) || a.UnlockedQuantity.LessThan(p.UnlockedQuantity) || a.TotalQuantity.LessThan(p.ClaimedQuantity) || a.RefundableUSDT.LessThan(p.RefundedUSDT) || a.RefundableUSDT.GreaterThan(p.ContributedUSDT) || (p.ClaimedQuantity.IsPositive() && (!a.TotalQuantity.Equal(p.AllocatedQuantity) || !a.RefundableUSDT.Equal(p.RefundableUSDT))) {
		return errors.New("allocation/vesting/refund contradicts settled history")
	}
	if a.RefundableUSDT.Equal(p.ContributedUSDT) && a.TotalQuantity.IsPositive() {
		return errors.New("full refund with token allocation is unsupported")
	}
	p.AllocatedQuantity = a.TotalQuantity
	p.UnlockedQuantity = a.UnlockedQuantity
	p.RefundableUSDT = a.RefundableUSDT
	s.Positions[p.ID] = p
	return nil
}

func (s *PaperState) mark(m PaperMark, now time.Time) error {
	p, ok := s.Positions[m.PositionID]
	if !ok || !strings.HasPrefix(m.Proof, "fixture:") || m.At.IsZero() || m.At.After(now) || now.Sub(m.At) > 5*time.Second || (m.Known && !m.PriceUSDT.IsPositive()) {
		return errors.New("invalid or stale fixture mark")
	}
	p.MarkAt = m.At
	p.MarkPrice = decimal.Zero
	if m.Known {
		p.MarkPrice = m.PriceUSDT
		if p.StopPrice.IsPositive() && p.MarkPrice.LessThanOrEqual(p.StopPrice) {
			p.ExitRequired = true
		}
	}
	s.Positions[p.ID] = p
	return nil
}

func (s PaperState) invariants(p PaperPolicy) error {
	if s.CashUSDT.IsNegative() || s.FeesUSDT.IsNegative() {
		return errors.New("negative paper balance")
	}
	accounted := s.CashUSDT.Add(s.reserved("")).Add(s.FeesUSDT)
	for _, position := range s.Positions {
		if position.CostUSDT.IsNegative() || position.LockedUSDT.IsNegative() || position.Quantity.IsNegative() || position.FeeBudgetUSDT.IsNegative() {
			return errors.New("negative paper inventory/basis/budget")
		}
		accounted = accounted.Add(position.CostUSDT).Add(position.LockedUSDT)
	}
	if !accounted.Equal(p.InitialUSDT.Add(s.RealizedGrossUSDT)) {
		return errors.New("paper cash/cost/fee conservation failed")
	}
	return nil
}
