package quant

// CrucibleWarmupDays is the default indicator pre-heat prefix length in days.
const CrucibleWarmupDays = 1200

// CrucibleWindow is one multi-period evaluation slice with its warmup prefix.
// Bars are ordered ascending by OpenTime and span from warmup start to the
// latest available bar. Only bars with OpenTime >= EvalStartMs count toward PnL.
type CrucibleWindow struct {
	Label       string  // "6m" | "2y" | "5y" | "full"
	Weight      float64 // fitness weight for ScoreTotal aggregation
	Bars        []Bar   // warmup prefix + eval region, ascending
	EvalStartMs int64   // unix seconds of first eval bar
}

// CrucibleResult holds the scored outcome for one window after Evaluate.
type CrucibleResult struct {
	Window string
	Score  float64
	ROI    float64
	MaxDD  float64
	Alpha  float64
}

// BuildCrucibleWindows slices bars into four evaluation windows ordered
// shortest-to-longest (6m → 2y → 5y → full) for cascading short-circuit.
// Each window carries a warmup prefix of warmupDays days before the eval region.
func BuildCrucibleWindows(bars []Bar, warmupDays int) []CrucibleWindow {
	if len(bars) == 0 {
		return nil
	}
	latest := bars[len(bars)-1].OpenTime
	first := bars[0].OpenTime
	warmupSec := int64(warmupDays) * secondsPerDay

	// lowerBound returns the first index i where bars[i].OpenTime >= ts.
	lowerBound := func(ts int64) int {
		lo, hi := 0, len(bars)
		for lo < hi {
			mid := (lo + hi) >> 1
			if bars[mid].OpenTime < ts {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		return lo
	}

	type spec struct {
		label    string
		weight   float64
		evalDays int64 // 0 = full dataset
	}
	specs := []spec{
		{"6m", 0.10, 183},
		{"2y", 0.20, 730},
		{"5y", 0.30, 1825},
		{"full", 0.40, 0},
	}

	out := make([]CrucibleWindow, 0, 4)
	for _, s := range specs {
		evalStart := first
		if s.evalDays > 0 {
			evalStart = latest - s.evalDays*secondsPerDay
			if evalStart < first {
				evalStart = first
			}
		}
		startIdx := lowerBound(evalStart - warmupSec)
		evalIdx := lowerBound(evalStart)
		if evalIdx >= len(bars) {
			continue
		}
		out = append(out, CrucibleWindow{
			Label:       s.label,
			Weight:      s.weight,
			Bars:        bars[startIdx:],
			EvalStartMs: bars[evalIdx].OpenTime,
		})
	}
	return out
}
