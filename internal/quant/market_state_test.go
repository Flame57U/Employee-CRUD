package quant

import "testing"

func TestComputeMarketStateBullWithRisingTrend(t *testing.T) {
	closes := make([]float64, 300)
	for i := range closes {
		closes[i] = 100.0 + float64(i)*0.5 // steady uptrend
	}
	ms := ComputeMarketState(MarketStateInput{
		Closes: closes,
		Params: ClampChromosome(DefaultSeedChromosome),
	})
	if ms.State != MarketBull {
		t.Fatalf("expected BULL, got %s", ms.State)
	}
	if ms.IsQuiet {
		t.Fatal("uptrend should not be QUIET")
	}
}

func TestComputeMarketStateBearWithDowntrend(t *testing.T) {
	closes := make([]float64, 300)
	for i := range closes {
		closes[i] = 200.0 - float64(i)*0.5 // steady downtrend
	}
	ms := ComputeMarketState(MarketStateInput{
		Closes: closes,
		Params: ClampChromosome(DefaultSeedChromosome),
	})
	if ms.State != MarketBear {
		t.Fatalf("expected BEAR, got %s", ms.State)
	}
	if ms.TimeDilationMultiplier <= 1.0 {
		t.Fatalf("BEAR should boost macro pace, got %f", ms.TimeDilationMultiplier)
	}
}

func TestComputeMarketStateQuietFlatPrice(t *testing.T) {
	closes := make([]float64, 300)
	for i := range closes {
		closes[i] = 100.0 // perfectly flat → ATR=0, dev=0
	}
	ms := ComputeMarketState(MarketStateInput{
		Closes: closes,
		Params: ClampChromosome(DefaultSeedChromosome),
	})
	if ms.State != MarketQuiet {
		t.Fatalf("expected QUIET, got %s", ms.State)
	}
	if !ms.IsQuiet {
		t.Fatal("QUIET state must set IsQuiet=true")
	}
	if ms.TimeDilationMultiplier != 0 {
		t.Fatalf("QUIET must silence macro, got TDM=%f", ms.TimeDilationMultiplier)
	}
}

func TestComputeMarketStateInsufficientDataDefaultsBull(t *testing.T) {
	ms := ComputeMarketState(MarketStateInput{
		Closes: []float64{100, 101, 102},
		Params: ClampChromosome(DefaultSeedChromosome),
	})
	if ms.State != MarketBull {
		t.Fatalf("expected BULL fallback, got %s", ms.State)
	}
}
