package quant

import "time"

// GhostDCAConfig is the configuration for the passive DCA baseline simulator.
//
// SimulateGhostDCA invests InitialCapital fully into the first close, then
// injects MonthlyInject CNY at the start of every subsequent calendar month.
// Used by GA fitness evaluation as the control-arm.
type GhostDCAConfig struct {
	InitialCapital float64
	MonthlyInject  float64
}

// GhostDCAResult is the audit-grade output of the baseline simulation.
type GhostDCAResult struct {
	FinalEquity   float64
	TotalInjected float64
	MaxDrawdown   float64
	ROI           float64 // Modified Dietz
	NAV           []float64
	Shares        float64
}

// SimulateGhostDCA runs the passive DCA baseline against a Bar slice (must be
// time-ordered, ascending).
func SimulateGhostDCA(bars []Bar, cfg GhostDCAConfig) GhostDCAResult {
	if len(bars) == 0 {
		return GhostDCAResult{}
	}

	type flow struct {
		Day    int     // 0-based day index from start
		Amount float64 // CNY injected (excludes initial capital)
	}
	var flows []flow

	startTime := time.Unix(bars[0].OpenTime, 0).UTC()
	startDay := startTime.YearDay() + startTime.Year()*1000 // monotonic day index

	// Initial buy with full InitialCapital at the first close.
	firstPrice := bars[0].Close
	if firstPrice <= 0 {
		return GhostDCAResult{}
	}
	shares := cfg.InitialCapital / firstPrice
	cash := 0.0
	totalInjected := cfg.InitialCapital

	nav := make([]float64, len(bars))
	nav[0] = shares*firstPrice + cash

	lastInjectMonth := startTime.Month()
	lastInjectYear := startTime.Year()

	for i := 1; i < len(bars); i++ {
		bar := bars[i]
		barTime := time.Unix(bar.OpenTime, 0).UTC()

		// Monthly inject on the first bar of any month after start.
		if (barTime.Year() != lastInjectYear || barTime.Month() != lastInjectMonth) && cfg.MonthlyInject > 0 {
			if bar.Close > 0 {
				shares += cfg.MonthlyInject / bar.Close
				totalInjected += cfg.MonthlyInject
				dayIdx := barTime.YearDay() + barTime.Year()*1000 - startDay
				flows = append(flows, flow{Day: dayIdx, Amount: cfg.MonthlyInject})
			}
			lastInjectMonth = barTime.Month()
			lastInjectYear = barTime.Year()
		}

		nav[i] = shares*bar.Close + cash
	}

	finalEquity := nav[len(nav)-1]
	maxDD := MaxDrawdown(nav)

	// Modified Dietz ROI.
	endTime := time.Unix(bars[len(bars)-1].OpenTime, 0).UTC()
	totalDays := int(endTime.Sub(startTime).Hours()/24) + 1
	if totalDays < 1 {
		totalDays = 1
	}
	beginEquity := cfg.InitialCapital
	weightedFlows := 0.0
	cashflowSum := 0.0
	for _, f := range flows {
		w := float64(totalDays-f.Day) / float64(totalDays)
		weightedFlows += f.Amount * w
		cashflowSum += f.Amount
	}
	denom := beginEquity + weightedFlows
	roi := 0.0
	if denom > 0 {
		roi = (finalEquity - beginEquity - cashflowSum) / denom
	}

	return GhostDCAResult{
		FinalEquity:   finalEquity,
		TotalInjected: totalInjected,
		MaxDrawdown:   maxDD,
		ROI:           roi,
		NAV:           nav,
		Shares:        shares,
	}
}

// MaxDrawdown computes peak-to-trough relative drawdown over an NAV curve.
// Returns a non-negative magnitude (0.20 = 20% drawdown).
func MaxDrawdown(nav []float64) float64 {
	if len(nav) == 0 {
		return 0
	}
	peak := nav[0]
	maxDD := 0.0
	for _, v := range nav {
		if v > peak {
			peak = v
		}
		if peak > 0 {
			dd := (peak - v) / peak
			if dd > maxDD {
				maxDD = dd
			}
		}
	}
	return maxDD
}
