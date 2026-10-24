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
	"github.com/alfa-hack/alfa-hack/services/content/internal/content"
	contentmigrations "github.com/alfa-hack/alfa-hack/services/content/migrations"
	"github.com/gin-gonic/gin"
)

func main() {
	logger := log.New("content")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, config.String("CONTENT_DATABASE_URL", "postgres://content_svc:content@localhost:5432/content_db?sslmode=disable"))
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, contentmigrations.FS, "."); err != nil {
		logger.Error("migrate database", "error", err)
		os.Exit(1)
	}
	if os.Getenv("MIGRATE_ONLY") == "1" {
		logger.Info("migrations applied")
		return
	}

	repo := content.NewRepository(pool)
	files := content.NewLocalStorage(config.String("CONTENT_DATA_DIR", "./.data/content"))
	handler := content.NewHandler(repo, files, logger)
	router := gin.New()
	router.Use(gin.Recovery(), obs.RequestID())
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "content"})
	})
	router.GET("/api/v1/news", handler.List)
	router.GET("/api/v1/news/:id", handler.Get)
	router.GET("/api/v1/search", handler.Search)
	router.POST("/api/v1/documents", handler.Ingest)
	router.POST("/api/v1/documents/pdf", handler.UploadPDF)
	router.GET("/api/v1/documents/:id/chunks", handler.Chunks)
	router.POST("/api/v1/rag/chunks", handler.UpsertRAGChunks)
	router.POST("/api/v1/rag/chunks/search", handler.SearchRAGChunks)
	router.POST("/api/v1/rag/chunks/list", handler.ListRAGChunks)
	router.GET("/api/v1/rag/chunks/count", handler.CountRAGChunks)
	router.DELETE("/api/v1/rag/chunks", handler.DeleteRAGChunks)

	server := &http.Server{
		Addr:              config.String("CONTENT_ADDR", ":8082"),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("content service started", "addr", server.Addr)
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
