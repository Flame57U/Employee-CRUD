package ga

import (
	"context"
	"encoding/json"
	"math"
	"math/rand"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/quantsaas/platform/internal/quant"
	"github.com/quantsaas/platform/internal/saas/store"
)

// EvolutionEngine runs a GA population over multiple generations to find the
// best chromosome for a given strategy and symbol.
type EvolutionEngine struct {
	evolvable EvolvableStrategy
	db        *store.DB

	PopSize                int
	MaxGenerations         int
	EliteCount             int
	MutationProbability    float64
	MutationScale          float64
	MutationProbabilityMax float64
	MutationScaleMax       float64
	MutationRampFactor     float64
	EarlyStopPatience      int
	EarlyStopMinDelta      float64
	TournamentSize         int
}

// NewEvolutionEngine returns an engine with default hyperparameters.
func NewEvolutionEngine(evolvable EvolvableStrategy, db *store.DB) *EvolutionEngine {
	return &EvolutionEngine{
		evolvable:              evolvable,
		db:                     db,
		PopSize:                300,
		MaxGenerations:         25,
		EliteCount:             8,
		MutationProbability:    0.15,
		MutationScale:          1.0,
		MutationProbabilityMax: 0.55,
		MutationScaleMax:       3.0,
		MutationRampFactor:     1.25,
		EarlyStopPatience:      5,
		EarlyStopMinDelta:      0.001,
		TournamentSize:         3,
	}
}

// EpochConfig overrides engine hyperparameters for one run.
type EpochConfig struct {
	PopSize            int
	MaxGenerations     int
	LotStepSize        float64
	LotMinQty          float64
	OnProgress         func(generation int, bestFitness, mutProb, mutScale float64)
	SpawnPointOverride *quant.SpawnPoint // non-nil overrides champion/default spawn
}

// EpochResult carries the output of one completed evolution run.
type EpochResult struct {
	BestGene     Gene
	BestFitness  float64
	BestMaxDD    float64
	GeneRecordID uint
	Generations  int
}

// individual pairs a gene with its evaluated fitness score.
type individual struct {
	gene    Gene
	fitness float64
}

// RunEpoch executes one full evolution cycle and persists the best challenger.
func (eng *EvolutionEngine) RunEpoch(ctx context.Context, strategyID uint, symbol string, cfg EpochConfig) (EpochResult, error) {
	popSize := eng.PopSize
	if cfg.PopSize > 0 {
		popSize = cfg.PopSize
	}
	maxGen := eng.MaxGenerations
	if cfg.MaxGenerations > 0 {
		maxGen = cfg.MaxGenerations
	}

	// Step 1: Build EvaluablePlan
	plan, err := eng.buildPlan(strategyID, symbol, cfg)
	if err != nil {
		return EpochResult{}, err
	}

	// Step 2: Initialize population
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	population := eng.initPopulation(strategyID, popSize, rng)

	// Step 3: Evaluate initial population
	scores := eng.evaluatePopulation(population, plan)
	inds := makeIndividuals(population, scores)

	mutProb := eng.MutationProbability
	mutScale := eng.MutationScale
	bestFitness := -math.MaxFloat64
	patienceCount := 0
	generation := 0

	// Step 4: Main evolution loop
	for gen := 0; gen < maxGen; gen++ {
		generation = gen
		select {
		case <-ctx.Done():
			return EpochResult{}, ctx.Err()
		default:
		}

		sort.Slice(inds, func(i, j int) bool {
			return inds[i].fitness > inds[j].fitness
		})

		currentBest := inds[0].fitness
		if currentBest-bestFitness >= eng.EarlyStopMinDelta {
			bestFitness = currentBest
			patienceCount = 0
		} else {
			patienceCount++
		}

		if patienceCount >= eng.EarlyStopPatience {
			mutProb = math.Min(mutProb*eng.MutationRampFactor, eng.MutationProbabilityMax)
			mutScale = math.Min(mutScale*eng.MutationRampFactor, eng.MutationScaleMax)
		}

		if cfg.OnProgress != nil {
			cfg.OnProgress(gen, currentBest, mutProb, mutScale)
		}

		// Early stop: ramp params both at ceiling and still no improvement
		if mutProb >= eng.MutationProbabilityMax &&
			mutScale >= eng.MutationScaleMax &&
			patienceCount >= eng.EarlyStopPatience {
			break
		}

		// Produce next generation
		next := make([]Gene, 0, popSize)
		for i := 0; i < eng.EliteCount && i < len(inds); i++ {
			next = append(next, inds[i].gene)
		}
		for len(next) < popSize {
			pa := eng.tournamentSelect(inds, rng)
			pb := eng.tournamentSelect(inds, rng)
			child := eng.evolvable.Crossover(pa, pb, rng)
			child = eng.evolvable.Mutate(child, mutProb, mutScale, rng)
			next = append(next, child)
		}

		newScores := eng.evaluatePopulation(next, plan)
		inds = makeIndividuals(next, newScores)
	}

	sort.Slice(inds, func(i, j int) bool {
		return inds[i].fitness > inds[j].fitness
	})

	// Step 5: Persist best individual as challenger
	champion := inds[0]
	paramPackBlob := eng.evolvable.EncodeResult(champion.gene, plan.Spawn)

	// Retrieve MaxDD for champion from the aggregate cache
	bestMaxDD := 0.0
	fp := eng.evolvable.Fingerprint(champion.gene)
	if v, ok := plan.AggregateCache.Load(fp); ok {
		if detail, ok := v.(EvalDetail); ok {
			bestMaxDD = detail.MaxDD
		}
	}

	rec := store.GeneRecord{
		StrategyID:  strategyID,
		Role:        store.GeneRoleChallenger,
		ParamPack:   paramPackBlob,
		ScoreTotal:  champion.fitness,
		MaxDrawdown: bestMaxDD,
	}
	if err := eng.db.Create(&rec).Error; err != nil {
		return EpochResult{}, err
	}

	return EpochResult{
		BestGene:     champion.gene,
		BestFitness:  champion.fitness,
		BestMaxDD:    bestMaxDD,
		GeneRecordID: rec.ID,
		Generations:  generation + 1,
	}, nil
}

