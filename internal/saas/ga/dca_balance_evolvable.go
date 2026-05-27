package ga

import (
	"encoding/binary"
	"encoding/json"
	"hash/fnv"
	"math"
	"math/rand"

	"github.com/quantsaas/platform/internal/adapters/backtest"
	"github.com/quantsaas/platform/internal/quant"
)

const (
	dcaBalanceStrategyID    = "dca_balance"
	fatalFitnessScore       = -99999.0
	maxDDFatalThreshold     = 0.88
	ddPenaltyFactor         = 1.5
)

// DCABalanceEvolvable adapts the DCA+Sigmoid strategy to the EvolvableStrategy
// interface. It is the only code that knows the Chromosome layout.
type DCABalanceEvolvable struct{}

func (e *DCABalanceEvolvable) StrategyID() string { return dcaBalanceStrategyID }

// Sample returns a uniformly random Chromosome within HardBounds.
func (e *DCABalanceEvolvable) Sample(rng *rand.Rand) Gene {
	b := quant.HardBounds
	chr := quant.Chromosome{
		NEMAFast:          randI(rng, b.NEMAFast),
		NEMAMid:           randI(rng, b.NEMAMid),
		NEMASlow:          randI(rng, b.NEMASlow),
		NAtr:              randI(rng, b.NAtr),
		AtrQuietThreshold: randF(rng, b.AtrQuietThreshold),
		QuietBand:         randF(rng, b.QuietBand),

		AlphaTrend:     randF(rng, b.AlphaTrend),
		AlphaReversion: randF(rng, b.AlphaReversion),
		AlphaVol:       randF(rng, b.AlphaVol),
		RevertScale:    randF(rng, b.RevertScale),
		AtrBaseline:    randF(rng, b.AtrBaseline),

		Beta:             randF(rng, b.Beta),
		Gamma:            randF(rng, b.Gamma),
		SigmaFloor:       randF(rng, b.SigmaFloor),
		MicroDustCNY:     randF(rng, b.MicroDustCNY),
		DeltaWeightWedge: randF(rng, b.DeltaWeightWedge),
		VolRatioWedge:    randF(rng, b.VolRatioWedge),
		BearMicroScale:   randF(rng, b.BearMicroScale),

		MacroReservePct:    randF(rng, b.MacroReservePct),
		MacroBaseAllocPct:  randF(rng, b.MacroBaseAllocPct),
		MacroMaxSinglePct:  randF(rng, b.MacroMaxSinglePct),
		MacroCooldownTicks: randI(rng, b.MacroCooldownTicks),
		NPeakWindow:        randI(rng, b.NPeakWindow),
		DcaBoostCoef:       randF(rng, b.DcaBoostCoef),
		DcaDdScale:         randF(rng, b.DcaDdScale),
		DcaConvexity:       randF(rng, b.DcaConvexity),
		BearMacroBoost:     randF(rng, b.BearMacroBoost),

		ReleaseProfitThreshold: randF(rng, b.ReleaseProfitThreshold),
		ReleaseMinHoldDays:     randI(rng, b.ReleaseMinHoldDays),
		WMicroReleaseCap:       randF(rng, b.WMicroReleaseCap),
	}
	return quant.ClampChromosome(chr)
}

