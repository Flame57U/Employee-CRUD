// Package backtest bridges the quant engine to historical bar data, providing
// a pure in-memory portfolio simulation for GA fitness evaluation.
package backtest

import (
	"fmt"
	"math"
	"time"

	"github.com/quantsaas/platform/internal/quant"
)

// BacktestResult holds the key PnL metrics from one simulated run.
type BacktestResult struct {
	ROI           float64
	MaxDD         float64
	FinalEquity   float64
	TotalInjected float64
}

// RunBacktest simulates the DCA+Sigmoid strategy over the eval region of window.
// Warmup bars (OpenTime < EvalStartMs) seed indicator state only; no trades occur.
// Capital is deployed at EvalStartMs; monthly injections follow calendar months.
func RunBacktest(window quant.CrucibleWindow, chr quant.Chromosome, spawn quant.SpawnPoint) BacktestResult {
	bars := window.Bars
	if len(bars) == 0 {
		return BacktestResult{}
	}

	feeRate := spawn.Risk.FeeRate
	policy := spawn.Policy

	var (
		cash          float64
		lots          []quant.SpotLot
		lotSeq        int
		runtime       quant.RuntimeState
		nav           []float64
		injs          []injectRec
		totalInjected float64
		evalBarIdx    int
		started       bool
		lastMonth     time.Month
		lastYear      int
	)

	closes := make([]float64, 0, len(bars))

	for _, bar := range bars {
		closes = append(closes, bar.Close)

		if bar.OpenTime < window.EvalStartMs {
			continue
		}

		price := bar.Close
		if price <= 0 {
			continue
		}
		barTime := time.Unix(bar.OpenTime, 0).UTC()

		if !started {
			cash = policy.TotalCapitalCNY
			totalInjected = policy.TotalCapitalCNY
			lastMonth = barTime.Month()
			lastYear = barTime.Year()
			started = true
		} else if barTime.Year() != lastYear || barTime.Month() != lastMonth {
			if policy.MonthlyInjectCNY > 0 {
				cash += policy.MonthlyInjectCNY
				totalInjected += policy.MonthlyInjectCNY
				injs = append(injs, injectRec{bar: evalBarIdx, amount: policy.MonthlyInjectCNY})
			}
			lastMonth = barTime.Month()
			lastYear = barTime.Year()
		}

		totalShares := allShares(lots)
		totalEquity := cash + totalShares*price
		if totalEquity <= 0 {
			nav = append(nav, 0)
			evalBarIdx++
			runtime.TicksSinceLastMacro++
			continue
		}

		ms := quant.ComputeMarketState(quant.MarketStateInput{Closes: closes, Params: chr})

		// Periodic soft release (~monthly cadence)
		if evalBarIdx > 0 && evalBarIdx%30 == 0 {
			floatW := quant.FloatHoldTotal(lots) * price / totalEquity
			if floatW < 0.5 {
				sellGap := (0.5 - floatW) * totalEquity / price
				ageMonths := chr.ReleaseMinHoldDays / 30
				if ageMonths < 1 {
					ageMonths = 1
				}
				lots, _ = quant.SoftRelease(lots, quant.SoftReleaseConfig{
					NowUnix:         bar.OpenTime,
					MinAgeMonths:    ageMonths,
					MaxReleaseRatio: chr.WMicroReleaseCap,
					SellGap:         sellGap,
				})
			}
		}

		// Recompute after release
		totalShares = allShares(lots)
		totalEquity = cash + totalShares*price
		spendable := cash - chr.MacroReservePct*totalEquity
		if spendable < 0 {
			spendable = 0
		}

		macroDec := quant.ComputeMacroDecision(quant.MacroDecisionInput{
			Closes:       closes,
			CurrentPrice: price,
			TotalEquity:  totalEquity,
			SpendableCNY: spendable,
			State:        ms,
			Runtime:      runtime,
			Params:       chr,
			Spawn:        spawn,
			Symbol:       policy.Symbol,
		})

		if macroDec.Triggered && macroDec.OrderCNY > 0 {
			spend := math.Min(macroDec.OrderCNY, cash)
			net := spend / (1 + feeRate)
			if net > 0 {
				lotSeq++
				lots = append(lots, quant.SpotLot{
					LotID:     fmt.Sprintf("m%d", lotSeq),
					LotType:   quant.LotDeadStack,
					Amount:    net / price,
					CostPrice: price,
					CreatedAt: bar.OpenTime,
				})
				cash -= spend
			}
			runtime.TicksSinceLastMacro = 0
			runtime.LastMacroTimestamp = bar.OpenTime
		} else {
			runtime.TicksSinceLastMacro++
		}

		// Recompute for micro engine
		totalShares = allShares(lots)
		totalEquity = cash + totalShares*price
		floatShares := quant.FloatHoldTotal(lots)
		currentMicroWeight := 0.0
		if totalEquity > 0 {
			currentMicroWeight = floatShares * price / totalEquity
		}

		microOut := quant.ComputeMicroDecision(quant.MicroEngineInput{
			Closes:         closes,
			CurrentPrice:   price,
			CurrentWeight:  currentMicroWeight,
			TotalEquity:    totalEquity,
			Params:         chr,
			BetaMultiplier: ms.BetaMultiplier,
			IsQuiet:        ms.IsQuiet,
		})

		switch {
		case microOut.OrderCNY > 0 && cash >= microOut.OrderCNY:
			spend := math.Min(microOut.OrderCNY, cash)
			net := spend / (1 + feeRate)
			if net > 0 {
				lotSeq++
				lots = append(lots, quant.SpotLot{
					LotID:     fmt.Sprintf("f%d", lotSeq),
					LotType:   quant.LotFloating,
					Amount:    net / price,
					CostPrice: price,
					CreatedAt: bar.OpenTime,
				})
				cash -= spend
			}
		case microOut.OrderCNY < 0:
			needShares := (-microOut.OrderCNY) / price
			avail := quant.FloatHoldTotal(lots)
			if avail < needShares {
				lots, _, _ = quant.HardRelease(lots, needShares-avail, bar.OpenTime)
			}
			toSell := math.Min(needShares, quant.FloatHoldTotal(lots))
			if toSell > 0 {
				lots = sellFloat(lots, toSell)
				cash += toSell * price * (1 - feeRate)
			}
		}

		totalShares = allShares(lots)
		nav = append(nav, cash+totalShares*price)
		evalBarIdx++
	}

	if len(nav) == 0 {
		return BacktestResult{}
	}

	finalEquity := nav[len(nav)-1]
	maxDD := quant.MaxDrawdown(nav)
	roi := modifiedDietz(policy.TotalCapitalCNY, finalEquity, injs, len(nav))

	return BacktestResult{
		ROI:           roi,
		MaxDD:         maxDD,
		FinalEquity:   finalEquity,
		TotalInjected: totalInjected,
	}
}

