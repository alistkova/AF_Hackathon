package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/alfa-hack/alfa-hack/internal/platform/config"
	"github.com/alfa-hack/alfa-hack/internal/platform/log"
	"github.com/alfa-hack/alfa-hack/internal/platform/obs"
	"github.com/alfa-hack/alfa-hack/services/gateway/internal/sse"
	"github.com/gin-gonic/gin"
)

func main() {
	logger := log.New("gateway")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	companyProxy := sse.Proxy(config.String("COMPANY_URL", "http://localhost:8081"), logger)
	contentProxy := sse.Proxy(config.String("CONTENT_URL", "http://localhost:8082"), logger)
	insightProxy := sse.Proxy(config.String("INSIGHT_URL", "http://localhost:8083"), logger)
	identityProxy := sse.Proxy(config.String("IDENTITY_URL", "http://localhost:8086"), logger)

	router := gin.New()
	router.Use(gin.Recovery(), obs.RequestID())
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "gateway"})
	})
	router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		switch {
		case path == "/api/v1/companies" || strings.HasPrefix(path, "/api/v1/companies/"):
			companyProxy(c)
		case path == "/api/v1/auth" || strings.HasPrefix(path, "/api/v1/auth/"):
			identityProxy(c)
		case path == "/api/v1/me" || path == "/api/v1/access" || strings.HasPrefix(path, "/api/v1/access/"):
			identityProxy(c)
		case path == "/api/v1/news" || strings.HasPrefix(path, "/api/v1/news/"):
			contentProxy(c)
		case path == "/api/v1/feed" || strings.HasPrefix(path, "/api/v1/insights/"):
			insightProxy(c)
		default:
			c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "Маршрут не найден"})
		}
	})

	server := &http.Server{
		Addr:              config.String("GATEWAY_ADDR", ":8080"),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("gateway started", "addr", server.Addr)
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
