package quant

import "math"

// MacroMinOrderCNYFloor is the system-wide floor on macro-buy ticket size:
// orders below this are not emitted. SpawnPoint.Policy.MacroMinOrderCNY may
// override upward but never below this constant.
const MacroMinOrderCNYFloor = 100.0

// MacroDecisionInput is the bundle the macro engine consumes per tick.
type MacroDecisionInput struct {
	Closes       []float64
	CurrentPrice float64
	TotalEquity  float64
	SpendableCNY float64
	State        MarketState
	Runtime      RuntimeState
	Params       Chromosome
	Spawn        SpawnPoint
	Symbol       string
}

// MacroDecision is the macro engine's output for one tick.
type MacroDecision struct {
	Triggered      bool
	OrderCNY       float64
	Order          Order
	PeakPrice      float64
	Drawdown       float64
	DcaMultiplier  float64
	StateMultiplier float64
}

// ComputeMacroDecision implements the dynamic DCA logic from doc Ch.4.
//
// Iron rules:
//   - Macro emits BUY intents only; never SELL.
//   - OrderCNY is clamped to SpendableCNY.
//   - Orders smaller than max(Policy.MacroMinOrderCNY, MacroMinOrderCNYFloor)
//     are suppressed (Triggered = false, OrderCNY = 0).
//   - QUIET state is silent; cooldown gates ticks; minimum spendable cash is
//     required.
func ComputeMacroDecision(in MacroDecisionInput) MacroDecision {
	p := in.Params
	out := MacroDecision{}
	minTicket := math.Max(in.Spawn.Policy.MacroMinOrderCNY, MacroMinOrderCNYFloor)

	// (1) State gate.
	if in.State.State == MarketQuiet {
		return out
	}
	// (2) Spendable cash gate.
	if in.SpendableCNY <= minTicket {
		return out
	}
	// (3) Cooldown gate.
	if in.Runtime.TicksSinceLastMacro < p.MacroCooldownTicks {
		return out
	}

	// Peak / drawdown over the rolling window.
	window := p.NPeakWindow
	start := len(in.Closes) - window
	if start < 0 {
		start = 0
	}
	peak := in.CurrentPrice
	for _, c := range in.Closes[start:] {
		if c > peak {
			peak = c
		}
	}
	dd := 0.0
	if peak > 0 {
		dd = (peak - in.CurrentPrice) / peak
		if dd < 0 {
			dd = 0
		}
	}

	dcaMul := 1.0 + p.DcaBoostCoef*math.Pow(dd/p.DcaDdScale, p.DcaConvexity)
	if math.IsNaN(dcaMul) || math.IsInf(dcaMul, 0) {
		dcaMul = 1.0
	}

	stateMul := 1.0
	if in.State.State == MarketBear {
		stateMul = p.BearMacroBoost
	}

	raw := in.SpendableCNY * p.MacroBaseAllocPct * dcaMul * stateMul
	cap := in.SpendableCNY * p.MacroMaxSinglePct
	orderCNY := math.Min(raw, cap)
	orderCNY = math.Min(orderCNY, in.SpendableCNY) // hard clamp

	if orderCNY < minTicket {
		return out
	}

	out.Triggered = true
	out.OrderCNY = RoundToCNY(orderCNY)
	out.PeakPrice = peak
	out.Drawdown = dd
	out.DcaMultiplier = dcaMul
	out.StateMultiplier = stateMul
	out.Order = Order{
		Action:    OrderBuy,
		LotType:   LotDeadStack,
		AmountCNY: out.OrderCNY,
		Symbol:    in.Symbol,
	}
	return out
}
