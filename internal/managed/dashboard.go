package managed

import (
	"cmp"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/shopspring/decimal"

	"github.com/rizz404/trading-bot/internal/ui"
)

// recentRows caps the tables that grow with time; the rest is in the journal
// and the archive (see managed-runtime.md).
const recentRows = 200

// view is everything a dashboard page needs. Maps in State are exposed as
// sorted slices because Go randomises map iteration and templates must be stable.
type view struct {
	Wallet  string
	Wallets []string
	Tab     string
	State   *State
	Token   string
	Error   string
	Rate    string
	money   func(decimal.Decimal) string
	links   []ui.NavItem
}

var tabs = []ui.NavItem{
	{Key: "ringkasan", Label: "Ringkasan"},
	{Key: "instance", Label: "Instance & strategi"},
	{Key: "posisi", Label: "Posisi & order"},
	{Key: "riwayat", Label: "Riwayat"},
}

func (v view) Money(d decimal.Decimal) string { return v.money(d) }
func (v view) Action() templ.SafeURL {
	return templ.SafeURL("/control?wallet=" + url.QueryEscape(v.Wallet) + "&tab=" + v.Tab)
}
func (v view) Export() templ.SafeURL {
	return templ.SafeURL("/api/state?wallet=" + url.QueryEscape(v.Wallet))
}
func (v view) Href(tab string) string {
	return "/?wallet=" + url.QueryEscape(v.Wallet) + "&tab=" + tab
}
func (v view) Tabs() []ui.NavItem {
	out := make([]ui.NavItem, len(tabs))
	for i, t := range tabs {
		out[i] = ui.NavItem{Key: t.Key, Label: t.Label, Href: v.Href(t.Key)}
	}
	return out
}
func (v view) WalletNav() []ui.NavItem {
	out := make([]ui.NavItem, len(v.Wallets))
	for i, id := range v.Wallets {
		out[i] = ui.NavItem{Key: id, Label: id, Href: "/?wallet=" + url.QueryEscape(id) + "&tab=" + v.Tab}
	}
	return append(out, v.links...)
}

func (v view) PnL() decimal.Decimal { return v.State.Equity().Sub(v.State.Spec.Capital) }
func (v view) InstancePnL(id string) decimal.Decimal {
	return v.State.InstanceEquity(id).Sub(v.State.Instances[id].Funded)
}
func (v view) PnLTone(d decimal.Decimal) string {
	switch {
	case d.IsPositive():
		return "pos"
	case d.IsNegative():
		return "neg"
	}
	return ""
}
func (v view) Drawdown(equity, peak decimal.Decimal) string { return drawdown(equity, peak) }

