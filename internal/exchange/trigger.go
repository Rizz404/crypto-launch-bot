package exchange

import (
	"sync"
	"time"

	"github.com/rizz404/trading-bot/internal/broker"
)

// TriggerBook maps orders created by triggered conditional (algo) orders
// back to the conditional order's client id. Exchanges report the fills of
// such orders under a new order id, sometimes before announcing which
// conditional order created it, so unknown fills are held briefly.
type TriggerBook struct {
	mu      sync.Mutex
	orders  map[string]string // exchange order id -> client id of the conditional order
	pending map[string][]heldEvent
	timeout time.Duration
}

type heldEvent struct {
	ev broker.Event
	at time.Time
}

func NewTriggerBook(timeout time.Duration) *TriggerBook {
	return &TriggerBook{orders: map[string]string{}, pending: map[string][]heldEvent{}, timeout: timeout}
}

// Lookup returns the client id mapped to an exchange order id.
func (b *TriggerBook) Lookup(orderID string) (string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	id, ok := b.orders[orderID]
	return id, ok
}

// Map records that orderID was created by the conditional order clientID
// and returns the held events of that order, re-labelled with clientID.
func (b *TriggerBook) Map(orderID, clientID string) []broker.Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.orders[orderID] = clientID
	held := b.pending[orderID]
	delete(b.pending, orderID)
	out := make([]broker.Event, len(held))
	for i, h := range held {
		h.ev.Order.ClientID, h.ev.Fill.OrderClientID = clientID, clientID
		out[i] = h.ev
	}
	return out
}

// Hold keeps an event of an unknown order until it is mapped or expires.
func (b *TriggerBook) Hold(orderID string, ev broker.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pending[orderID] = append(b.pending[orderID], heldEvent{ev, time.Now()})
}

// Expired removes and returns held events older than the timeout.
func (b *TriggerBook) Expired() []broker.Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []broker.Event
	for id, hs := range b.pending {
		if time.Since(hs[0].at) < b.timeout {
			continue
		}
		for _, h := range hs {
			out = append(out, h.ev)
		}
		delete(b.pending, id)
	}
	return out
}

// Forget drops the mapping of an exchange order id.
func (b *TriggerBook) Forget(orderID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.orders, orderID)
}
