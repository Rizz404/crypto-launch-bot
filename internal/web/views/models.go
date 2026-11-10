// Package views holds the templ templates of the web dashboard and the
// view models they render. Values are pre-formatted strings.
package views

import "strconv"

func itoa(n int) string     { return strconv.Itoa(n) }
func itoa64(n int64) string { return strconv.FormatInt(n, 10) }

// Status is the live session summary.
type Status struct {
	Mode          string
	Live          bool
	Equity        string
	Unrealized    string
	UnrealizedPos bool
	Exposure      string
	OpenPositions int
	Killed        bool
	KillReason    string
	Paused        bool
	Accounts      []AccountRow
}

type AccountRow struct {
	Key, Balance, Equity, Unrealized, FreeMargin string
	Positions                                    int
}

type PositionRow struct {
	Account, Symbol, Side, Qty, Entry, Mark, UPnL, Strategy, Opened, ChartHref string
	UPnLPositive                                                               bool
}

type OrderRow struct {
	Symbol, Side, Type, Tag, Qty, Price, Strategy, Status, Created string
}

type TradeRow struct {
	Closed, Strategy, Symbol, Side, Qty, Entry, Exit, Net, Fees, Reason, ChartHref string
	Win                                                                            bool
}

type RunRow struct {
	ID                                                   int64
	Href, Instance, Symbols, Segment, Period, Started    string
	Return, MaxDD, Sharpe, Trades, WinRate, ProfitFactor string
	Positive                                             bool
}

type RunDetail struct {
	Run       RunRow
	Metrics   [][2]string
	Params    string
	Trades    []TradeRow
	EquitySrc string
	Charts    []Link
}

type Link struct {
	Label, Href string
	Active      bool
}

type ChartPage struct {
	Series     []Link
	Timeframes []Link
	Selected   string
	Src        string
	Note       string
}
