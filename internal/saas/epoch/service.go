// Package epoch implements the evolution task lifecycle: creation, async
// execution, progress tracking, and spawn-mode resolution.
package epoch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/quantsaas/platform/internal/quant"
	"github.com/quantsaas/platform/internal/saas/ga"
	"github.com/quantsaas/platform/internal/saas/store"
	"gorm.io/gorm"
)

// EpochService manages evolution task lifecycle. Only one task may run at a time.
type EpochService struct {
	db      *store.DB
	engine  *ga.EvolutionEngine
	mu      sync.Mutex
	current *runningTask
}

type runningTask struct {
	taskID uint
	cancel context.CancelFunc
}

// NewEpochService constructs an EpochService.
func NewEpochService(db *store.DB, engine *ga.EvolutionEngine) *EpochService {
	return &EpochService{db: db, engine: engine}
}

// CreateTaskRequest is the request body for POST /api/v1/evolution/tasks.
type CreateTaskRequest struct {
	StrategyID     uint              `json:"strategy_id"`
	Symbol         string            `json:"symbol"`
	PopSize        int               `json:"pop_size"`
	MaxGenerations int               `json:"max_generations"`
	SpawnMode      string            `json:"spawn_mode"` // "inherit" | "random_once" | "manual"
	SpawnPoint     *quant.SpawnPoint `json:"spawn_point,omitempty"`
}

// CreateAndRunTask validates the request, creates an EvolutionTask in the DB,
// resolves the SpawnPoint, and launches the evolution goroutine asynchronously.
func (s *EpochService) CreateAndRunTask(req CreateTaskRequest) (*store.EvolutionTask, error) {
	if req.Symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.current != nil {
		var existing store.EvolutionTask
		if err := s.db.First(&existing, s.current.taskID).Error; err == nil &&
			existing.Status == "running" {
			return nil, errors.New("another evolution task is already running")
		}
	}

	spawnOverride, err := s.resolveSpawn(req)
	if err != nil {
		return nil, fmt.Errorf("resolve spawn: %w", err)
	}

	cfgJSON, _ := json.Marshal(req)
	task := store.EvolutionTask{
		StrategyID: req.StrategyID,
		Status:     "running",
		Config:     cfgJSON,
	}
	if err := s.db.Create(&task).Error; err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.current = &runningTask{taskID: task.ID, cancel: cancel}

	go s.runEpoch(ctx, task.ID, req, spawnOverride)
	return &task, nil
}

// CancelCurrentTask cancels the running evolution task, if any.
func (s *EpochService) CancelCurrentTask() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current != nil {
		s.current.cancel()
	}
}

// resolveSpawn resolves the SpawnPoint based on spawn_mode.
func (s *EpochService) resolveSpawn(req CreateTaskRequest) (*quant.SpawnPoint, error) {
	switch req.SpawnMode {
	case "inherit", "":
		var champ store.GeneRecord
		err := s.db.Where("strategy_id = ? AND role = ?", req.StrategyID, store.GeneRoleChampion).
			First(&champ).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			sp := defaultSpawnPoint(req.Symbol)
			return &sp, nil
		}
		if err != nil {
			return nil, err
		}
		sp, ok := decodeSpawn(champ.ParamPack)
		if !ok {
			def := defaultSpawnPoint(req.Symbol)
			return &def, nil
		}
		return &sp, nil

	case "random_once":
		sp := randomSpawnPoint(req.Symbol)
		return &sp, nil

	case "manual":
		if req.SpawnPoint == nil {
			return nil, errors.New("spawn_point required for spawn_mode=manual")
		}
		return req.SpawnPoint, nil

	default:
		return nil, fmt.Errorf("unknown spawn_mode: %q", req.SpawnMode)
	}
}

// runEpoch is the async goroutine body.
func (s *EpochService) runEpoch(ctx context.Context, taskID uint, req CreateTaskRequest, spawnOverride *quant.SpawnPoint) {
	cfg := ga.EpochConfig{
		PopSize:            req.PopSize,
		MaxGenerations:     req.MaxGenerations,
		SpawnPointOverride: spawnOverride,
		OnProgress: func(gen int, _, _, _ float64) {
			prog := 0.0
			total := req.MaxGenerations
			if total > 0 {
				prog = float64(gen) / float64(total)
			}
			s.db.Model(&store.EvolutionTask{}).Where("id = ?", taskID).
				Update("progress", prog)
		},
	}

	_, epochErr := s.engine.RunEpoch(ctx, req.StrategyID, req.Symbol, cfg)

	status := "done"
	if epochErr != nil {
		status = "failed"
	}
	s.db.Model(&store.EvolutionTask{}).Where("id = ?", taskID).
		Updates(map[string]any{"status": status, "progress": 1.0})

	s.mu.Lock()
	if s.current != nil && s.current.taskID == taskID {
		s.current = nil
	}
	s.mu.Unlock()
}

// -- helpers --

type spawnWrapper struct {
	SpawnPoint quant.SpawnPoint `json:"spawn_point"`
}

func decodeSpawn(raw json.RawMessage) (quant.SpawnPoint, bool) {
	var w spawnWrapper
	if err := json.Unmarshal(raw, &w); err != nil {
		return quant.SpawnPoint{}, false
	}
	return w.SpawnPoint, true
}

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

// randomSpawnPoint draws a random SpawnPoint within conservative bounds.
func randomSpawnPoint(symbol string) quant.SpawnPoint {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	sp := defaultSpawnPoint(symbol)
	// Vary capital allocation slightly for exploration
	sp.Policy.TotalCapitalCNY = 50000 + rng.Float64()*150000  // 50k–200k
	sp.Policy.MonthlyInjectCNY = 500 + rng.Float64()*4500     // 500–5000
	sp.Risk.FeeRate = 0.00005 + rng.Float64()*0.00015         // 0.5–2 bps
	return sp
}
