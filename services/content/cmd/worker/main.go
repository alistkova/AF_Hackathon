package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/alfa-hack/alfa-hack/internal/platform/config"
	"github.com/alfa-hack/alfa-hack/internal/platform/db"
	"github.com/alfa-hack/alfa-hack/internal/platform/log"
)

func main() {
	logger := log.New("content-worker")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, config.String("CONTENT_DATABASE_URL", "postgres://content_svc:content@localhost:5432/content_db?sslmode=disable"))
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	logger.Info("content worker started")
	<-ctx.Done()
	logger.Info("content worker stopped")
}
