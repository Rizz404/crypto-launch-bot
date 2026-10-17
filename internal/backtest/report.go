package backtest

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// WriteReport prints a human-readable summary.
func WriteReport(w io.Writer, title string, r *Result) {
	m := r.Metrics
	pct := func(f float64) string { return fmt.Sprintf("%.2f%%", f*100) }
	pf := fmt.Sprintf("%.2f", m.ProfitFactor)
	if math.IsInf(m.ProfitFactor, 1) {
		pf = "∞"
	}
	rows := [][2]string{
		{"Period", m.Start.Format(time.DateOnly) + " → " + m.End.Format(time.DateOnly)},
		{"Candles", fmt.Sprint(r.Candles)},
		{"Equity", fmt.Sprintf("%.2f → %.2f", m.InitialEquity, m.FinalEquity)},
		{"Total return", pct(m.TotalReturn)},
		{"CAGR", pct(m.CAGR)},
		{"Max drawdown", pct(m.MaxDrawdown)},
		{"Sharpe / Sortino", fmt.Sprintf("%.2f / %.2f", m.Sharpe, m.Sortino)},
		{"Trades", fmt.Sprint(m.Trades)},
		{"Win rate", pct(m.WinRate)},
		{"Profit factor", pf},
		{"Avg trade / win / loss", fmt.Sprintf("%.2f / %.2f / %.2f", m.AvgTrade, m.AvgWin, m.AvgLoss)},
		{"Fees / funding", fmt.Sprintf("%.2f / %.2f", m.TotalFees, m.TotalFunding)},
		{"Avg holding time", m.AvgHoldingTime.Round(time.Minute).String()},
		{"Open at end", fmt.Sprint(len(r.OpenPositions))},
	}
	_, _ = fmt.Fprintf(w, "\n== %s ==\n", title)
	for _, row := range rows {
		_, _ = fmt.Fprintf(w, "  %-24s %s\n", row[0], row[1])
	}
}

// WriteCSV writes equity.csv and trades.csv into dir with the given prefix.
func WriteCSV(dir, prefix string, r *Result) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := func(s string) string { return filepath.Join(dir, strings.Trim(prefix+"-"+s, "-")) }

	eq := [][]string{{"time", "equity"}}
	for _, p := range r.Equity {
		eq = append(eq, []string{p.Time.Format(time.RFC3339), p.Equity.StringFixed(4)})
	}
	if err := writeCSV(name("equity.csv"), eq); err != nil {
		return err
	}

	tr := [][]string{{"symbol", "side", "qty", "entry", "exit", "opened", "closed", "gross_pnl", "fees", "funding", "net_pnl", "exit_reason"}}
	for _, t := range r.Trades {
		tr = append(tr, []string{
			t.Symbol, string(t.Side), t.Quantity.String(), t.EntryPrice.String(), t.ExitPrice.String(),
			t.OpenedAt.Format(time.RFC3339), t.ClosedAt.Format(time.RFC3339),
			t.GrossPnL.StringFixed(4), t.Fees.StringFixed(4), t.Funding.StringFixed(4), t.NetPnL().StringFixed(4), t.ExitReason,
		})
	}
	return writeCSV(name("trades.csv"), tr)
}

func writeCSV(path string, rows [][]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	if err := w.WriteAll(rows); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
