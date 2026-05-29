// Package main is the SaaS HTTP server entry point.
//
// Bootstraps config → DB → Redis → WebSocket Hub → instance.Manager →
// evolution service, then hands them all to api.Register for routing.
package main

import (
	"flag"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/quantsaas/platform/internal/saas/api"
	"github.com/quantsaas/platform/internal/saas/auth"
	"github.com/quantsaas/platform/internal/saas/config"
	"github.com/quantsaas/platform/internal/saas/epoch"
	"github.com/quantsaas/platform/internal/saas/ga"
	"github.com/quantsaas/platform/internal/saas/instance"
	"github.com/quantsaas/platform/internal/saas/store"
	"github.com/quantsaas/platform/internal/saas/ws"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := store.NewDB(cfg)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	redis, err := store.NewRedis(cfg)
	if err != nil {
		log.Fatalf("open redis: %v", err)
	}

	authSvc := auth.NewService(cfg)
	hub := ws.NewHub(authSvc, db)

	// instance.Manager needs a Hub adapter + a market fetcher + strategy registry.
	// These are wired in upstream phases; for now this server bootstraps with
	// nil-safe defaults so the REST layer can run on its own. The instance
	// lifecycle endpoints (start/stop/delete) do not depend on the Hub adapter
	// or fetcher; the Tick cron loop does, and that is wired separately.
	mgr := instance.New(db, redis, nil, nil, nil)

	engine := ga.NewEvolutionEngine(&ga.DCABalanceEvolvable{}, db)
	evolveSvc := epoch.NewEpochService(db, engine)

	r := gin.Default()
	r.HandleMethodNotAllowed = true
	api.Register(r, api.Deps{
		DB:        db,
		Redis:     redis,
		Auth:      authSvc,
		Hub:       hub,
		Manager:   mgr,
		EvolveSvc: evolveSvc,
	})

	port := cfg.Server.Port
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	log.Printf("saas listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server: %v", err)
	}
}
