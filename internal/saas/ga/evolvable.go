// Package ga implements the genetic algorithm evolution engine and the
// EvolvableStrategy interface contract that concrete strategies must satisfy.
package ga

import (
	"encoding/json"
	"math/rand"
	"sync"

	"github.com/quantsaas/platform/internal/quant"
)

// Gene is the opaque carrier type for any chromosome representation.
// The engine operates on Gene values exclusively through EvolvableStrategy;
// it never reads chromosome field names directly.
type Gene = any

// EvalDetail is stored in EvaluablePlan.AggregateCache (keyed by fingerprint)
// so the engine can retrieve MaxDD for the champion after evaluation.
type EvalDetail struct {
	Score float64
	MaxDD float64
}

// DCABaseline holds the pre-computed Ghost DCA control-arm result for one window.
type DCABaseline struct {
	FinalEquity   float64
	TotalInjected float64
	MaxDrawdown   float64
	ROI           float64 // Modified Dietz ROI, used for Alpha = ROI_strategy - ROI_dca
}

// EvaluablePlan is built once at Epoch start and shared read-only across all
// concurrent workers. It bundles everything an Evaluate call needs except the gene.
type EvaluablePlan struct {
	Symbol         string
	TemplateName   string
	Spawn          quant.SpawnPoint
	LotStep        float64
	LotMin         float64
	Windows        []quant.CrucibleWindow // ordered short → long for cascade short-circuit
	DCABaselines   []DCABaseline          // DCABaselines[i] corresponds to Windows[i]
	AggregateCache sync.Map               // fingerprint (uint64) → EvalDetail
}

// EvolvableStrategy is the 8-verb contract between the evolution engine and any
// concrete strategy. The engine never reads chromosome field names directly.
type EvolvableStrategy interface {
	// StrategyID returns the immutable strategy template name.
	StrategyID() string

	// Sample draws a uniformly random gene from the legal search space.
	Sample(rng *rand.Rand) Gene

	// Mutate applies additive Gaussian perturbation to each dimension with
	// independent Bernoulli probability prob, scaled by scale, then clamps.
	Mutate(gene Gene, prob, scale float64, rng *rand.Rand) Gene

	// Crossover performs uniform crossover: each dimension drawn from a or b
	// with equal probability, then clamped for structural constraints.
	Crossover(a, b Gene, rng *rand.Rand) Gene

	// Fingerprint returns a stable FNV-1a-64 hash of the gene quantised to 1e-6
	// for de-duplication within one Epoch.
	Fingerprint(gene Gene) uint64

	// Evaluate runs the multi-window crucible fitness function and returns
	// ScoreTotal. Returns fatalFitnessScore (-99999) on catastrophic drawdown.
	// Also stores EvalDetail in plan.AggregateCache for post-run inspection.
	Evaluate(gene Gene, plan *EvaluablePlan) float64

	// DecodeElite decodes a GeneRecord.ParamPack JSON blob into a gene.
	// Falls back to DefaultSeedChromosome when raw is empty or malformed.
	DecodeElite(raw json.RawMessage) Gene

	// EncodeResult serialises a champion gene and its spawn point into a
	// ParamPack JSON blob ready to be stored in GeneRecord.ParamPack.
	EncodeResult(gene Gene, spawn quant.SpawnPoint) json.RawMessage
}
