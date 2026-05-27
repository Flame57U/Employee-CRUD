package quant

import "math"

// Market state enumeration (doc Ch.2).
const (
	MarketBull  = "BULL"
	MarketBear  = "BEAR"
	MarketQuiet = "QUIET"
)

// MarketState is the audit-grade output of the market-state sensing layer.
//
// The struct fields below are the contract surface consumed by the macro and
// micro engines and MUST NOT be renamed.
type MarketState struct {
	State                  string
	TimeDilationMultiplier float64 // macro pacing multiplier (>=1 expands DCA cadence)
	BetaMultiplier         float64 // micro sigmoid responsiveness multiplier
	IsQuiet                bool    // QUIET → micro dust orders forced to zero
	// Audit fields (not part of engine contract)
	EMAFast float64
	EMAMid  float64
	EMASlow float64
	AtrN    float64
}

// MarketStateInput is the bundle of priors needed to classify the market.
type MarketStateInput struct {
	Closes []float64
	Params Chromosome
}

// ComputeMarketState classifies the current tick into BULL / BEAR / QUIET
// using the EMA-fast / EMA-mid / EMA-slow trend trio plus normalised ATR.
// Implementation follows doc §2.3 priority order: QUIET → BEAR → BULL fallback.
func ComputeMarketState(in MarketStateInput) MarketState {
	closes := in.Closes
	p := in.Params
	ms := MarketState{
		State:                  MarketBull,
		TimeDilationMultiplier: 1.0,
		BetaMultiplier:         1.0,
		IsQuiet:                false,
	}

	if len(closes) < p.NEMASlow+1 {
		// Insufficient data: default to BULL with neutral multipliers.
		return ms
	}

	ms.EMAFast = EMA(closes, p.NEMAFast)
	ms.EMAMid = EMA(closes, p.NEMAMid)
	ms.EMASlow = EMA(closes, p.NEMASlow)
	ms.AtrN = normalisedATR(closes, p.NAtr)

	priceLast := closes[len(closes)-1]
	priceDevFromMid := math.Abs(priceLast-ms.EMAMid) / ms.EMAMid

	// ① QUIET (priority)
	if ms.AtrN < p.AtrQuietThreshold && priceDevFromMid < p.QuietBand {
		ms.State = MarketQuiet
		ms.IsQuiet = true
		ms.TimeDilationMultiplier = 0.0 // macro silent
		ms.BetaMultiplier = 1.0
		return ms
	}

	// ② BEAR
	if priceLast < ms.EMASlow && ms.EMAFast < ms.EMAMid {
		ms.State = MarketBear
		ms.TimeDilationMultiplier = p.BearMacroBoost // accelerate macro pace
		ms.BetaMultiplier = 1.0                      // sigmoid scaling handled by bear_micro_scale layer
		return ms
	}

	// ③ BULL (fallback)
	return ms
}

// normalisedATR is the EMA of |Δclose|/close_prev over period N.
func normalisedATR(closes []float64, period int) float64 {
	if period <= 0 || len(closes) < period+1 {
		return 0
	}
	rel := make([]float64, len(closes)-1)
	for i := 1; i < len(closes); i++ {
		if closes[i-1] == 0 {
			rel[i-1] = 0
			continue
		}
		rel[i-1] = math.Abs(closes[i]-closes[i-1]) / closes[i-1]
	}
	return EMA(rel, period)
}