// buildPlan constructs the immutable EvaluablePlan for the Epoch.
func (eng *EvolutionEngine) buildPlan(strategyID uint, symbol string, cfg EpochConfig) (*EvaluablePlan, error) {
	var klines []store.KLine
	if err := eng.db.Where("symbol = ? AND interval = ?", symbol, "1d").
		Order("open_time ASC").
		Find(&klines).Error; err != nil {
		return nil, err
	}

	bars := make([]quant.Bar, len(klines))
	for i, k := range klines {
		bars[i] = quant.Bar{
			OpenTime: k.OpenTime.Unix(),
			Open:     k.Open,
			High:     k.High,
			Low:      k.Low,
			Close:    k.Close,
			Volume:   k.Volume,
		}
	}

	windows := quant.BuildCrucibleWindows(bars, quant.CrucibleWarmupDays)

	// Resolve spawn point: override → champion DB → default
	spawn := defaultSpawnPoint(symbol)
	if cfg.SpawnPointOverride != nil {
		spawn = *cfg.SpawnPointOverride
	} else {
		var champ store.GeneRecord
		if err := eng.db.Where("strategy_id = ? AND role = ?", strategyID, store.GeneRoleChampion).
			First(&champ).Error; err == nil {
			if sp, ok := decodeSpawnFromPack(champ.ParamPack); ok {
				spawn = sp
			}
		}
	}
	spawn.Policy.Symbol = symbol

	dcaCfg := quant.GhostDCAConfig{
		InitialCapital: spawn.Policy.TotalCapitalCNY,
		MonthlyInject:  spawn.Policy.MonthlyInjectCNY,
	}
	dcaBases := make([]DCABaseline, len(windows))
	for i, w := range windows {
		evalBars := evalOnlyBars(w)
		r := quant.SimulateGhostDCA(evalBars, dcaCfg)
		dcaBases[i] = DCABaseline{
			FinalEquity:   r.FinalEquity,
			TotalInjected: r.TotalInjected,
			MaxDrawdown:   r.MaxDrawdown,
			ROI:           r.ROI,
		}
	}

	return &EvaluablePlan{
		Symbol:       symbol,
		TemplateName: eng.evolvable.StrategyID(),
		Spawn:        spawn,
		LotStep:      cfg.LotStepSize,
		LotMin:       cfg.LotMinQty,
		Windows:      windows,
		DCABaselines: dcaBases,
	}, nil
}

