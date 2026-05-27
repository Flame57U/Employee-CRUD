package store

import (
	"fmt"

	"github.com/quantsaas/platform/internal/saas/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// DB wraps *gorm.DB so callers import only this package.
type DB struct {
	*gorm.DB
}

// NewDB opens a Postgres connection and runs AutoMigrate for all models.
// Schema is managed exclusively through Go structs — no SQL migration files.
func NewDB(cfg *config.Config) (*DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.Database.DSN), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.AutoMigrate(
		&User{},
		&StrategyTemplate{},
		&StrategyInstance{},
		&PortfolioState{},
		&RuntimeState{},
		&SpotLot{},
		&TradeRecord{},
		&SpotExecution{},
		&AuditLog{},
		&GeneRecord{},
		&EvolutionTask{},
		&Backtest{},
		&KLine{},
	); err != nil {
		return nil, fmt.Errorf("automigrate: %w", err)
	}

	return &DB{db}, nil
}