// Mutate applies independent Bernoulli+Gaussian perturbation to each field.
func (e *DCABalanceEvolvable) Mutate(gene Gene, prob, scale float64, rng *rand.Rand) Gene {
	chr := asChromosome(gene)
	b := quant.HardBounds

	mf := func(v float64, bounds quant.FloatBounds) float64 {
		if rng.Float64() >= prob {
			return v
		}
		step := (bounds.Max - bounds.Min) / 10.0
		return v + rng.NormFloat64()*step*scale
	}
	mi := func(v int, bounds quant.IntBounds) int {
		if rng.Float64() >= prob {
			return v
		}
		step := math.Max(1.0, float64(bounds.Max-bounds.Min)/10.0)
		return v + int(math.Round(rng.NormFloat64()*step*scale))
	}

	chr.NEMAFast = mi(chr.NEMAFast, b.NEMAFast)
	chr.NEMAMid = mi(chr.NEMAMid, b.NEMAMid)
	chr.NEMASlow = mi(chr.NEMASlow, b.NEMASlow)
	chr.NAtr = mi(chr.NAtr, b.NAtr)
	chr.AtrQuietThreshold = mf(chr.AtrQuietThreshold, b.AtrQuietThreshold)
	chr.QuietBand = mf(chr.QuietBand, b.QuietBand)

	chr.AlphaTrend = mf(chr.AlphaTrend, b.AlphaTrend)
	chr.AlphaReversion = mf(chr.AlphaReversion, b.AlphaReversion)
	chr.AlphaVol = mf(chr.AlphaVol, b.AlphaVol)
	chr.RevertScale = mf(chr.RevertScale, b.RevertScale)
	chr.AtrBaseline = mf(chr.AtrBaseline, b.AtrBaseline)

	chr.Beta = mf(chr.Beta, b.Beta)
	chr.Gamma = mf(chr.Gamma, b.Gamma)
	chr.SigmaFloor = mf(chr.SigmaFloor, b.SigmaFloor)
	chr.MicroDustCNY = mf(chr.MicroDustCNY, b.MicroDustCNY)
	chr.DeltaWeightWedge = mf(chr.DeltaWeightWedge, b.DeltaWeightWedge)
	chr.VolRatioWedge = mf(chr.VolRatioWedge, b.VolRatioWedge)
	chr.BearMicroScale = mf(chr.BearMicroScale, b.BearMicroScale)

	chr.MacroReservePct = mf(chr.MacroReservePct, b.MacroReservePct)
	chr.MacroBaseAllocPct = mf(chr.MacroBaseAllocPct, b.MacroBaseAllocPct)
	chr.MacroMaxSinglePct = mf(chr.MacroMaxSinglePct, b.MacroMaxSinglePct)
	chr.MacroCooldownTicks = mi(chr.MacroCooldownTicks, b.MacroCooldownTicks)
	chr.NPeakWindow = mi(chr.NPeakWindow, b.NPeakWindow)
	chr.DcaBoostCoef = mf(chr.DcaBoostCoef, b.DcaBoostCoef)
	chr.DcaDdScale = mf(chr.DcaDdScale, b.DcaDdScale)
	chr.DcaConvexity = mf(chr.DcaConvexity, b.DcaConvexity)
	chr.BearMacroBoost = mf(chr.BearMacroBoost, b.BearMacroBoost)

	chr.ReleaseProfitThreshold = mf(chr.ReleaseProfitThreshold, b.ReleaseProfitThreshold)
	chr.ReleaseMinHoldDays = mi(chr.ReleaseMinHoldDays, b.ReleaseMinHoldDays)
	chr.WMicroReleaseCap = mf(chr.WMicroReleaseCap, b.WMicroReleaseCap)

	return quant.ClampChromosome(chr)
}

// Crossover performs uniform crossover: each field chosen from parent a or b with P=0.5.
func (e *DCABalanceEvolvable) Crossover(a, b Gene, rng *rand.Rand) Gene {
	ca := asChromosome(a)
	cb := asChromosome(b)

	pf := func(x, y float64) float64 {
		if rng.Float64() < 0.5 {
			return x
		}
		return y
	}
	pi := func(x, y int) int {
		if rng.Float64() < 0.5 {
			return x
		}
		return y
	}

	chr := quant.Chromosome{
		NEMAFast:          pi(ca.NEMAFast, cb.NEMAFast),
		NEMAMid:           pi(ca.NEMAMid, cb.NEMAMid),
		NEMASlow:          pi(ca.NEMASlow, cb.NEMASlow),
		NAtr:              pi(ca.NAtr, cb.NAtr),
		AtrQuietThreshold: pf(ca.AtrQuietThreshold, cb.AtrQuietThreshold),
		QuietBand:         pf(ca.QuietBand, cb.QuietBand),

		AlphaTrend:     pf(ca.AlphaTrend, cb.AlphaTrend),
		AlphaReversion: pf(ca.AlphaReversion, cb.AlphaReversion),
		AlphaVol:       pf(ca.AlphaVol, cb.AlphaVol),
		RevertScale:    pf(ca.RevertScale, cb.RevertScale),
		AtrBaseline:    pf(ca.AtrBaseline, cb.AtrBaseline),

		Beta:             pf(ca.Beta, cb.Beta),
		Gamma:            pf(ca.Gamma, cb.Gamma),
		SigmaFloor:       pf(ca.SigmaFloor, cb.SigmaFloor),
		MicroDustCNY:     pf(ca.MicroDustCNY, cb.MicroDustCNY),
		DeltaWeightWedge: pf(ca.DeltaWeightWedge, cb.DeltaWeightWedge),
		VolRatioWedge:    pf(ca.VolRatioWedge, cb.VolRatioWedge),
		BearMicroScale:   pf(ca.BearMicroScale, cb.BearMicroScale),

		MacroReservePct:    pf(ca.MacroReservePct, cb.MacroReservePct),
		MacroBaseAllocPct:  pf(ca.MacroBaseAllocPct, cb.MacroBaseAllocPct),
		MacroMaxSinglePct:  pf(ca.MacroMaxSinglePct, cb.MacroMaxSinglePct),
		MacroCooldownTicks: pi(ca.MacroCooldownTicks, cb.MacroCooldownTicks),
		NPeakWindow:        pi(ca.NPeakWindow, cb.NPeakWindow),
		DcaBoostCoef:       pf(ca.DcaBoostCoef, cb.DcaBoostCoef),
		DcaDdScale:         pf(ca.DcaDdScale, cb.DcaDdScale),
		DcaConvexity:       pf(ca.DcaConvexity, cb.DcaConvexity),
		BearMacroBoost:     pf(ca.BearMacroBoost, cb.BearMacroBoost),

		ReleaseProfitThreshold: pf(ca.ReleaseProfitThreshold, cb.ReleaseProfitThreshold),
		ReleaseMinHoldDays:     pi(ca.ReleaseMinHoldDays, cb.ReleaseMinHoldDays),
		WMicroReleaseCap:       pf(ca.WMicroReleaseCap, cb.WMicroReleaseCap),
	}
	return quant.ClampChromosome(chr)
}