// initPopulation seeds the population following doc §2.1 elite-seeding rules.
func (eng *EvolutionEngine) initPopulation(strategyID uint, popSize int, rng *rand.Rand) []Gene {
	pop := make([]Gene, 0, popSize)

	var eliteRecords []store.GeneRecord
	eng.db.Where("strategy_id = ? AND role IN ?", strategyID,
		[]string{store.GeneRoleChampion, store.GeneRoleRetired}).
		Order("score_total DESC").
		Limit(20).
		Find(&eliteRecords)

	elites := make([]Gene, 0, len(eliteRecords))
	for _, rec := range eliteRecords {
		elites = append(elites, eng.evolvable.DecodeElite(rec.ParamPack))
	}

	mutRng := rand.New(rand.NewSource(rng.Int63()))

	if len(elites) == 0 {
		// No DB elites: index 0 = default seed, rest = random
		pop = append(pop, quant.DefaultSeedChromosome)
		for len(pop) < popSize {
			pop = append(pop, eng.evolvable.Sample(rng))
		}
		return pop
	}

	// Index 0: current champion (first in sorted elites)
	pop = append(pop, elites[0])

	remaining := popSize - 1
	nExact := max(1, remaining/10)
	nMutated := max(1, (remaining*40)/100)
	// remainder = random

	for i := 0; i < nExact && len(pop) < popSize; i++ {
		pop = append(pop, elites[i%len(elites)])
	}
	for i := 0; i < nMutated && len(pop) < popSize; i++ {
		parent := elites[i%len(elites)]
		pop = append(pop, eng.evolvable.Mutate(parent, 0.15, 1.5, mutRng))
	}
	for len(pop) < popSize {
		pop = append(pop, eng.evolvable.Sample(rng))
	}

	return pop
}

// evaluatePopulation concurrently scores every gene in pop using a worker pool
// with a fingerprint cache to skip duplicate evaluations.
func (eng *EvolutionEngine) evaluatePopulation(pop []Gene, plan *EvaluablePlan) []float64 {
	n := len(pop)
	workers := runtime.NumCPU()
	if workers > n {
		workers = n
	}

	scores := make([]float64, n)
	var cache sync.Map // fingerprint (uint64) → float64 score

	type task struct {
		idx  int
		gene Gene
	}
	tasks := make(chan task, n)
	for i, g := range pop {
		tasks <- task{i, g}
	}
	close(tasks)

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range tasks {
				fp := eng.evolvable.Fingerprint(t.gene)
				if cached, ok := cache.Load(fp); ok {
					scores[t.idx] = cached.(float64)
					continue
				}
				s := eng.evolvable.Evaluate(t.gene, plan)
				cache.Store(fp, s)
				scores[t.idx] = s
			}
		}()
	}
	wg.Wait()
	return scores
}

// tournamentSelect draws TournamentSize candidates at random and returns the
// gene with the highest fitness.
func (eng *EvolutionEngine) tournamentSelect(inds []individual, rng *rand.Rand) Gene {
	n := len(inds)
	best := inds[rng.Intn(n)]
	for i := 1; i < eng.TournamentSize; i++ {
		c := inds[rng.Intn(n)]
		if c.fitness > best.fitness {
			best = c
		}
	}
	return best.gene
}

// -- helpers --

func makeIndividuals(pop []Gene, scores []float64) []individual {
	inds := make([]individual, len(pop))
	for i := range pop {
		inds[i] = individual{gene: pop[i], fitness: scores[i]}
	}
	return inds
}

// evalOnlyBars extracts the subset of window bars from EvalStartMs onward.
func evalOnlyBars(w quant.CrucibleWindow) []quant.Bar {
	for i, b := range w.Bars {
		if b.OpenTime >= w.EvalStartMs {
			return w.Bars[i:]
		}
	}
	return nil
}

// spawnPackWrapper is used only to extract SpawnPoint from a ParamPack blob
// without knowing the full Chromosome layout.
type spawnPackWrapper struct {
	SpawnPoint quant.SpawnPoint `json:"spawn_point"`
}

func decodeSpawnFromPack(raw json.RawMessage) (quant.SpawnPoint, bool) {
	var w spawnPackWrapper
	if err := json.Unmarshal(raw, &w); err != nil {
		return quant.SpawnPoint{}, false
	}
	return w.SpawnPoint, true
}

// defaultSpawnPoint returns sensible Epoch defaults when no champion exists.
func defaultSpawnPoint(symbol string) quant.SpawnPoint {
	return quant.SpawnPoint{
		Policy: quant.Policy{
			Symbol:             symbol,
			AssetClass:         "A_STOCK_ETF",
			TotalCapitalCNY:    100000,
			MonthlyInjectCNY:   2000,
			DeadlineRatio:      0.8,
			MacroMinOrderCNY:   200,
			QuietDustThreshold: 0.005,
			MaxLotsPerTick:     3,
			ReleaseTrigger:     0.15,
		},
		Risk: quant.Risk{
			FeeRate:           0.0001,
			GlobalStopLoss:    0.30,
			MaxDailyDrawdown:  0.05,
			MaxConsecLoseDays: 10,
		},
	}
}
