package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/quantsaas/platform/internal/agent/broker"
	"github.com/quantsaas/platform/internal/agent/config"
	"github.com/quantsaas/platform/internal/agent/ws"
)

func main() {
	cfgPath := flag.String("config", "config.agent.yaml", "path to agent config file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("[agent] config error: %v", err)
	}

	brokerClient := broker.New(
		cfg.Broker.APIKey,
		cfg.Broker.SecretKey,
		cfg.Broker.TradePassword,
		cfg.Broker.Simulated,
	)

	agent := ws.New(cfg.SaaSURL, cfg.Email, cfg.Password, brokerClient)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("[agent] starting — connecting to %s", cfg.SaaSURL)
	agent.Run(ctx)
	log.Printf("[agent] stopped")
}
