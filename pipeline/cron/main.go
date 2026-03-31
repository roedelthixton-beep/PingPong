package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"pingpong_server/pkg/config"
	"pingpong_server/pkg/db"
	"pingpong_server/pkg/es"
	"pingpong_server/pkg/logger"
	"pingpong_server/pkg/mq"
)

func main() {
	cfg := config.Load()
	logger.Init("pipeline/cron/.log")

	// Init PostgreSQL
	db.Init(cfg.PgDSN)
	log.Println("PostgreSQL connected")

	// Init Redis
	mq.Init(cfg.RedisAddr, cfg.RedisPassword)
	log.Println("Redis connected")

	// Init Elasticsearch
	if err := es.InitES(cfg.EmbeddingDimensions); err != nil {
		log.Fatalf("Failed to initialize Elasticsearch: %v", err)
	}
	log.Println("Elasticsearch connected")

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start cron jobs
	go StartAgentCountUpdater(ctx, cfg, mq.RDB)
	go StartStatsCalibrator(ctx, cfg, mq.RDB)

	log.Println("Cron service started")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down cron service...")
	cancel()

	log.Println("Cron service stopped")
}
