// Package indicator implements streaming technical indicators. Each one
// is updated with one value per closed candle in O(1) and reports Ready once
// it has seen enough data. Indicators use float64; prices for orders stay
// decimal.
package indicator

import "math"

// window is a fixed-size ring buffer with a running sum.
type window struct {
	buf   []float64
	next  int
	count int
	sum   float64
	sumSq float64
}

func newWindow(n int) *window { return &window{buf: make([]float64, n)} }

func (w *window) push(v float64) {
	if w.count == len(w.buf) {
		old := w.buf[w.next]
		w.sum -= old
		w.sumSq -= old * old
	} else {
		w.count++
	}
	w.buf[w.next] = v
	w.sum += v
	w.sumSq += v * v
	w.next = (w.next + 1) % len(w.buf)
}

func (w *window) full() bool    { return w.count == len(w.buf) }
func (w *window) mean() float64 { return w.sum / float64(w.count) }

// stddev is the population standard deviation, as used by Bollinger Bands.
func (w *window) stddev() float64 {
	m := w.mean()
	return math.Sqrt(math.Max(w.sumSq/float64(w.count)-m*m, 0))
}

// SMA is a simple moving average.
type SMA struct{ w *window }

func NewSMA(period int) *SMA { return &SMA{w: newWindow(period)} }

func (s *SMA) Update(v float64) float64 { s.w.push(v); return s.Value() }
func (s *SMA) Ready() bool              { return s.w.full() }
func (s *SMA) Value() float64 {
	if s.w.count == 0 {
		return 0
	}
	return s.w.mean()
}

// EMA is an exponential moving average seeded with the SMA of the first
// period values (the TradingView/TA-Lib convention).
type EMA struct {
	period int
	alpha  float64
	seed   *window
	value  float64
	ready  bool
}

func NewEMA(period int) *EMA {
	return &EMA{period: period, alpha: 2 / float64(period+1), seed: newWindow(period)}
}

func (e *EMA) Update(v float64) float64 {
	if !e.ready {
		e.seed.push(v)
		e.value = e.seed.mean()
		e.ready = e.seed.full()
		return e.value
	}
	e.value += e.alpha * (v - e.value)
	return e.value
}

func (e *EMA) Ready() bool    { return e.ready }
func (e *EMA) Value() float64 { return e.value }

// wilder is Wilder's smoothing (RMA): an EMA with alpha = 1/period,
// seeded with the SMA of the first period values.
type wilder struct {
	period int
	seed   *window
	value  float64
	ready  bool
}

func newWilder(period int) *wilder { return &wilder{period: period, seed: newWindow(period)} }

func (w *wilder) update(v float64) float64 {
	if !w.ready {
		w.seed.push(v)
		w.value = w.seed.mean()
		w.ready = w.seed.full()
		return w.value
	}
	w.value = (w.value*float64(w.period-1) + v) / float64(w.period)
	return w.value
}

// RSI is Wilder's relative strength index (0-100).
type RSI struct {
	gain, loss *wilder
	prev       float64
	seen       bool
}

func NewRSI(period int) *RSI { return &RSI{gain: newWilder(period), loss: newWilder(period)} }

func (r *RSI) Update(close float64) float64 {
	if !r.seen {
		r.prev, r.seen = close, true
		return r.Value()
	}
	ch := close - r.prev
	r.prev = close
	r.gain.update(math.Max(ch, 0))
	r.loss.update(math.Max(-ch, 0))
	return r.Value()
}

func (r *RSI) Ready() bool { return r.gain.ready }
func (r *RSI) Value() float64 {
	if !r.gain.ready {
		return 50
	}
	if r.loss.value == 0 {
		if r.gain.value == 0 {
			return 50
		}
		return 100
	}
	rs := r.gain.value / r.loss.value
	return 100 - 100/(1+rs)
}

// ATR is Wilder's average true range.
type ATR struct {
	rma       *wilder
	prevClose float64
	seen      bool
}

func NewATR(period int) *ATR { return &ATR{rma: newWilder(period)} }

func (a *ATR) Update(high, low, close float64) float64 {
	tr := high - low
	if a.seen {
		tr = math.Max(tr, math.Max(math.Abs(high-a.prevClose), math.Abs(low-a.prevClose)))
	}
	a.prevClose, a.seen = close, true
	return a.rma.update(tr)
}

func (a *ATR) Ready() bool    { return a.rma.ready }
func (a *ATR) Value() float64 { return a.rma.value }

// MACD is the moving average convergence/divergence.
type MACD struct {
	fast, slow, signal *EMA
	line               float64
}

func NewMACD(fast, slow, signal int) *MACD {
	return &MACD{fast: NewEMA(fast), slow: NewEMA(slow), signal: NewEMA(signal)}
}

func (m *MACD) Update(v float64) {
	f, s := m.fast.Update(v), m.slow.Update(v)
	if !m.slow.Ready() {
		return
	}
	m.line = f - s
	m.signal.Update(m.line)
}

func (m *MACD) Ready() bool        { return m.slow.Ready() && m.signal.Ready() }
func (m *MACD) Line() float64      { return m.line }
func (m *MACD) Signal() float64    { return m.signal.Value() }
func (m *MACD) Histogram() float64 { return m.line - m.signal.Value() }

// Bollinger bands: SMA ± k standard deviations.
type Bollinger struct {
	w *window
	k float64
}

func NewBollinger(period int, k float64) *Bollinger { return &Bollinger{w: newWindow(period), k: k} }

func (b *Bollinger) Update(v float64) { b.w.push(v) }
func (b *Bollinger) Ready() bool      { return b.w.full() }
func (b *Bollinger) Middle() float64  { return b.w.mean() }
func (b *Bollinger) Upper() float64   { return b.w.mean() + b.k*b.w.stddev() }
func (b *Bollinger) Lower() float64   { return b.w.mean() - b.k*b.w.stddev() }

// Cross detects a crossover between two series from one update to the next.
type Cross struct {
	prevA, prevB float64
	seen         bool
}

// Update returns +1 when a crosses above b, -1 when it crosses below, else 0.
func (c *Cross) Update(a, b float64) int {
	defer func() { c.prevA, c.prevB, c.seen = a, b, true }()
	if !c.seen {
		return 0
	}
	switch {
	case c.prevA <= c.prevB && a > b:
		return 1
	case c.prevA >= c.prevB && a < b:
		return -1
	}
	return 0
}