type injectRec struct {
	bar    int
	amount float64
}

func allShares(lots []quant.SpotLot) float64 {
	return quant.DeadHoldTotal(lots) + quant.FloatHoldTotal(lots) + quant.ColdSealedHoldTotal(lots)
}

// sellFloat removes qty shares from FLOATING lots in FIFO order.
func sellFloat(lots []quant.SpotLot, qty float64) []quant.SpotLot {
	out := make([]quant.SpotLot, 0, len(lots))
	rem := qty
	for _, lot := range lots {
		if rem <= 0 || lot.LotType != quant.LotFloating {
			out = append(out, lot)
			continue
		}
		if lot.Amount <= rem {
			rem -= lot.Amount
		} else {
			lot.Amount -= rem
			rem = 0
			out = append(out, lot)
		}
	}
	return out
}

// modifiedDietz computes Modified Dietz ROI over the eval period.
func modifiedDietz(beginEquity, finalEquity float64, injs []injectRec, totalBars int) float64 {
	cashflowSum := 0.0
	weighted := 0.0
	for _, inj := range injs {
		w := float64(totalBars-inj.bar) / float64(totalBars)
		cashflowSum += inj.amount
		weighted += inj.amount * w
	}
	denom := beginEquity + weighted
	if denom <= 0 {
		return 0
	}
	return (finalEquity - beginEquity - cashflowSum) / denom
}
