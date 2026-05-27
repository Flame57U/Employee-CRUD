package quant

// ExtractCloses returns the close-price series from a Bar slice.
// Spot strategies (IsSpot=true) must use this ACL helper rather than reaching
// into Bar fields directly from inside the strategy kernel.
func ExtractCloses(bars []Bar) []float64 {
	out := make([]float64, len(bars))
	for i, b := range bars {
		out[i] = b.Close
	}
	return out
}

// ExtractTimestamps returns the OpenTime series from a Bar slice.
func ExtractTimestamps(bars []Bar) []int64 {
	out := make([]int64, len(bars))
	for i, b := range bars {
		out[i] = b.OpenTime
	}
	return out
}
