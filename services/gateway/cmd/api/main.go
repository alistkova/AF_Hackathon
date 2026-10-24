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
	"github.com/alfa-hack/alfa-hack/internal/platform/log"
	"github.com/alfa-hack/alfa-hack/services/gateway/internal/server"
)

func main() {
	logger := log.New("gateway")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	router := server.New(server.Config{
		CompanyURL:  config.String("COMPANY_URL", "http://localhost:8081"),
		ContentURL:  config.String("CONTENT_URL", "http://localhost:8082"),
		InsightURL:  config.String("INSIGHT_URL", "http://localhost:8083"),
		IdentityURL: config.String("IDENTITY_URL", "http://localhost:8086"),
		CookieName:  config.String("SESSION_COOKIE_NAME", "alfa_session"),
		Logger:      logger,
	})

	httpServer := &http.Server{
		Addr:              config.String("GATEWAY_ADDR", ":8080"),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("gateway started", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("serve", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", "error", err)
	}
}
