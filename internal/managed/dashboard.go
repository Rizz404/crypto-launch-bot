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

	"github.com/rizz404/crypto-launch-bot/internal/ui"
)

// recentRows caps the tables that grow with time; the rest is in the journal
// and the archive (see docs/agents/managed-runtime.md).
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
	// OK is the result of the last action, shown once as a toast.
	OK   string
	Rate string
	// Trade shows the trading screen in the menu; Auth the account links.
	Trade, Auth bool
	// Inactive lists strategies added since the last start (no engine yet).
	Inactive   []string
	money      func(decimal.Decimal) string
	fiat       Fiat
	startPrice func(symbol string, at time.Time) (decimal.Decimal, bool)
	links      []ui.NavItem
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

// TabLabel names the current dashboard tab for the page heading.
func (v view) TabLabel() string {
	for _, t := range tabs {
		if t.Key == v.Tab {
			return t.Label
		}
	}
	return tabs[0].Label
}

// Sidebar is the menu of every page after login. The bot is the core: its
// four sections come first and open; manual trading is parked until it can
// trade any pair (its /trade screen still exists, unlinked).
func (v view) Sidebar() []ui.NavGroup {
	bot := make([]ui.NavItem, 0, len(tabs)+len(v.links))
	for _, t := range tabs {
		bot = append(bot, ui.NavItem{Key: t.Key, Label: t.Label, Href: v.Href(t.Key)})
	}
	bot = append(bot, v.links...)
	wallets := make([]ui.NavItem, len(v.Wallets))
	for i, id := range v.Wallets {
		n := ui.NavItem{Label: id, Href: v.walletHref(id), Current: id == v.Wallet}
		if n.Current && v.State != nil {
			n.Hint, n.Tone = ui.ModeLabel(v.State.Spec.Mode), ui.ModeTone(v.State.Spec.Mode)
		}
		wallets[i] = n
	}
	return []ui.NavGroup{
		{Title: "Trading bot", Items: bot, Collapsible: true, Open: true},
		{Items: []ui.NavItem{{Label: "Trading manual", Hint: "Segera", Disabled: true}}},
		{Title: "Akun trading", Items: wallets, Collapsible: true, Open: true},
	}
}

// walletHref switches wallet while staying on the same page.
func (v view) walletHref(id string) string {
	switch v.Tab {
	case "trade":
		return "/trade?wallet=" + url.QueryEscape(id)
	case "account":
		return "/?wallet=" + url.QueryEscape(id)
	}
	return "/?wallet=" + url.QueryEscape(id) + "&tab=" + v.Tab
}

// shellPage is the common page frame; Active selects the menu item.
func (v view) shellPage(title, footer string) ui.Page {
	p := ui.Page{Title: title, Brand: "crypto-launch-bot", Active: v.Tab, Sidebar: v.Sidebar(), Footer: footer, Currency: v.fiat != nil}
	switch {
	case v.Error != "":
		p.Notice, p.NoticeTone = v.Error, "neg"
	case v.OK != "":
		p.Notice, p.NoticeTone = v.OK, "pos"
	}
	if v.Auth {
		p.SideFoot = sideAccount(v.Tab == "account")
	}
	return p
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

// ScopeStatus summarizes the account, an instance or a strategy: running,
// paused, mixed (some instances paused), halted, closing, stopped, archived or empty.
func (v view) ScopeStatus(scope string) string {
	if b := v.State.Bindings[scope]; b != nil {
		if b.Halted {
			return "halted"
		}
		return b.Status
	}
	if i := v.State.Instances[scope]; i != nil {
		if i.Halted {
			return "halted"
		}
		return i.Status
	}
	if v.State.Halted {
		return "halted"
	}
	seen := map[string]bool{}
	for _, i := range v.State.Instances {
		if i.Status != "archived" {
			seen[i.Status] = true
		}
	}
	switch {
	case len(seen) == 0:
		return "empty"
	case len(seen) > 1 && seen["running"]:
		return "mixed"
	case seen["running"]:
		return "running"
	case seen["paused"]:
		return "paused"
	}
	for status := range seen {
		return status
	}
	return "empty"
}

var statusText = map[string]string{"running": "Berjalan", "paused": "Dijeda", "mixed": "Sebagian dijeda", "halted": "Dihentikan (batas risiko)",
	"closing": "Sedang menutup", "stopped": "Berakhir", "archived": "Diarsipkan", "empty": "Belum ada instance"}

func (v view) StatusText(status string) string {
	if t, ok := statusText[status]; ok {
		return t
	}
	return status
}

// Primary are the buttons shown next to the status; the rest go in the menu.
func (v view) Primary(scope string) []control {
	switch v.ScopeStatus(scope) {
	case "running":
		return controlSet[:1]
	case "paused", "closing", "stopped":
		return controlSet[1:2]
	case "mixed":
		return controlSet[:2]
	case "halted":
		return []control{controlSet[7]}
	}
	return nil
}

// More are the menu actions: everything except pause/resume, safe ones first.
func (v view) More() (safe, danger []control) {
	for _, c := range controlSet[2:] {
		if c.Danger {
			danger = append(danger, c)
		} else {
			safe = append(safe, c)
		}
	}
	return safe, danger
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

// bindTimeframes are the kline intervals Binance offers (domain.ParseTimeframe).
var bindTimeframes = []string{"1m", "3m", "5m", "15m", "30m", "1h", "2h", "4h", "6h", "8h", "12h", "1d", "3d", "1w"}

// primaryClass highlights the action that changes the current status.
func primaryClass(c control) string {
	if c.Op == "resume" {
		return ui.BtnPrimary + ui.BtnSmall
	}
	return c.Class()
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
