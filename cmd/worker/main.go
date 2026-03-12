package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/m4rcel-lol/cliverse/internal/activitypub"
	"github.com/m4rcel-lol/cliverse/internal/config"
	"github.com/m4rcel-lol/cliverse/internal/db"
	"github.com/m4rcel-lol/cliverse/internal/federation"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("load config", zap.Error(err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	database, err := db.New(ctx, cfg.DatabaseDSN)
	cancel()
	if err != nil {
		logger.Fatal("connect db", zap.Error(err))
	}
	defer database.Close()

	deliverer := activitypub.NewDeliverer(database, logger)
	inboxProcessor := activitypub.NewInboxProcessor(database, cfg, logger, deliverer)
	worker := federation.NewWorker(database, cfg, logger, inboxProcessor)

	workerCtx, workerCancel := context.WithCancel(context.Background())
	go worker.Start(workerCtx)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("worker shutting down")
	workerCancel()
	time.Sleep(2 * time.Second)
	os.Exit(0)
}
