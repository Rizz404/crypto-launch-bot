package managed

import (
	"context"
	"log/slog"
	"math"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/rizz404/crypto-launch-bot/internal/broker"
	"github.com/rizz404/crypto-launch-bot/internal/domain"
	"github.com/rizz404/crypto-launch-bot/internal/exchange"
	"github.com/rizz404/crypto-launch-bot/internal/marketview"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

// Preview login for the fixture server; nothing here touches a real account.
const (
	previewEmail    = "preview@example.com"
	previewPassword = "preview-pass"
)

// previewSource fakes a market around 105 USDT: a candle history, a book and
// a trade/ticker tick every second, so the trading screen looks alive.
type previewSource struct{}

func (previewSource) Name() string { return "preview" }

func previewPrice(t time.Time) float64 {
	h := float64(t.Unix()) / 3600
	return 105 + 6*math.Sin(h/9) + 2*math.Sin(h/1.7) + 0.6*math.Sin(h*3.1)
}

func (previewSource) Klines(_ context.Context, req exchange.KlineRequest) ([]domain.Candle, error) {
	step := req.Timeframe.Duration()
	end := req.Timeframe.Truncate(time.Now().UTC())
	out := make([]domain.Candle, 0, 300)
	for i := 299; i >= 0; i-- {
		at := end.Add(-time.Duration(i) * step)
		o, c := previewPrice(at), previewPrice(at.Add(step))
		hi, lo := math.Max(o, c)*1.002, math.Min(o, c)*0.998
		out = append(out, domain.Candle{Symbol: req.Symbol, Timeframe: req.Timeframe, OpenTime: at,
			Open: decimal.NewFromFloat(o).Round(2), High: decimal.NewFromFloat(hi).Round(2), Low: decimal.NewFromFloat(lo).Round(2), Close: decimal.NewFromFloat(c).Round(2), Volume: decimal.NewFromInt(40 + int64(i%17))})
	}
	return out, nil
}

func previewBook(mid float64) (bids, asks []exchange.Level) {
	for i := 1; i <= 12; i++ {
		q := decimal.NewFromFloat(0.3 + float64((i*7)%11)/4).Round(3)
		bids = append(bids, exchange.Level{Price: decimal.NewFromFloat(mid - 0.05*float64(i)).Round(2), Qty: q})
		asks = append(asks, exchange.Level{Price: decimal.NewFromFloat(mid + 0.05*float64(i)).Round(2), Qty: q})
	}
	return bids, asks
}

func (previewSource) Depth(context.Context, string, int) (exchange.DepthSnapshot, error) {
	bids, asks := previewBook(previewPrice(time.Now()))
	return exchange.DepthSnapshot{LastUpdateID: 1, Bids: bids, Asks: asks}, nil
}

func (previewSource) StreamMarket(ctx context.Context, symbols []string, _ []domain.Timeframe) (<-chan exchange.MarketEvent, error) {
	ch := make(chan exchange.MarketEvent, 16)
	go func() {
		defer close(ch)
		send := func(e exchange.MarketEvent) bool {
			select {
			case ch <- e:
				return true
			case <-ctx.Done():
				return false
			}
		}
		if !send(exchange.MarketEvent{Kind: exchange.MarketConnected}) {
			return
		}
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for id := int64(2); ; id++ {
			now := time.Now()
			p := previewPrice(now) + 0.08*math.Sin(float64(now.UnixMilli())/700)
			price := decimal.NewFromFloat(p).Round(2)
			for _, s := range symbols {
				bids, asks := previewBook(p)
				ok := send(exchange.MarketEvent{Kind: exchange.MarketDepth, Depth: exchange.DepthUpdate{Symbol: s, FirstID: id, FinalID: id, Bids: bids, Asks: asks}}) &&
					send(exchange.MarketEvent{Kind: exchange.MarketTrade, Trade: exchange.Trade{Symbol: s, Price: price, Qty: decimal.NewFromFloat(0.05 + float64(id%9)/20).Round(3), Time: now, BuyerMaker: id%3 == 0}}) &&
					send(exchange.MarketEvent{Kind: exchange.MarketTicker, Ticker: exchange.Ticker24{Symbol: s, Last: price, Change: d("1.6"), ChangePct: d("1.55"), High: d("112.40"), Low: d("98.75"), Volume: d("15230"), QuoteVol: d("1603400")}})
				if !ok {
					return
				}
			}
			select {
			case <-tick.C:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

// TestDashboardPreview serves fixtures only. It never starts a runtime/feed,
// connects a database, loads .env, or creates exchange clients. Log in with
// previewEmail / previewPassword.
func TestDashboardPreview(t *testing.T) {
	addr := os.Getenv("TB_DASHBOARD_PREVIEW")
	if addr == "" {
		t.Skip("set TB_DASHBOARD_PREVIEW=127.0.0.1:18082 for fixture UI preview")
	}
	sp := spec()
	sp.Instances[0].Strategies = append(sp.Instances[0].Strategies, BindingSpec{ID: "manual", Name: ManualStrategy, Symbols: []string{"ETHUSDT"}, Risk: generous()})
	repo, err := NewMemory(sp)
	require.NoError(t, err)
	now := time.Now().UTC()
	require.NoError(t, repo.Update(t.Context(), "fixture", "fixture", nil, func(s *State) error {
		s.StartedAt = now.Add(-24 * time.Hour)
		s.Mark("ETHUSDT", d("100"))
		s.PriceAt["ETHUSDT"] = now
		for n := 0; n < 24; n++ {
			s.Instances["a"].Cash = d("500").Add(d("0.5").Mul(decimal.NewFromInt(int64(n))))
			s.Sample(now.Add(time.Duration(n-24) * time.Hour))
		}
		s.Instances["a"].Cash = d("480")
		s.Holding("a/one", "ETHUSDT").Quantity = d("0.2")
		s.Holding("a/one", "ETHUSDT").Cost = d("20")
		s.Mark("ETHUSDT", d("105"))
		s.Sample(now)
		s.Observe(now)
		s.Bindings["a/one"].LastNotice = "Menunggu candle dan sinyal strategi berikutnya"
		return nil
	}))
	rt, err := NewRuntime(t.Context(), repo, broker.NewSim(broker.SimConfig{Exchange: "sim"}), testInstruments(), d("0.001"), nil, nil)
	require.NoError(t, err)

	hub := &marketview.Hub{Source: previewSource{}, Symbols: []string{"ETHUSDT"}, Timeframes: marketview.ChartTimeframes, Log: slog.New(slog.DiscardHandler)}
	go func() { _ = hub.Run(t.Context()) }()

	// An empty wallet, like the fresh production start: build instances from the dashboard.
	fresh, err := NewMemory(Spec{ID: "fresh", Exchange: "sim", Mode: "paper", Quote: "USDT", Capital: d("1000")})
	require.NoError(t, err)
	freshRT, err := NewRuntime(t.Context(), fresh, broker.NewSim(broker.SimConfig{Exchange: "sim"}), testInstruments(), d("0.001"), nil, nil)
	require.NoError(t, err)

	h := NewHTTP(map[string]*Runtime{"fixture-paper": rt, "fresh": freshRT}, "", nil)
	h.Markets = func(string) *marketview.Hub { return hub }
	h.Fiat = func() (decimal.Decimal, decimal.Decimal, bool) { return d("0.999"), d("18090"), true }
	// The real bot exits and the container restarts; the preview only reports it.
	h.Restart = func() { t.Log("restart requested: in production the bot restarts and new strategies start") }
	h.Owners = &MemOwners{}
	hash, err := hashPassword(previewPassword)
	require.NoError(t, err)
	_, err = h.Owners.Register(t.Context(), Owner{Email: previewEmail, Hash: hash})
	require.NoError(t, err)

	srv := &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 5 * time.Second}
	t.Cleanup(func() { _ = srv.Close() })
	t.Log("fixture dashboard: http://" + addr + "/login  (" + previewEmail + " / " + previewPassword + ")")
	require.ErrorIs(t, srv.ListenAndServe(), http.ErrServerClosed)
}
