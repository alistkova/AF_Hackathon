package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alfa-hack/alfa-hack/internal/platform/config"
	"github.com/alfa-hack/alfa-hack/internal/platform/db"
	"github.com/alfa-hack/alfa-hack/internal/platform/log"
	"github.com/alfa-hack/alfa-hack/internal/platform/obs"
	"github.com/alfa-hack/alfa-hack/services/insight/internal/insight"
	insightmigrations "github.com/alfa-hack/alfa-hack/services/insight/migrations"
	"github.com/gin-gonic/gin"
)

func main() {
	logger := log.New("insight")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, config.String("INSIGHT_DATABASE_URL", "postgres://insight_svc:insight@localhost:5432/insight_db?sslmode=disable"))
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, insightmigrations.FS, "."); err != nil {
		logger.Error("migrate database", "error", err)
		os.Exit(1)
	}
	if os.Getenv("MIGRATE_ONLY") == "1" {
		logger.Info("migrations applied")
		return
	}

	delay := config.Duration("AGENT_STEP_DELAY", 350*time.Millisecond)
	var analyzer insight.Analyzer
	if config.String("ANALYZER_MODE", "stub") == "agent" {
		analyzer = insight.NewAgentAnalyzer(
			config.String("AGENT_URL", "http://localhost:8090"),
			config.Duration("AGENT_TIMEOUT", 120*time.Second),
		)
	} else {
		analyzer = insight.NewStubAnalyzer(delay)
	}

	jobs := insight.NewManager(analyzer, logger)
	companyClient := insight.NewCompanyClient(config.String("COMPANY_URL", "http://localhost:8081"))
	handler := insight.NewHandler(jobs, companyClient)

	router := gin.New()
	router.Use(gin.Recovery(), obs.RequestID())
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "insight"})
	})
	v1 := router.Group("/api/v1")
	v1.GET("/feed", handler.Feed)
	v1.POST("/insights/jobs", handler.CreateJob)
	v1.GET("/insights/jobs/:jobId", handler.GetJob)
	v1.GET("/insights/jobs/:jobId/events", handler.StreamJob)

	server := &http.Server{
		Addr:              config.String("INSIGHT_ADDR", ":8083"),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("insight service started", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("serve", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", "error", err)
	}
}
