package quant

import "testing"

func bullState() MarketState {
	return MarketState{State: MarketBull, TimeDilationMultiplier: 1, BetaMultiplier: 1}
}

func bearState(boost float64) MarketState {
	return MarketState{State: MarketBear, TimeDilationMultiplier: boost, BetaMultiplier: 1}
}

func TestComputeMacroDecisionFiresInBull(t *testing.T) {
	closes := make([]float64, 200)
	for i := range closes {
		closes[i] = 100.0
	}
	in := MacroDecisionInput{
		Closes:       closes,
		CurrentPrice: 100,
		TotalEquity:  100_000,
		SpendableCNY: 50_000,
		State:        bullState(),
		Runtime:      RuntimeState{TicksSinceLastMacro: 100},
		Params:       ClampChromosome(DefaultSeedChromosome),
		Spawn:        SpawnPoint{Policy: Policy{MacroMinOrderCNY: 100}},
		Symbol:       "510300.SH",
	}
	d := ComputeMacroDecision(in)
	if !d.Triggered {
		t.Fatal("expected macro to fire in BULL with healthy cash")
	}
	if d.Order.Action != OrderBuy || d.Order.LotType != LotDeadStack {
		t.Fatalf("macro must emit BUY/DEAD_STACK, got %+v", d.Order)
	}
	if d.OrderCNY > in.SpendableCNY {
		t.Fatalf("OrderCNY exceeds SpendableCNY: %f > %f", d.OrderCNY, in.SpendableCNY)
	}
}

func TestComputeMacroDecisionSilentInQuiet(t *testing.T) {
	closes := make([]float64, 200)
	for i := range closes {
		closes[i] = 100.0
	}
	in := MacroDecisionInput{
		Closes:       closes,
		CurrentPrice: 100,
		SpendableCNY: 50_000,
		State:        MarketState{State: MarketQuiet, IsQuiet: true},
		Runtime:      RuntimeState{TicksSinceLastMacro: 100},
		Params:       ClampChromosome(DefaultSeedChromosome),
		Spawn:        SpawnPoint{Policy: Policy{MacroMinOrderCNY: 100}},
	}
	d := ComputeMacroDecision(in)
	if d.Triggered {
		t.Fatal("macro must be silent in QUIET")
	}
}

func TestComputeMacroDecisionCooldownGate(t *testing.T) {
	closes := make([]float64, 200)
	for i := range closes {
		closes[i] = 100.0
	}
	params := ClampChromosome(DefaultSeedChromosome)
	in := MacroDecisionInput{
		Closes:       closes,
		CurrentPrice: 100,
		SpendableCNY: 50_000,
		State:        bullState(),
		Runtime:      RuntimeState{TicksSinceLastMacro: params.MacroCooldownTicks - 1},
		Params:       params,
		Spawn:        SpawnPoint{Policy: Policy{MacroMinOrderCNY: 100}},
	}
	d := ComputeMacroDecision(in)
	if d.Triggered {
		t.Fatal("cooldown gate failed")
	}
}

func TestComputeMacroDecisionBearBoost(t *testing.T) {
	closes := make([]float64, 200)
	for i := range closes {
		closes[i] = 100.0
	}
	params := ClampChromosome(DefaultSeedChromosome)
	base := MacroDecisionInput{
		Closes:       closes,
		CurrentPrice: 100,
		SpendableCNY: 50_000,
		State:        bullState(),
		Runtime:      RuntimeState{TicksSinceLastMacro: 100},
		Params:       params,
		Spawn:        SpawnPoint{Policy: Policy{MacroMinOrderCNY: 100}},
	}
	bull := ComputeMacroDecision(base)
	base.State = bearState(params.BearMacroBoost)
	bear := ComputeMacroDecision(base)
	if bear.OrderCNY <= bull.OrderCNY {
		t.Fatalf("BEAR should outsize BULL: bear=%f, bull=%f", bear.OrderCNY, bull.OrderCNY)
	}
}
