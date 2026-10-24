package server

import (
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/alfa-hack/alfa-hack/internal/platform/httpx"
	"github.com/alfa-hack/alfa-hack/internal/platform/obs"
	"github.com/alfa-hack/alfa-hack/services/gateway/internal/auth"
	"github.com/alfa-hack/alfa-hack/services/gateway/internal/sse"
	"github.com/gin-gonic/gin"
)

const (
	contextUser  = "auth_user"
	contextToken = "auth_token"
)

type Config struct {
	CompanyURL  string
	ContentURL  string
	InsightURL  string
	IdentityURL string
	CookieName  string
	Logger      *slog.Logger
}

func New(cfg Config) *gin.Engine {
	companyProxy := sse.Proxy(cfg.CompanyURL, cfg.Logger)
	contentProxy := sse.Proxy(cfg.ContentURL, cfg.Logger)
	insightProxy := sse.Proxy(cfg.InsightURL, cfg.Logger)
	identityProxy := sse.Proxy(cfg.IdentityURL, cfg.Logger)
	guard := &guard{
		identity:   auth.NewClient(cfg.IdentityURL),
		cookieName: cfg.CookieName,
		logger:     cfg.Logger,
	}

	router := gin.New()
	router.Use(gin.Recovery(), obs.RequestID())
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "gateway"})
	})

	companies := router.Group("/api/v1/companies")
	companies.Use(guard.session(), guard.companyAccess())
	companies.GET("", companyProxy)
	companies.GET("/:id", companyProxy)
	companies.GET("/:id/people", companyProxy)
	companies.GET("/:id/relations", companyProxy)

	router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		switch {
		case path == "/api/v1/auth" || strings.HasPrefix(path, "/api/v1/auth/"):
			identityProxy(c)
		case path == "/api/v1/me" || strings.HasPrefix(path, "/api/v1/me/"):
			identityProxy(c)
		case path == "/api/v1/access" || strings.HasPrefix(path, "/api/v1/access/"):
			identityProxy(c)
		case path == "/api/v1/news" || strings.HasPrefix(path, "/api/v1/news/"):
			contentProxy(c)
		case path == "/api/v1/search":
			contentProxy(c)
		case path == "/api/v1/feed" || strings.HasPrefix(path, "/api/v1/insights/"):
			insightProxy(c)
		default:
			c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "Маршрут не найден"})
		}
	})

	return router
}

type guard struct {
	identity   *auth.Client
	cookieName string
	logger     *slog.Logger
}

func (g *guard) session() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := g.extractToken(c)
		if token == "" {
			httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Требуется авторизация")
			return
		}

		user, err := g.identity.Me(c.Request.Context(), token)
		if errors.Is(err, auth.ErrUnauthorized) {
			httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Сессия недействительна")
			return
		}
		if err != nil {
			g.fail(c, "resolve session", err)
			return
		}

		c.Set(contextUser, user)
		c.Set(contextToken, token)
		c.Next()
	}
}

func (g *guard) companyAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := currentUser(c)
		if !ok {
			httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Требуется авторизация")
			return
		}
		if user.Role == auth.RoleAdmin {
			c.Next()
			return
		}

		allowed, err := g.identity.Companies(c.Request.Context(), c.GetString(contextToken))
		if errors.Is(err, auth.ErrUnauthorized) {
			httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Сессия недействительна")
			return
		}
		if err != nil {
			g.fail(c, "resolve company access", err)
			return
		}

		if c.Request.URL.Path == "/api/v1/companies" {
			if len(allowed) == 0 {
				c.AbortWithStatusJSON(http.StatusOK, gin.H{"items": []string{}})
				return
			}
			query := c.Request.URL.Query()
			query.Del("ids")
			for _, id := range allowed {
				query.Add("ids", id)
			}
			c.Request.URL.RawQuery = query.Encode()
			c.Next()
			return
		}

		if !slices.Contains(allowed, companyIDFromPath(c.Request.URL.Path)) {
			httpx.Error(c, http.StatusNotFound, "company_not_found", "Компания не найдена")
			return
		}
		c.Next()
	}
}

func (g *guard) fail(c *gin.Context, action string, err error) {
	g.logger.Error(action, "error", err, "request_id", c.GetString("request_id"))
	httpx.Error(c, http.StatusServiceUnavailable, "identity_unavailable", "Сервис идентификации недоступен")
}

func (g *guard) extractToken(c *gin.Context) string {
	if header := c.GetHeader("Authorization"); header != "" {
		parts := strings.Fields(header)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return parts[1]
		}
	}
	if g.cookieName == "" {
		return ""
	}
	if value, err := c.Cookie(g.cookieName); err == nil {
		return value
	}
	return ""
}

func currentUser(c *gin.Context) (auth.User, bool) {
	value, exists := c.Get(contextUser)
	if !exists {
		return auth.User{}, false
	}
	user, ok := value.(auth.User)
	return user, ok
}

func companyIDFromPath(path string) string {
	rest, found := strings.CutPrefix(path, "/api/v1/companies/")
	if !found {
		return ""
	}
	id, _, _ := strings.Cut(rest, "/")
	return id
}
