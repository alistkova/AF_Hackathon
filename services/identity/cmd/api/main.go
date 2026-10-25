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
	"github.com/alfa-hack/alfa-hack/services/identity/internal/identity"
	identitymigrations "github.com/alfa-hack/alfa-hack/services/identity/migrations"
	"github.com/gin-gonic/gin"
)

func main() {
	logger := log.New("identity")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, config.String("IDENTITY_DATABASE_URL", "postgres://identity_svc:identity@localhost:5432/identity_db?sslmode=disable"))
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, identitymigrations.FS, "."); err != nil {
		logger.Error("migrate database", "error", err)
		os.Exit(1)
	}
	if os.Getenv("MIGRATE_ONLY") == "1" {
		logger.Info("migrations applied")
		return
	}

	store := identity.NewDBStore(pool)
	pruneSessions := func() {
		removed, err := store.PruneSessions(ctx)
		if err != nil {
			logger.Error("prune sessions", "error", err)
			return
		}
		if removed > 0 {
			logger.Info("pruned sessions", "count", removed)
		}
	}
	pruneSessions()
	go func() {
		interval := config.Duration("IDENTITY_SESSION_PRUNE_INTERVAL", time.Hour)
		if interval <= 0 {
			interval = time.Hour
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pruneSessions()
			}
		}
	}()

	handler := identity.NewHandler(store, logger)
	auth := identity.AuthMiddleware(store)
	router := gin.New()
	router.Use(gin.Recovery(), obs.RequestID())
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "identity"})
	})
	v1 := router.Group("/api/v1")
	v1.POST("/auth/register", handler.Register)
	v1.POST("/auth/login", handler.Login)
	v1.POST("/auth/logout", handler.Logout)
	protected := v1.Group("")
	protected.Use(auth)
	protected.GET("/me", handler.Me)
	protected.PATCH("/me", handler.UpdateMe)
	protected.POST("/me/password", handler.ChangePassword)
	protected.GET("/me/companies", handler.AccessibleCompanies)
	protected.GET("/me/memberships", handler.Memberships)
	protected.GET("/companies/:companyId/members", handler.CompanyMembers)
	protected.POST("/companies/:companyId/members", handler.AddCompanyMember)
	protected.DELETE("/companies/:companyId/members/:userId", handler.RemoveCompanyMember)
	admin := protected.Group("/access")
	admin.Use(identity.RequireAdmin())
	admin.POST("/grant", handler.GrantAccess)
	admin.POST("/revoke", handler.RevokeAccess)

	internal := v1.Group("/internal")
	internal.POST("/memberships", handler.CreateMembership)

	server := &http.Server{
		Addr:              config.String("IDENTITY_ADDR", ":8086"),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("identity service started", "addr", server.Addr)
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
