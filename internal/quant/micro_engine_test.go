package quant

import (
	"math"
	"testing"
)

// generate a long enough close series with a deterministic shape.
func sineCloses(n int, base, amp, period float64) []float64 {
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		out[i] = base + amp*math.Sin(2*math.Pi*float64(i)/period)
	}
	return out
}

func TestMicroDecisionNeutralSignalReturnsCenter(t *testing.T) {
	closes := make([]float64, 200)
	for i := range closes {
		closes[i] = 100.0
	}
	in := MicroEngineInput{
		Closes:         closes,
		CurrentPrice:   100.0,
		CurrentWeight:  0.5,
		TotalEquity:    100_000,
		Params:         ClampChromosome(DefaultSeedChromosome),
		BetaMultiplier: 1.0,
		IsQuiet:        false,
	}
	out := ComputeMicroDecision(in)
	// At flat prices, signal ≈ 0, inventory bias = 0, so target ≈ 0.5
	if math.Abs(out.TargetWeight-0.5) > 0.01 {
		t.Fatalf("expected target ~ 0.5, got %f", out.TargetWeight)
	}
}

func TestMicroDecisionPositiveSignalLowersTarget(t *testing.T) {
	// Price spike above EMA → positive signal → target < 0.5
	closes := make([]float64, 200)
	for i := range closes {
		closes[i] = 100.0
	}
	in := MicroEngineInput{
		Closes:         closes,
		CurrentPrice:   105.0, // price above flat EMA
		CurrentWeight:  0.5,
		TotalEquity:    100_000,
		Params:         ClampChromosome(DefaultSeedChromosome),
		BetaMultiplier: 1.0,
	}
	out := ComputeMicroDecision(in)
	if out.TargetWeight >= 0.5 {
		t.Fatalf("expected target < 0.5 for positive signal, got %f", out.TargetWeight)
	}
}

func TestMicroDecisionQuietWedgeBlocksDust(t *testing.T) {
	closes := make([]float64, 200)
	for i := range closes {
		closes[i] = 100.0 + 0.001*float64(i%2) // tiny noise → small TheoreticalCNY
	}
	in := MicroEngineInput{
		Closes:         closes,
		CurrentPrice:   100.0005,
		CurrentWeight:  0.5,
		TotalEquity:    100_000,
		Params:         ClampChromosome(DefaultSeedChromosome),
		BetaMultiplier: 1.0,
		IsQuiet:        true, // quiet state → dust must zero
	}
	out := ComputeMicroDecision(in)
	if out.OrderCNY != 0 {
		t.Fatalf("quiet state must zero dust orders, got %f", out.OrderCNY)
	}
}

func TestMicroDecisionVolatilityRatioWithinBounds(t *testing.T) {
	closes := sineCloses(200, 100, 5, 30)
	in := MicroEngineInput{
		Closes:         closes,
		CurrentPrice:   closes[len(closes)-1],
		CurrentWeight:  0.5,
		TotalEquity:    100_000,
		Params:         ClampChromosome(DefaultSeedChromosome),
		BetaMultiplier: 1.0,
	}
	out := ComputeMicroDecision(in)
	if out.VolatilityRatio < MicroVolRatioMin || out.VolatilityRatio > MicroVolRatioMax {
		t.Fatalf("VolatilityRatio out of [%f, %f]: %f", MicroVolRatioMin, MicroVolRatioMax, out.VolatilityRatio)
	}
}
