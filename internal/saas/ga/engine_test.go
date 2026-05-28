package ga

import (
	"context"
	"encoding/json"
	"math/rand"
	"testing"

	"github.com/quantsaas/platform/internal/quant"
)

// fakeEvolvable is a deterministic stand-in for a real strategy so the GA loop
// dynamics can be tested without backtests or a database. Genes are plain ints;
// crossover/mutation behaviour is supplied per-test via the func fields.
type fakeEvolvable struct {
	crossover func(a, b Gene, rng *rand.Rand) Gene
	mutate    func(gene Gene, prob, scale float64, rng *rand.Rand) Gene
}

func (f fakeEvolvable) StrategyID() string                  { return "fake" }
func (f fakeEvolvable) Sample(rng *rand.Rand) Gene          { return 0 }
func (f fakeEvolvable) Fingerprint(gene Gene) uint64        { return uint64(gene.(int)) }
func (f fakeEvolvable) Evaluate(gene Gene, _ *EvaluablePlan) float64 {
	return float64(gene.(int))
}
func (f fakeEvolvable) DecodeElite(_ json.RawMessage) Gene { return 0 }
func (f fakeEvolvable) EncodeResult(_ Gene, _ quant.SpawnPoint) json.RawMessage {
	return nil
}
func (f fakeEvolvable) Crossover(a, b Gene, rng *rand.Rand) Gene {
	return f.crossover(a, b, rng)
}
func (f fakeEvolvable) Mutate(gene Gene, prob, scale float64, rng *rand.Rand) Gene {
	return f.mutate(gene, prob, scale, rng)
}

func geneValues(inds []individual) map[int]bool {
	set := make(map[int]bool, len(inds))
	for _, ind := range inds {
		set[ind.gene.(int)] = true
	}
	return set
}

