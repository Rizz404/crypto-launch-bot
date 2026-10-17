package binance

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rizz404/trading-bot/internal/broker"
	"github.com/rizz404/trading-bot/internal/domain"
)

// TestRecordedSpotTestnetPayloads replays responses and user stream messages
// recorded on the Binance Spot Testnet (testnet_test.go, TB_TESTNET_RECORD=1)
// through the parsers, so real payload shapes stay covered offline.
func TestRecordedSpotTestnetPayloads(t *testing.T) {
	files, err := filepath.Glob("testdata/testnet/*.json")
	require.NoError(t, err)
	require.NotEmpty(t, files)
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			raw, err := os.ReadFile(file)
			require.NoError(t, err)
			var payloads []struct {
				Stream, Method, Path string
				Body                 json.RawMessage
			}
			require.NoError(t, json.Unmarshal(raw, &payloads))
			tr := NewTrader(New(Options{Testnet: true, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}),
				"key", "secret", []domain.Market{domain.Spot})
			var events []broker.Event
			tr.SetHandler(func(ev broker.Event) { events = append(events, ev) })
			seen := map[string]int{}
			for _, p := range payloads {
				switch {
				case p.Stream == "spot-user":
					before := len(events)
					tr.handleSpotMessage(p.Body)
					checkExecution(t, p.Body, events[before:], seen)
				case p.Method == "POST" && p.Path == "/api/v3/order":
					var r orderResp
					require.NoError(t, json.Unmarshal(p.Body, &r))
					assert.Positive(t, r.OrderID)
					assert.Regexp(t, `^tbit[0-9a-z]+$`, r.ClientOrderID)
					seen["rest order"]++
				case p.Path == "/api/v3/orderList/oco":
					var r struct {
						Orders []struct {
							OrderID       int64  `json:"orderId"`
							ClientOrderID string `json:"clientOrderId"`
						} `json:"orders"`
					}
					require.NoError(t, json.Unmarshal(p.Body, &r))
					require.Len(t, r.Orders, 2)
					for _, o := range r.Orders {
						assert.Positive(t, o.OrderID)
						assert.NotEmpty(t, o.ClientOrderID)
					}
					seen["rest oco"]++
				case p.Path == "/api/v3/openOrders":
					var orders []spotOrder
					require.NoError(t, json.Unmarshal(p.Body, &orders))
					for _, o := range orders {
						d := o.domain("binance-testnet")
						assert.True(t, d.Status.Open(), "%s %s", d.ClientID, o.Status)
						assert.True(t, d.Quantity.IsPositive())
						if d.Type != domain.OrderLimit {
							assert.True(t, d.StopPrice.IsPositive(), "conditional %s needs a stop price", d.ClientID)
						}
					}
				case p.Path == "/api/v3/order": // GET (query) or DELETE (cancel)
					var o spotOrder
					require.NoError(t, json.Unmarshal(p.Body, &o))
					assert.False(t, o.domain("binance-testnet").Status.Open(), "%s %s", p.Method, o.Status)
					seen["rest final "+p.Method]++
				}
			}
			t.Logf("checked %v", seen)
			assert.Positive(t, seen["rest order"])
		})
	}
}

// checkExecution verifies the events emitted for one user stream message.
// Keys are read through a map because Binance uses case twins ("c"/"C").
func checkExecution(t *testing.T, body json.RawMessage, got []broker.Event, seen map[string]int) {
	t.Helper()
	var msg struct {
		Event map[string]json.RawMessage `json:"event"`
	}
	require.NoError(t, json.Unmarshal(body, &msg))
	str := func(k string) string {
		var s string
		_ = json.Unmarshal(msg.Event[k], &s)
		return s
	}
	if str("e") != "executionReport" {
		assert.Empty(t, got, "%s emits no broker event", str("e"))
		return
	}
	exec := str("x")
	seen["ws "+exec]++
	switch exec {
	case "TRADE":
		require.Len(t, got, 1)
		ev := got[0]
		assert.Equal(t, broker.EventFill, ev.Kind)
		assert.True(t, ev.Fill.Quantity.Equal(decimal.RequireFromString(str("l"))))
		assert.True(t, ev.Fill.Price.Equal(decimal.RequireFromString(str("L"))))
		var trade int64
		require.NoError(t, json.Unmarshal(msg.Event["t"], &trade))
		assert.Equal(t, strconv.FormatInt(trade, 10), ev.Fill.TradeID)
		assert.Equal(t, orderStatus(str("X")), ev.Order.Status)
		assert.Equal(t, str("c"), ev.Fill.OrderClientID)
	case "CANCELED", "EXPIRED":
		require.Len(t, got, 1)
		id := str("c")
		if exec == "CANCELED" && str("C") != "" {
			id = str("C") // the cancel request's own id is in "c"
		}
		assert.Equal(t, broker.EventOrder, got[0].Kind)
		assert.Equal(t, id, got[0].Order.ClientID)
		assert.Equal(t, orderStatus(str("X")), got[0].Order.Status)
		assert.False(t, got[0].Order.Status.Open())
	default: // NEW: acknowledged by the REST response already
		assert.Empty(t, got, "execution %s", exec)
	}
}
