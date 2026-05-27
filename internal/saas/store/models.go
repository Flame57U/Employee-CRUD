package store

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// Instance status values.
const (
	InstanceStatusRunning = "RUNNING"
	InstanceStatusStopped = "STOPPED"
	InstanceStatusError   = "ERROR"
)

// SpotLot.LotType values — must match TradeCommand.lot_type semantics.
const (
	LotTypeDeadStack  = "DEAD_STACK"
	LotTypeFloating   = "FLOATING"
	LotTypeColdSealed = "COLD_SEALED"
)

// GeneRecord.Role values.
const (
	GeneRoleChallenger = "challenger"
	GeneRoleChampion   = "champion"
	GeneRoleRetired    = "retired"
)

// User holds credentials and subscription plan.
type User struct {
	gorm.Model
	Email        string `gorm:"uniqueIndex;not null"`
	PasswordHash string `gorm:"not null"`
	Plan         string `gorm:"not null;default:'free'"` // free / pro / elite
}

// StrategyTemplate is the immutable blueprint for a strategy.
// Manifest is a JSON blob containing parameter schema and strategy-specific metadata.
type StrategyTemplate struct {
	gorm.Model
	Name     string          `gorm:"not null"`
	Version  string          `gorm:"not null"`
	IsSpot   bool            `gorm:"not null;default:true"`
	Manifest json.RawMessage `gorm:"type:jsonb"`
}

// StrategyInstance binds a template to a user account with a capital allocation.
type StrategyInstance struct {
	gorm.Model
	UserID     uint             `gorm:"not null;index"`
	User       User             `gorm:"foreignKey:UserID"`
	TemplateID uint             `gorm:"not null;index"`
	Template   StrategyTemplate `gorm:"foreignKey:TemplateID"`
	// Status is the runtime lifecycle state; transitions driven by SaaS orchestration.
	Status string `gorm:"not null;default:'STOPPED'"` // RUNNING / STOPPED / ERROR
}

// PortfolioState is the authoritative account snapshot for one instance,
// updated on every DeltaReport received from the Agent.
// LastProcessedBarTime is used by the cron scheduler to skip already-processed bars.
type PortfolioState struct {
	gorm.Model
	InstanceID           uint       `gorm:"uniqueIndex;not null"`
	CNYBalance           float64    `gorm:"not null;default:0"`
	DeadHold             float64    `gorm:"not null;default:0"`
	FloatHold            float64    `gorm:"not null;default:0"`
	ColdSealedHold       float64    `gorm:"not null;default:0"`
	TotalEquity          float64    `gorm:"not null;default:0"`
	LastProcessedBarTime *time.Time
}

// RuntimeState persists the opaque state blob produced by Step() so the
// strategy can resume deterministically after a SaaS restart.
type RuntimeState struct {
	gorm.Model
	InstanceID uint            `gorm:"uniqueIndex;not null"`
	Payload    json.RawMessage `gorm:"type:jsonb;not null"`
}

// SpotLot records an individual position lot with three-state semantics.
type SpotLot struct {
	gorm.Model
	InstanceID   uint    `gorm:"not null;index"`
	LotType      string  `gorm:"not null"` // DEAD_STACK / FLOATING / COLD_SEALED
	Amount       float64 `gorm:"not null"`
	CostPrice    float64 `gorm:"not null"`
	IsColdSealed bool    `gorm:"not null;default:false"`
}

// TradeRecord is the final settled record of one completed trade.
type TradeRecord struct {
	gorm.Model
	InstanceID    uint    `gorm:"not null;index"`
	ClientOrderID string  `gorm:"uniqueIndex;not null"` // format: inst{id}-{type}-{ts}
	Action        string  `gorm:"not null"`             // BUY / SELL
	Engine        string  `gorm:"not null"`             // MACRO / MICRO
	Symbol        string  `gorm:"not null"`
	FilledQty     float64 `gorm:"not null;default:0"`
	FilledPrice   float64 `gorm:"not null;default:0"`
	Fee           float64 `gorm:"not null;default:0"`
}

// SpotExecution tracks a single TradeCommand through its pending→filled/failed lifecycle.
type SpotExecution struct {
	gorm.Model
	InstanceID    uint    `gorm:"not null;index"`
	ClientOrderID string  `gorm:"uniqueIndex;not null"`
	Status        string  `gorm:"not null;default:'pending'"` // pending / filled / failed
	Symbol        string  `gorm:"not null"`
	Action        string  `gorm:"not null"` // BUY / SELL
	LotType       string  `gorm:"not null"` // DEAD_STACK / FLOATING
	FilledQty     float64 `gorm:"not null;default:0"`
	FilledPrice   float64 `gorm:"not null;default:0"`
	Fee           float64 `gorm:"not null;default:0"`
}

// AuditLog is an append-only event log for every significant system action.
type AuditLog struct {
	gorm.Model
	InstanceID uint            `gorm:"index"`
	EventType  string          `gorm:"not null"`
	Payload    json.RawMessage `gorm:"type:jsonb"`
}

// GeneRecord stores one parameter pack produced by the evolution engine.
type GeneRecord struct {
	gorm.Model
	StrategyID  uint            `gorm:"not null;index"`
	Role        string          `gorm:"not null"` // challenger / champion / retired
	ParamPack   json.RawMessage `gorm:"type:jsonb;not null"`
	ScoreTotal  float64         `gorm:"not null;default:0"`
	MaxDrawdown float64         `gorm:"not null;default:0"`
}

// EvolutionTask tracks one GA/DE/PSO/CMA-ES run.
type EvolutionTask struct {
	gorm.Model
	StrategyID uint            `gorm:"not null;index"`
	Status     string          `gorm:"not null;default:'pending'"` // pending / running / done / failed
	Progress   float64         `gorm:"not null;default:0"`
	Config     json.RawMessage `gorm:"type:jsonb;not null"`
}

// Backtest records a single on-demand backtest run requested via the REST API.
// ParamPack holds the chromosome + spawn point; Result is populated when the
// run completes.
type Backtest struct {
	gorm.Model
	UserID       uint            `gorm:"not null;index"`
	TemplateID   uint            `gorm:"not null;index"`
	Symbol       string          `gorm:"not null"`
	Status       string          `gorm:"not null;default:'pending'"` // pending / running / done / failed
	ParamPack    json.RawMessage `gorm:"type:jsonb;not null"`
	Result       json.RawMessage `gorm:"type:jsonb"`
	ErrorMessage string
}

// KLine stores OHLCV bars fetched from market data sources.
// The composite unique index prevents duplicate bars on the same symbol/interval/time.
type KLine struct {
	gorm.Model
	Symbol   string    `gorm:"not null;uniqueIndex:idx_kline_key"`
	Interval string    `gorm:"not null;uniqueIndex:idx_kline_key"`
	OpenTime time.Time `gorm:"not null;uniqueIndex:idx_kline_key"`
	Open     float64   `gorm:"not null"`
	High     float64   `gorm:"not null"`
	Low      float64   `gorm:"not null"`
	Close    float64   `gorm:"not null"`
	Volume   float64   `gorm:"not null"`
}
