package managed

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

func (s *State) Sample(now time.Time) {
	at := now.UTC().Truncate(time.Hour)
	point := EquitySample{At: at, Account: s.Equity(), Instances: map[string]decimal.Decimal{}}
	for id := range s.Instances {
		point.Instances[id] = s.InstanceEquity(id)
	}
	if n := len(s.Samples); n > 0 && s.Samples[n-1].At.Equal(at) {
		s.Samples[n-1] = point
	} else {
		s.Samples = append(s.Samples, point)
	}
	if len(s.Samples) > 720 {
		s.Samples = s.Samples[len(s.Samples)-720:]
	}
}

func (s *State) Unrealized(owner string) decimal.Decimal {
	v := s.Inventory(owner)
	for _, h := range s.Holdings {
		if owner == "" || owner == h.Owner {
			v = v.Sub(h.Cost)
		}
	}
	return v
}

func drawdown(equity, peak decimal.Decimal) string {
	if !peak.IsPositive() {
		return "0.00%"
	}
	return decimal.Max(peak.Sub(equity).Div(peak).Mul(decimal.NewFromInt(100)), decimal.Zero).StringFixed(2) + "%"
}

// chartPath contains only coordinates; templates still escape its attribute.
func chartPath(points []EquitySample) string {
	if len(points) < 2 {
		return ""
	}
	lo, hi := points[0].Account, points[0].Account
	for _, p := range points {
		lo = decimal.Min(lo, p.Account)
		hi = decimal.Max(hi, p.Account)
	}
	span := hi.Sub(lo)
	if span.IsZero() {
		span = decimal.NewFromInt(1)
	}
	var out strings.Builder
	for i, p := range points {
		x := float64(i)*980/float64(len(points)-1) + 10
		y, _ := p.Account.Sub(lo).Div(span).Mul(decimal.NewFromInt(160)).Float64()
		verb := "L"
		if i == 0 {
			verb = "M"
		}
		fmt.Fprintf(&out, "%s%.2f %.2f ", verb, x, 180-y)
	}
	return out.String()
}
