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
	logger := log.New("insight-worker")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, config.String("INSIGHT_DATABASE_URL", "postgres://insight_svc:insight@localhost:5432/insight_db?sslmode=disable"))
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	logger.Info("insight worker started")
	<-ctx.Done()
	logger.Info("insight worker stopped")
}
