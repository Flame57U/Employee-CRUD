package cron

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/quantsaas/platform/internal/saas/instance"
	"github.com/quantsaas/platform/internal/saas/store"
)

// Scheduler drives the one-minute base scan that advances RUNNING instances.
// Each scan fans out one goroutine per instance so a slow tick does not
// block the rest.
type Scheduler struct {
	db      *store.DB
	manager *instance.Manager
}

// New constructs a Scheduler.
func New(db *store.DB, manager *instance.Manager) *Scheduler {
	return &Scheduler{db: db, manager: manager}
}

// Start begins the one-minute base scan. It blocks until ctx is cancelled.
// Designed to run in a dedicated goroutine:
//
//	go scheduler.Start(ctx)
func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	log.Println("[cron] scheduler started, base scan interval: 1m")
	for {
		select {
		case <-ctx.Done():
			log.Println("[cron] scheduler stopped")
			return
		case <-ticker.C:
			s.scan(ctx)
		}
	}
}

// scan fetches all RUNNING instances and concurrently calls Tick for each.
func (s *Scheduler) scan(ctx context.Context) {
	var instances []store.StrategyInstance
	if err := s.db.WithContext(ctx).
		Preload("Template").
		Where("status = ?", store.InstanceStatusRunning).
		Find(&instances).Error; err != nil {
		log.Printf("[cron] scan error: %v", err)
		return
	}

	if len(instances) == 0 {
		return
	}

	var wg sync.WaitGroup
	for _, inst := range instances {
		wg.Add(1)
		go func(i store.StrategyInstance) {
			defer wg.Done()
			s.manager.Tick(ctx, i)
		}(inst)
	}
	wg.Wait()
}
