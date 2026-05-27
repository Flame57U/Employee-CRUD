package quant

import (
	"testing"
	"time"
)

func makeBars(n int, startUnix int64, priceFn func(i int) float64) []Bar {
	out := make([]Bar, n)
	for i := 0; i < n; i++ {
		ts := startUnix + int64(i)*86400 // 1 bar per day
		p := priceFn(i)
		out[i] = Bar{OpenTime: ts, Open: p, High: p, Low: p, Close: p, Volume: 1}
	}
	return out
}

func TestSimulateGhostDCAFlatPrice(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	bars := makeBars(365, start, func(i int) float64 { return 100.0 })
	res := SimulateGhostDCA(bars, GhostDCAConfig{InitialCapital: 10_000, MonthlyInject: 1000})

	// Flat price → final equity = initial + sum of injects (within rounding).
	if res.FinalEquity <= 10_000 {
		t.Fatalf("FinalEquity not growing with injects, got %f", res.FinalEquity)
	}
	if res.TotalInjected <= 10_000 {
		t.Fatalf("TotalInjected should exceed initial capital after monthly injects, got %f", res.TotalInjected)
	}
	// Flat price → MaxDrawdown == 0
	if res.MaxDrawdown > 1e-9 {
		t.Fatalf("expected zero drawdown at flat price, got %f", res.MaxDrawdown)
	}
}

func TestMaxDrawdown(t *testing.T) {
	nav := []float64{100, 110, 121, 90, 95, 80, 130}
	// Peak 121, trough 80 → DD = 41/121
	dd := MaxDrawdown(nav)
	want := (121.0 - 80.0) / 121.0
	if abs(dd-want) > 1e-9 {
		t.Fatalf("MaxDrawdown = %f, want %f", dd, want)
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
