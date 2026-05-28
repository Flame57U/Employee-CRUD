package backtest

import (
	"math"
	"testing"
	"time"

	"github.com/quantsaas/platform/internal/quant"
)

// syntheticWindow builds a deterministic CrucibleWindow: a trending price path
// with a sinusoidal ripple, daily bars, and a warmup prefix before EvalStartMs.
// The shape is chosen so both the macro (DCA) and micro (Sigmoid) engines have
// reason to act, exercising most of the RunBacktest code paths.
func syntheticWindow(nBars, warmup int) quant.CrucibleWindow {
	start := time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	const day = int64(86400)

	bars := make([]quant.Bar, nBars)
	for i := 0; i < nBars; i++ {
		// Drift upward with a multi-scale ripple; strictly positive prices.
		base := 100.0 + 0.05*float64(i)
		ripple := 6*math.Sin(2*math.Pi*float64(i)/40) + 2.5*math.Sin(2*math.Pi*float64(i)/11)
		price := base + ripple
		t := start + int64(i)*day
		bars[i] = quant.Bar{
			OpenTime: t,
			Open:     price,
			High:     price + 1,
			Low:      price - 1,
			Close:    price,
			Volume:   1000,
		}
	}

	return quant.CrucibleWindow{
		Label:       "synthetic",
		Weight:      1.0,
		Bars:        bars,
		EvalStartMs: bars[warmup].OpenTime,
	}
}

func syntheticSpawn() quant.SpawnPoint {
	return quant.SpawnPoint{
		Policy: quant.Policy{
			Symbol:           "510300.SH",
			AssetClass:       "A_STOCK_ETF",
			TotalCapitalCNY:  100_000,
			MonthlyInjectCNY: 2_000,
			MacroMinOrderCNY: 200,
		},
		Risk: quant.Risk{FeeRate: 0.0001},
	}
}

// TestRunBacktestDeterministic asserts that RunBacktest is a pure function:
// identical inputs must yield byte-identical results across repeated runs. This
// is the foundational guarantee that backtest and live share one code path with
// no hidden state (no rng, no wall-clock, no map-ordering leakage).
func TestRunBacktestDeterministic(t *testing.T) {
	window := syntheticWindow(600, 200)
	chr := quant.ClampChromosome(quant.DefaultSeedChromosome)
	spawn := syntheticSpawn()

	first := RunBacktest(window, chr, spawn)
	second := RunBacktest(window, chr, spawn)

	if first != second {
		t.Fatalf("RunBacktest is non-deterministic:\n first  = %+v\n second = %+v", first, second)
	}

	// Sanity: the run must have actually deployed capital, otherwise the
	// determinism check is vacuous (two empty results trivially match).
	if first.TotalInjected <= 0 {
		t.Fatalf("expected capital to be deployed, got TotalInjected=%f", first.TotalInjected)
	}
	if first.FinalEquity <= 0 {
		t.Fatalf("expected positive final equity, got %f", first.FinalEquity)
	}
}

// TestRunBacktestDeterministicAcrossRuns hammers the same inputs many times to
// flush out any non-determinism that only surfaces intermittently (e.g. from
// concurrent map access were it ever introduced).
func TestRunBacktestDeterministicAcrossRuns(t *testing.T) {
	window := syntheticWindow(500, 180)
	chr := quant.ClampChromosome(quant.DefaultSeedChromosome)
	spawn := syntheticSpawn()

	want := RunBacktest(window, chr, spawn)
	for i := 0; i < 25; i++ {
		got := RunBacktest(window, chr, spawn)
		if got != want {
			t.Fatalf("run %d diverged:\n want = %+v\n got  = %+v", i, want, got)
		}
	}
}
