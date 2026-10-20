package managed

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
)

// ResolveMinAge keeps an operator from settling a submission that may still
// be in flight: Binance can take seconds to make a new order queryable.
const ResolveMinAge = 5 * time.Minute

// resolveOrder settles an uncertain order (submitting, unknown or canceling)
// from the exchange's own answer, never from the operator's assumption:
//   - not found: the order never reached the exchange; release its reservation
//   - open: track it again as an open order
//   - terminal: book missed fills and the final state through SyncOrders
//
// The decision is journaled under the command's idempotency key.
func (r *Runtime) resolveOrder(ctx context.Context, c Command) error {
	if !c.Confirm {
		return errors.New("explicit confirmation required")
	}
	st, err := r.repo.Load(ctx)
	if err != nil {
		return err
	}
	if _, ok := st.Requests[c.ID]; ok {
		return nil
	}
	tracked := st.Orders[c.Order]
	if tracked == nil {
		return errors.New("unknown bot order")
	}
	if !slices.Contains([]string{"submitting", "unknown", "canceling"}, tracked.State) {
		return fmt.Errorf("order is %s; only uncertain orders can be resolved", tracked.State)
	}
	created := tracked.Order.CreatedAt
	if created.IsZero() {
		created = tracked.Order.UpdatedAt
	}
	if age := r.clock().Sub(created); created.IsZero() || age < ResolveMinAge {
		return fmt.Errorf("order may still be in flight; retry after %s", ResolveMinAge)
	}
	q, ok := r.physical.(broker.OrderQuerier)
	if !ok {
		return errors.New("exchange order query unavailable; order left unresolved")
	}
	final, err := q.QueryOrder(ctx, tracked.Order)
	switch {
	case errors.Is(err, broker.ErrOrderNotFound):
		if tracked.Order.FilledQty.IsPositive() {
			return errors.New("order has booked fills but exchange has no record; review exchange history")
		}
		err = r.repo.Update(ctx, c.ID, "resolve_order", c, func(s *State) error {
			o := s.Orders[c.Order]
			if o == nil || o.State == "terminal" {
				return nil
			}
			s.Reject(c.Order, "operator verified: exchange has no such order")
			s.Pending = append(s.Pending, broker.Event{Kind: broker.EventOrder, Order: o.Order})
			s.Note("resolve_order", o.Owner, c.Order+" not found on exchange; reservation released")
			return nil
		})
	case err != nil:
		return fmt.Errorf("exchange query failed; order left unresolved: %w", err)
	default:
		err = r.repo.Update(ctx, c.ID, "resolve_order", c, func(s *State) error {
			o := s.Orders[c.Order]
			if o == nil {
				return nil
			}
			if final.Status.Open() {
				o.Order.ExchangeID = final.ExchangeID
				o.State = "open"
			}
			s.Note("resolve_order", o.Owner, c.Order+" found on exchange as "+string(final.Status))
			return nil
		})
		if err == nil {
			if live, ok := r.physical.(broker.Live); ok {
				// Books fills the stream missed and, when terminal, the final state.
				live.SyncOrders(ctx, []domain.Order{tracked.Order}, nil)
				if final.Status.Open() {
					live.Track([]domain.Order{tracked.Order})
				}
			}
		}
	}
	if err != nil {
		return err
	}
	if err = r.drain(ctx); err != nil {
		return err
	}
	// Attention is derived from the whole wallet; recompute it from evidence.
	return r.reconcile(ctx)
}
