package quant

import "math"

// Micro-engine non-evolvable constants (Build Plan 3C).
const (
	MicroSignalEMABars     = 20  // EMA window for the signal anchor
	MicroSignalStdDevBars  = 20  // σ window for the signal anchor
	MicroVolRatioLongBars  = 60  // long-window MAVAbsChange
	MicroVolRatioShortBars = 10  // short-window MAVAbsChange
	MicroVolRatioMin       = 0.1 // VolatilityRatio clip min
	MicroVolRatioMax       = 3.0 // VolatilityRatio clip max
)

// MicroEngineInput is the bundle of inputs to the Sigmoid dynamic balance.
type MicroEngineInput struct {
	Closes          []float64
	CurrentPrice    float64
	CurrentWeight   float64 // V_float / TotalEquity
	TotalEquity     float64
	Params          Chromosome
	BetaMultiplier  float64
	IsQuiet         bool
}

// MicroEngineOutput is the audit-grade result for one micro-engine tick.
type MicroEngineOutput struct {
	TargetWeight    float64
	Signal          float64
	TheoreticalCNY  float64
	OrderCNY        float64
	VolatilityRatio float64
}

// ComputeMicroDecision implements the Sigmoid dynamic balance per Build Plan 3C.
//
// Design philosophy: Signal is the external force (market pressure);
// InventoryBias is the spring's restoring force (current position vs neutral);
// Beta is the spring stiffness (how aggressively the balance responds);
// Gamma decides whether the spring is engaged (0 = pure trend follower);
// VolatilityRatio wedge filter controls dust orders during quiet regimes
// — the system breathes wider in active markets and clamps shut in flat ones.
//
// Sign convention: positive Signal → reduce target weight (sell-pressure /
// overbought); negative Signal → increase target weight (buy-pressure /
// oversold). Inventory bias above 0.5 nudges the target downward.
func ComputeMicroDecision(in MicroEngineInput) MicroEngineOutput {
	p := in.Params
	out := MicroEngineOutput{
		VolatilityRatio: 1.0,
	}

	// Step 1: anchor EMA + σ (with sigma_floor protection).
	if len(in.Closes) < MicroSignalEMABars || len(in.Closes) < MicroSignalStdDevBars {
		return out
	}
	anchor := EMA(in.Closes, MicroSignalEMABars)
	sigma := StdDev(in.Closes, MicroSignalStdDevBars)
	if sigma < p.SigmaFloor {
		sigma = p.SigmaFloor
	}
	if sigma == 0 {
		return out
	}

	// Step 2: dimensionless signal. z-score of current price vs anchor,
	// clipped to a safe range for the sigmoid input.
	rawSignal := (in.CurrentPrice - anchor) / (anchor * sigma)
	signal := ClipFloat64(rawSignal, -5.0, 5.0)
	out.Signal = signal

	// Step 3: Sigmoid target weight with inventory-bias restoring force.
	effectiveBeta := math.Max(0.01, p.Beta*in.BetaMultiplier)
	inventoryBias := ClipFloat64(in.CurrentWeight, 0, 1) - 0.5
	exponent := effectiveBeta*signal + p.Gamma*inventoryBias
	target := 1.0 / (1.0 + math.Exp(exponent))
	target = ClipFloat64(target, 0, 1)
	out.TargetWeight = target

	// Step 4: theoretical CNY delta order.
	deltaWeight := target - in.CurrentWeight
	out.TheoreticalCNY = deltaWeight * in.TotalEquity

	// Step 5: VolatilityRatio (long window / short window of MAVAbsChange).
	volRatio := 1.0
	if len(in.Closes) >= MicroVolRatioLongBars && len(in.Closes) >= MicroVolRatioShortBars {
		longMav := MAVAbsChange(in.Closes, MicroVolRatioLongBars)
		shortMav := MAVAbsChange(in.Closes, MicroVolRatioShortBars)
		if longMav > 0 {
			volRatio = ClipFloat64(shortMav/longMav, MicroVolRatioMin, MicroVolRatioMax)
		}
	}
	out.VolatilityRatio = volRatio

	// Step 6: wedge filter.
	absTheo := math.Abs(out.TheoreticalCNY)
	switch {
	case absTheo >= p.MicroDustCNY:
		out.OrderCNY = RoundToCNY(out.TheoreticalCNY)
	case absTheo > 0:
		// Inside the wedge. Breakthrough requires non-quiet state AND either
		// a large enough weight delta OR a volatility expansion.
		breakthrough := !in.IsQuiet &&
			(math.Abs(deltaWeight) > p.DeltaWeightWedge ||
				volRatio > p.VolRatioWedge)
		if breakthrough {
			sign := 1.0
			if out.TheoreticalCNY < 0 {
				sign = -1.0
			}
			out.OrderCNY = RoundToCNY(sign * p.MicroDustCNY)
		} else {
			out.OrderCNY = 0
		}
	default:
		out.OrderCNY = 0
	}

	return out
}
