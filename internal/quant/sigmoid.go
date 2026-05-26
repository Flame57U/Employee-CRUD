package quant

import "math"

// SigmoidEngine converts a strategy signal into target weight and rebalance delta.
type SigmoidEngine struct {
	Beta                  float64
	Gamma                 float64
	MinRebalanceThreshold float64
}

// Step maps signal/current weight to target weight and rebalance delta.
func (e SigmoidEngine) Step(signal float64, currentWeight float64) (targetWeight float64, delta float64) {
	exponent := -e.Beta*signal + e.Gamma*(currentWeight-0.5)
	targetWeight = 1.0 / (1.0 + math.Exp(exponent))
	delta = targetWeight - currentWeight

	if math.Abs(delta) < e.MinRebalanceThreshold {
		delta = 0
	}

	return targetWeight, delta
}
