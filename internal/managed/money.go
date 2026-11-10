package managed

import (
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
)

// Fiat returns the USD and IDR value of one USDT; ok is false until rates are known.
type Fiat func() (usd, idr decimal.Decimal, ok bool)

// idNumber formats d with Indonesian separators: 1.234,56.
func idNumber(d decimal.Decimal, places int32) string {
	s := d.Abs().StringFixed(places)
	whole, frac, _ := strings.Cut(s, ".")
	var b strings.Builder
	if d.Round(places).IsNegative() {
		b.WriteByte('-')
	}
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	if frac != "" {
		b.WriteString("," + frac)
	}
	return b.String()
}

// usdtText is the default amount format of the dashboard.
func usdtText(d decimal.Decimal) string { return idNumber(d, 2) + " USDT" }

// FiatText is the small USD and IDR line under an amount.
func (v view) FiatText(d decimal.Decimal) (usd, idr string, ok bool) {
	if v.fiat == nil {
		return "", "", false
	}
	rUSD, rIDR, ok := v.fiat()
	if !ok {
		return "", "", false
	}
	return "≈ $" + idNumber(d.Mul(rUSD), 2), "≈ Rp " + idNumber(d.Mul(rIDR), 0), true
}

// slug turns a display name into an ID: lowercase letters and digits joined
// by '-', at most 40 characters; fallback when nothing is left.
func slug(name, fallback string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			dash = false
		default:
			dash = true
		}
		if b.Len() >= 40 {
			break
		}
	}
	if b.Len() == 0 {
		return fallback
	}
	return strings.TrimRight(b.String()[:min(b.Len(), 40)], "-")
}

// uniqueID adds -2, -3, ... until taken reports the ID free.
func uniqueID(base string, taken func(string) bool) string {
	id := base
	for n := 2; taken(id); n++ {
		id = base + "-" + strconv.Itoa(n)
	}
	return id
}

// bindingBase names a strategy from its template, candle and single pair,
// e.g. pullback-4h, dca-1h, grid-1m-btc.
func bindingBase(b BindingSpec) string {
	parts := []string{b.Name}
	if mode, ok := b.Params["mode"].(string); ok && b.Name == "trendspot" {
		parts[0] = mode
	}
	if tf, ok := b.Params["timeframe"].(string); ok && b.Name != ManualStrategy {
		parts = append(parts, tf)
	}
	if len(b.Symbols) == 1 {
		parts = append(parts, strings.TrimSuffix(strings.ToLower(b.Symbols[0]), "usdt"))
	}
	return slug(strings.Join(parts, " "), "strategi")
}