// TestEliteSurvivesOneGeneration verifies elitism: after one generation the top
// EliteCount genes of the previous generation must still be present in the new
// population. To prove their presence is due to elite carry-over (and not a
// breeding coincidence), mutation shifts every bred child far out of the
// original gene range, so the originals can only reappear via the elite copy.
func TestEliteSurvivesOneGeneration(t *testing.T) {
	eng := &EvolutionEngine{
		evolvable: fakeEvolvable{
			crossover: func(a, b Gene, _ *rand.Rand) Gene { return a },
			mutate:    func(g Gene, _, _ float64, _ *rand.Rand) Gene { return g.(int) + 100_000 },
		},
		EliteCount:             3,
		TournamentSize:         3,
		MutationProbability:    0.2,
		MutationScale:          1.0,
		MutationProbabilityMax: 0.55,
		MutationScaleMax:       3.0,
		MutationRampFactor:     1.25,
		EarlyStopPatience:      5,
		EarlyStopMinDelta:      0.001,
	}

	initial := []Gene{10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	// Fitness == gene value (see fakeEvolvable.Evaluate is unused; evaluate
	// closure below mirrors it), so the gen-0 top 3 are {10, 9, 8}.
	topN := []int{10, 9, 8}
	evaluate := func(pop []Gene) []float64 {
		s := make([]float64, len(pop))
		for i, g := range pop {
			s[i] = float64(g.(int))
		}
		return s
	}

	rng := rand.New(rand.NewSource(7))
	final, _, err := eng.runEvolution(context.Background(), initial, 1, rng, evaluate, nil)
	if err != nil {
		t.Fatal(err)
	}

	present := geneValues(final)
	for _, g := range topN {
		if !present[g] {
			t.Fatalf("elite gene %d did not survive one generation; population=%v", g, present)
		}
	}
}

// TestNextGenerationCopiesElitesInOrder asserts the structural guarantee that
// the first EliteCount slots of the successor population are exactly the top
// genes, copied verbatim and in fitness order.
func TestNextGenerationCopiesElitesInOrder(t *testing.T) {
	eng := &EvolutionEngine{
		evolvable: fakeEvolvable{
			crossover: func(a, b Gene, _ *rand.Rand) Gene { return a },
			mutate:    func(g Gene, _, _ float64, _ *rand.Rand) Gene { return g.(int) + 100_000 },
		},
		EliteCount:     3,
		TournamentSize: 3,
	}
	// Must be supplied sorted by descending fitness.
	inds := []individual{
		{gene: 10, fitness: 10},
		{gene: 9, fitness: 9},
		{gene: 8, fitness: 8},
		{gene: 7, fitness: 7},
		{gene: 6, fitness: 6},
	}
	rng := rand.New(rand.NewSource(1))
	next := eng.nextGeneration(inds, 10, 0.2, 1.0, rng)

	if len(next) != 10 {
		t.Fatalf("expected population size 10, got %d", len(next))
	}
	for i := 0; i < eng.EliteCount; i++ {
		if next[i].(int) != inds[i].gene.(int) {
			t.Fatalf("elite slot %d = %v, want %v", i, next[i], inds[i].gene)
		}
	}
}

// TestMutationRampAfterStall verifies the mutation-rate ramp: with a fitness
// landscape that never improves, after exactly EarlyStopPatience generations of
// no improvement the mutation probability must step up to (initial × ramp
// factor) — no sooner, and to that exact value.
func TestMutationRampAfterStall(t *testing.T) {
	eng := &EvolutionEngine{
		evolvable: fakeEvolvable{
			crossover: func(a, b Gene, _ *rand.Rand) Gene { return a },
			mutate:    func(g Gene, _, _ float64, _ *rand.Rand) Gene { return g },
		},
		EliteCount:             2,
		TournamentSize:         3,
		MutationProbability:    0.15,
		MutationScale:          1.0,
		MutationProbabilityMax: 0.55,
		MutationScaleMax:       3.0,
		MutationRampFactor:     1.25,
		EarlyStopPatience:      5,
		EarlyStopMinDelta:      0.001,
	}

	initial := []Gene{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	// Constant (zero) fitness every generation → no improvement after gen 0.
	evaluate := func(pop []Gene) []float64 { return make([]float64, len(pop)) }

	var probs []float64
	onProgress := func(_ int, _, mutProb, _ float64) {
		probs = append(probs, mutProb)
	}

	rng := rand.New(rand.NewSource(3))
	if _, _, err := eng.runEvolution(context.Background(), initial, 12, rng, evaluate, onProgress); err != nil {
		t.Fatal(err)
	}

	rampIdx := -1
	for i, p := range probs {
		if p != eng.MutationProbability {
			rampIdx = i
			break
		}
	}
	if rampIdx == -1 {
		t.Fatalf("mutation probability never ramped: %v", probs)
	}
	if rampIdx != eng.EarlyStopPatience {
		t.Fatalf("ramp first occurred at generation %d, want EarlyStopPatience=%d (probs=%v)",
			rampIdx, eng.EarlyStopPatience, probs)
	}
	want := eng.MutationProbability * eng.MutationRampFactor
	if probs[rampIdx] != want {
		t.Fatalf("ramped mutProb = %v, want initial×factor = %v", probs[rampIdx], want)
	}
}

// TestTournamentRarelySelectsFatal verifies that catastrophic individuals
// (score == fatalFitnessScore) are almost never chosen by tournament selection:
// across 1000 selections a single fatal individual among 100 must be picked far
// less than 5% of the time.
func TestTournamentRarelySelectsFatal(t *testing.T) {
	eng := &EvolutionEngine{TournamentSize: 3}

	const n = 100
	inds := make([]individual, n)
	for i := range inds {
		inds[i] = individual{gene: i, fitness: float64(i)}
	}
	const fatalGene = 999
	inds[0] = individual{gene: fatalGene, fitness: fatalFitnessScore}

	rng := rand.New(rand.NewSource(42))
	const trials = 1000
	fatalPicks := 0
	for i := 0; i < trials; i++ {
		if eng.tournamentSelect(inds, rng).(int) == fatalGene {
			fatalPicks++
		}
	}

	if fatalPicks >= trials*5/100 {
		t.Fatalf("fatal individual selected %d/%d times, expected < 5%%", fatalPicks, trials)
	}
}