func (v view) InstanceIDs() []string {
	ids := make([]string, 0, len(v.State.Instances))
	for id := range v.State.Instances {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// BindingIDs returns the owners (instance/binding) of one instance, sorted.
func (v view) BindingIDs(instance string) []string {
	var ids []string
	for id, b := range v.State.Bindings {
		if b.InstanceID == instance {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}

type priceRow struct {
	Symbol string
	Price  decimal.Decimal
	At     time.Time
}

func (v view) Prices() []priceRow {
	rows := make([]priceRow, 0, len(v.State.Prices))
	for sym, p := range v.State.Prices {
		rows = append(rows, priceRow{sym, p, v.State.PriceAt[sym]})
	}
	slices.SortFunc(rows, func(a, b priceRow) int { return cmp.Compare(a.Symbol, b.Symbol) })
	return rows
}

func (v view) Holdings() []*Holding {
	var rows []*Holding
	for _, h := range v.State.Holdings {
		if h.Quantity.IsPositive() {
			rows = append(rows, h)
		}
	}
	slices.SortFunc(rows, func(a, b *Holding) int { return cmp.Or(cmp.Compare(a.Owner, b.Owner), cmp.Compare(a.Symbol, b.Symbol)) })
	return rows
}
func (v view) HoldingValue(h *Holding) decimal.Decimal {
	return h.Quantity.Mul(v.State.Prices[h.Symbol])
}

func (v view) Orders() []*Order {
	rows := make([]*Order, 0, len(v.State.Orders))
	for _, o := range v.State.Orders {
		rows = append(rows, o)
	}
	slices.SortFunc(rows, func(a, b *Order) int { return a.Order.CreatedAt.Compare(b.Order.CreatedAt) })
	return rows
}

// Uncertain lists orders whose ledger state needs an exchange check.
func (v view) Uncertain() []*Order {
	var out []*Order
	for _, o := range v.Orders() {
		if o.State == "submitting" || o.State == "unknown" || o.State == "canceling" {
			out = append(out, o)
		}
	}
	return out
}

// Executions and History return the newest rows first, capped at recentRows;
// the second value is how many rows exist in total.
func (v view) Executions() ([]Execution, int) {
	rows := slices.Clone(v.State.Executions)
	slices.Reverse(rows)
	return rows[:min(len(rows), recentRows)], len(rows)
}
func (v view) History() ([]Audit, int) {
	rows := slices.Clone(v.State.History)
	slices.Reverse(rows)
	return rows[:min(len(rows), recentRows)], len(rows)
}

func (v view) ChartPath() string { return chartPath(v.State.Samples) }
func (v view) ChartRange() (lo, hi decimal.Decimal) {
	return chartBounds(v.State.Samples)
}
func (v view) SampleSpan() (first, last time.Time) {
	s := v.State.Samples
	return s[0].At, s[len(s)-1].At
}

type limitsView struct {
	Scope                                     string
	Risk, Position, Exposure, Daily, Drawdown string
	Cap                                       string
}

func ts(t time.Time) string { return timestamp(t) }
func itoa(n int) string     { return strconv.Itoa(n) }

func pct(f float64) string { return strconv.FormatFloat(f*100, 'f', -1, 64) }

func (v view) Limits(scope string) limitsView {
	c := v.State.Spec.Risk
	capital := decimal.Zero
	if i := v.State.Instances[scope]; i != nil {
		c = i.Risk
	}
	if b := v.State.Bindings[scope]; b != nil {
		c = b.Spec.Risk
		capital = b.Spec.Cap
	}
	c = effectiveRisk(c)
	return limitsView{scope, pct(c.RiskPerTrade), pct(c.MaxPositionPct), pct(c.MaxExposurePct), pct(c.DailyLossLimit), pct(c.MaxDrawdown), capital.String()}
}

func (v view) Unrealized(owner string) decimal.Decimal { return v.State.Unrealized(owner) }
func (v view) FreeCash(id string) decimal.Decimal      { return v.State.FreeCash(id) }

func (v view) StatusTone(status string) string {
	switch status {
	case "running":
		return "pos"
	case "paused":
		return "warn"
	case "halted", "killed", "error":
		return "neg"
	}
	return ""
}

func (v view) Symbols(b *Binding) string { return strings.Join(b.Spec.Symbols, ", ") }

func sortedWallets(wallets map[string]*Runtime) []string {
	ids := make([]string, 0, len(wallets))
	for id := range wallets {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// control describes one lifecycle button. Confirm ones go through the
// confirmation dialog, which is also what sets the server's confirm flag.
type control struct {
	Op, Label, Policy, Effect string
	Confirm, Danger           bool
}

var controlSet = []control{
	{Op: "pause", Label: "Jeda entry", Effect: "Entry baru ditahan; exit dan proteksi tetap berjalan."},
	{Op: "resume", Label: "Lanjutkan", Effect: "Entry dibuka lagi bila status sehat."},
	{Op: "cancel", Label: "Batalkan order pending", Confirm: true, Effect: "Membatalkan order entry/raw yang masih menunggu. Tidak menjual koin."},
	{Op: "flatten", Label: "Tutup posisi", Confirm: true, Danger: true, Effect: "Membatalkan proteksi lalu menjual seluruh inventori scope ini."},
	{Op: "finish", Policy: "retain", Label: "Akhiri · pertahankan koin", Confirm: true, Effect: "Menyelesaikan order dan mengakhiri eksperimen; koin tetap disimpan."},
	{Op: "finish", Policy: "flatten", Label: "Akhiri · jual koin", Confirm: true, Danger: true, Effect: "Menyelesaikan order, menjual koin milik scope ini, lalu mengakhiri eksperimen."},
	{Op: "kill", Label: "Penghentian darurat", Confirm: true, Danger: true, Effect: "Halt permanen sampai di-reset. Reset tidak menghapus selisih saldo."},
	{Op: "reset", Label: "Reset batas risiko", Confirm: true, Effect: "Menghapus halt risiko. Selisih saldo tidak ikut terhapus."},
	{Op: "archive", Label: "Arsipkan", Confirm: true, Effect: "Hanya berhasil bila order selesai dan posisi kosong atau tinggal debu."},
}

func (c control) Class() string {
	if c.Danger {
		return ui.BtnDanger + ui.BtnSmall
	}
	return ui.BtnSecondary + ui.BtnSmall
}

func (c control) Attrs(label string) templ.Attributes {
	if !c.Confirm {
		return nil
	}
	a := templ.Attributes{
		"data-confirm":       c.Label + "?",
		"data-confirm-body":  "Scope: " + label + "\n" + c.Effect,
		"data-confirm-label": c.Label,
		"data-confirm-set":   "confirm=yes",
	}
	if c.Danger {
		a["data-confirm-tone"] = "danger"
	}
	return a
}
