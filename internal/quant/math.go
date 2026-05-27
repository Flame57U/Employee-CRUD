package quant

import "math"

// EMA computes the exponential moving average over the last `period` samples of
// `series` and returns the final smoothed value. Returns 0 if inputs are
// insufficient.
func EMA(series []float64, period int) float64 {
	if period <= 0 || len(series) < period {
		return 0
	}
	alpha := 2.0 / (float64(period) + 1.0)
	ema := series[len(series)-period]
	for i := len(series) - period + 1; i < len(series); i++ {
		ema = alpha*series[i] + (1-alpha)*ema
	}
	return ema
}

// StdDev returns the sample standard deviation of the last `period` samples of
// `series`. Returns 0 when insufficient data.
func StdDev(series []float64, period int) float64 {
	if period <= 1 || len(series) < period {
		return 0
	}
	window := series[len(series)-period:]
	var sum float64
	for _, v := range window {
		sum += v
	}
	mean := sum / float64(period)
	var sq float64
	for _, v := range window {
		d := v - mean
		sq += d * d
	}
	return math.Sqrt(sq / float64(period-1))
}

// MAVAbsChange returns the mean absolute period-to-period change of the last L
// closes: sum(|c_i - c_{i-1}|) / (L-1). Insufficient data → 0.
func MAVAbsChange(series []float64, L int) float64 {
	if L < 2 || len(series) < L {
		return 0
	}
	window := series[len(series)-L:]
	var sum float64
	for i := 1; i < L; i++ {
		sum += math.Abs(window[i] - window[i-1])
	}
	return sum / float64(L-1)
}

// ClipFloat64 clamps v into the inclusive range [lo, hi].
func ClipFloat64(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// RoundToCNY rounds to two decimal places using banker-safe half-away-from-zero.
func RoundToCNY(v float64) float64 {
	if v >= 0 {
		return math.Floor(v*100+0.5) / 100
	}
	return -math.Floor(-v*100+0.5) / 100
}