// Fingerprint returns FNV-1a-64 hash of all chromosome fields quantised to 1e-6.
func (e *DCABalanceEvolvable) Fingerprint(gene Gene) uint64 {
	chr := asChromosome(gene)
	h := fnv.New64a()
	buf := make([]byte, 8)

	wf := func(v float64) {
		binary.LittleEndian.PutUint64(buf, math.Float64bits(math.Round(v*1e6)/1e6))
		h.Write(buf)
	}
	wi := func(v int) {
		binary.LittleEndian.PutUint64(buf, uint64(v))
		h.Write(buf)
	}

	wi(chr.NEMAFast); wi(chr.NEMAMid); wi(chr.NEMASlow); wi(chr.NAtr)
	wf(chr.AtrQuietThreshold); wf(chr.QuietBand)
	wf(chr.AlphaTrend); wf(chr.AlphaReversion); wf(chr.AlphaVol)
	wf(chr.RevertScale); wf(chr.AtrBaseline)
	wf(chr.Beta); wf(chr.Gamma); wf(chr.SigmaFloor)
	wf(chr.MicroDustCNY); wf(chr.DeltaWeightWedge); wf(chr.VolRatioWedge)
	wf(chr.BearMicroScale)
	wf(chr.MacroReservePct); wf(chr.MacroBaseAllocPct); wf(chr.MacroMaxSinglePct)
	wi(chr.MacroCooldownTicks); wi(chr.NPeakWindow)
	wf(chr.DcaBoostCoef); wf(chr.DcaDdScale); wf(chr.DcaConvexity); wf(chr.BearMacroBoost)
	wf(chr.ReleaseProfitThreshold); wi(chr.ReleaseMinHoldDays); wf(chr.WMicroReleaseCap)

	return h.Sum64()
}

// Evaluate runs the multi-window crucible (6m → 2y → 5y → full) with cascade
// short-circuit: MaxDD ≥ 88% on any window returns fatal immediately.
// Stores EvalDetail in plan.AggregateCache for post-run champion inspection.
func (e *DCABalanceEvolvable) Evaluate(gene Gene, plan *EvaluablePlan) float64 {
	chr := asChromosome(gene)
	fp := e.Fingerprint(gene)

	scoreTotal := 0.0
	worstMaxDD := 0.0

	for i, window := range plan.Windows {
		result := backtest.RunBacktest(window, chr, plan.Spawn)
		baseline := plan.DCABaselines[i]

		if result.MaxDD >= maxDDFatalThreshold {
			plan.AggregateCache.Store(fp, EvalDetail{Score: fatalFitnessScore, MaxDD: result.MaxDD})
			return fatalFitnessScore
		}
		if result.MaxDD > worstMaxDD {
			worstMaxDD = result.MaxDD
		}

		alpha := result.ROI - baseline.ROI
		ddExcess := math.Max(0, result.MaxDD-baseline.MaxDrawdown)
		scoreTotal += window.Weight * (alpha - ddPenaltyFactor*ddExcess)
	}

	plan.AggregateCache.Store(fp, EvalDetail{Score: scoreTotal, MaxDD: worstMaxDD})
	return scoreTotal
}

// DecodeElite decodes a GeneRecord.ParamPack into a Chromosome gene.
// Returns DefaultSeedChromosome when raw is empty or malformed.
func (e *DCABalanceEvolvable) DecodeElite(raw json.RawMessage) Gene {
	if len(raw) == 0 {
		return quant.DefaultSeedChromosome
	}
	var pack paramPack
	if err := json.Unmarshal(raw, &pack); err != nil {
		return quant.DefaultSeedChromosome
	}
	return pack.Chromosome
}

// EncodeResult serialises champion chromosome and spawn point into a ParamPack blob.
func (e *DCABalanceEvolvable) EncodeResult(gene Gene, spawn quant.SpawnPoint) json.RawMessage {
	raw, _ := json.Marshal(paramPack{
		Chromosome: asChromosome(gene),
		SpawnPoint: spawn,
	})
	return raw
}

// paramPack is the canonical JSON format stored in GeneRecord.ParamPack.
type paramPack struct {
	Chromosome quant.Chromosome `json:"chromosome"`
	SpawnPoint quant.SpawnPoint `json:"spawn_point"`
}

func asChromosome(gene Gene) quant.Chromosome {
	if chr, ok := gene.(quant.Chromosome); ok {
		return chr
	}
	return quant.DefaultSeedChromosome
}

func randF(rng *rand.Rand, b quant.FloatBounds) float64 {
	return b.Min + rng.Float64()*(b.Max-b.Min)
}

func randI(rng *rand.Rand, b quant.IntBounds) int {
	return b.Min + rng.Intn(b.Max-b.Min+1)
}
