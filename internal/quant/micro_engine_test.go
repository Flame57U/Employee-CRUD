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

// flatCloses returns n bars at a constant price; StdDev collapses to 0 so the
// signal magnitude is governed entirely by CurrentPrice vs the (flat) anchor
// and the SigmaFloor — handy for deterministic sign/threshold assertions.
func flatCloses(n int, price float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = price
	}
	return out
}

// TestMicroDecisionNegativeSignalRaisesTarget: a price below the anchor is a
// negative (buy-pressure / oversold) signal and must push the target weight
// above the 0.5 neutral point.
func TestMicroDecisionNegativeSignalRaisesTarget(t *testing.T) {
	in := MicroEngineInput{
		Closes:         flatCloses(200, 100.0),
		CurrentPrice:   95.0, // price below flat EMA → negative signal
		CurrentWeight:  0.5,
		TotalEquity:    100_000,
		Params:         ClampChromosome(DefaultSeedChromosome),
		BetaMultiplier: 1.0,
	}
	out := ComputeMicroDecision(in)
	if out.Signal >= 0 {
		t.Fatalf("expected negative signal, got %f", out.Signal)
	}
	if out.TargetWeight <= 0.5 {
		t.Fatalf("expected target > 0.5 for negative signal, got %f", out.TargetWeight)
	}
}

// TestMicroDecisionNeutralPointZeroBias: when the position sits exactly at the
// neutral point (CurrentWeight = 0.5) the inventory bias is zero, so even with
// Gamma > 0 a zero signal must yield TargetWeight == 0.5 exactly.
func TestMicroDecisionNeutralPointZeroBias(t *testing.T) {
	chr := ClampChromosome(DefaultSeedChromosome)
	if chr.Gamma <= 0 {
		t.Fatalf("test precondition: Gamma must be > 0, got %f", chr.Gamma)
	}
	in := MicroEngineInput{
		Closes:         flatCloses(200, 100.0),
		CurrentPrice:   100.0, // equal to anchor → signal = 0
		CurrentWeight:  0.5,   // inventory bias = 0
		TotalEquity:    100_000,
		Params:         chr,
		BetaMultiplier: 1.0,
	}
	out := ComputeMicroDecision(in)
	if out.Signal != 0 {
		t.Fatalf("expected exactly zero signal, got %v", out.Signal)
	}
	if out.TargetWeight != 0.5 {
		t.Fatalf("at neutral point with zero signal, target must be exactly 0.5, got %v", out.TargetWeight)
	}
}

// TestMicroDecisionDeterministic: ComputeMicroDecision is a pure function, so
// two calls with identical input must produce byte-identical output.
func TestMicroDecisionDeterministic(t *testing.T) {
	in := MicroEngineInput{
		Closes:         sineCloses(200, 100, 7, 23),
		CurrentPrice:   103.5,
		CurrentWeight:  0.42,
		TotalEquity:    87_654,
		Params:         ClampChromosome(DefaultSeedChromosome),
		BetaMultiplier: 1.3,
		IsQuiet:        false,
	}
	a := ComputeMicroDecision(in)
	b := ComputeMicroDecision(in)
	if a != b {
		t.Fatalf("non-deterministic output:\n a = %+v\n b = %+v", a, b)
	}
}

// wedgeChromosome tunes the engine so a small total equity yields a theoretical
// order strictly inside the wedge (|TheoreticalCNY| < MicroDustCNY = 100) while
// the weight delta is large enough to satisfy the breakthrough condition.
func wedgeChromosome() Chromosome {
	chr := DefaultSeedChromosome
	chr.SigmaFloor = 0.01 // keeps the z-score signal at ±1.0 (un-clipped)
	chr.Beta = 2.0
	chr.Gamma = 1.0
	chr.MicroDustCNY = 100 // floor of the hard bounds → ±100 breakthrough order
	chr.DeltaWeightWedge = 0.01
	chr.VolRatioWedge = 1.5
	return ClampChromosome(chr)
}

// TestMicroDecisionQuietBlocksSubDust: with IsQuiet=true, a theoretical order
// inside the wedge (|TheoreticalCNY| < 100) must be suppressed to zero.
func TestMicroDecisionQuietBlocksSubDust(t *testing.T) {
	in := MicroEngineInput{
		Closes:         flatCloses(200, 100.0),
		CurrentPrice:   101.0, // small positive signal → target below 0.5
		CurrentWeight:  0.5,
		TotalEquity:    200, // tiny equity → |TheoreticalCNY| well under 100
		Params:         wedgeChromosome(),
		BetaMultiplier: 1.0,
		IsQuiet:        true,
	}
	out := ComputeMicroDecision(in)
	if math.Abs(out.TheoreticalCNY) >= 100 {
		t.Fatalf("test precondition: |TheoreticalCNY| must be < 100, got %f", out.TheoreticalCNY)
	}
	if out.OrderCNY != 0 {
		t.Fatalf("quiet sub-dust order must be zero, got %f", out.OrderCNY)
	}
}

// TestMicroDecisionWedgeBreakthroughEmitsFloor: when IsQuiet=false and the
// breakthrough condition holds, a sub-dust theoretical order must be lifted to
// exactly ±MicroDustCNY (±100), signed by the direction of the order.
func TestMicroDecisionWedgeBreakthroughEmitsFloor(t *testing.T) {
	chr := wedgeChromosome()

	// Sell side: price above anchor → positive signal → target < 0.5 →
	// negative theoretical order → OrderCNY must be -100.
	sell := ComputeMicroDecision(MicroEngineInput{
		Closes:         flatCloses(200, 100.0),
		CurrentPrice:   101.0,
		CurrentWeight:  0.5,
		TotalEquity:    200,
		Params:         chr,
		BetaMultiplier: 1.0,
		IsQuiet:        false,
	})
	if sell.TheoreticalCNY >= 0 || math.Abs(sell.TheoreticalCNY) >= 100 {
		t.Fatalf("sell precondition failed: TheoreticalCNY=%f", sell.TheoreticalCNY)
	}
	if sell.OrderCNY != -100 {
		t.Fatalf("sell breakthrough must emit -100, got %f", sell.OrderCNY)
	}

	// Buy side: price below anchor → negative signal → target > 0.5 →
	// positive theoretical order → OrderCNY must be +100.
	buy := ComputeMicroDecision(MicroEngineInput{
		Closes:         flatCloses(200, 100.0),
		CurrentPrice:   99.0,
		CurrentWeight:  0.5,
		TotalEquity:    200,
		Params:         chr,
		BetaMultiplier: 1.0,
		IsQuiet:        false,
	})
	if buy.TheoreticalCNY <= 0 || math.Abs(buy.TheoreticalCNY) >= 100 {
		t.Fatalf("buy precondition failed: TheoreticalCNY=%f", buy.TheoreticalCNY)
	}
	if buy.OrderCNY != 100 {
		t.Fatalf("buy breakthrough must emit +100, got %f", buy.OrderCNY)
	}
}
