package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
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
	contextUser      = "auth_user"
	contextToken     = "auth_token"
	contextCompanies = "auth_companies"
)

const maxJobBody = 1 << 20

type Config struct {
	CompanyURL    string
	ContentURL    string
	InsightURL    string
	IdentityURL   string
	InternalToken string
	CookieName    string
	Logger        *slog.Logger
}

func New(cfg Config) *gin.Engine {
	companyProxy := sse.Proxy(cfg.CompanyURL, cfg.Logger)
	contentProxy := sse.Proxy(cfg.ContentURL, cfg.Logger)
	insightProxy := sse.Proxy(cfg.InsightURL, cfg.Logger)
	identityProxy := sse.Proxy(cfg.IdentityURL, cfg.Logger)
	guard := &guard{
		identity:   auth.NewClient(cfg.IdentityURL, cfg.InternalToken),
		insights:   newInsightClient(cfg.InsightURL),
		companies:  newCompanyClient(cfg.CompanyURL),
		cookieName: cfg.CookieName,
		logger:     cfg.Logger,
	}

	router := gin.New()
	router.Use(gin.Recovery(), obs.RequestID())
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "gateway"})
	})

	router.POST("/api/v1/companies", guard.session(), guard.createCompany)

	companies := router.Group("/api/v1/companies")
	companies.Use(guard.session())
	companies.GET("", guard.companyAccess(), companyProxy)
	companies.GET("/:id", guard.companyAccess(), companyProxy)
	companies.GET("/:id/people", guard.companyAccess(), companyProxy)
	companies.GET("/:id/relations", guard.companyAccess(), companyProxy)
	companies.GET("/:id/members", guard.companyManageAccess(), identityProxy)
	companies.POST("/:id/members", guard.companyManageAccess(), identityProxy)
	companies.PATCH("/:id/members/:userId", guard.companyManageAccess(), identityProxy)
	companies.DELETE("/:id/members/:userId", guard.companyManageAccess(), identityProxy)
	companies.PUT("/:id", guard.companyOwnerAccess(), guard.updateCompany)

	access := router.Group("/api/v1/access")
	access.Use(guard.session(), guard.requireAdmin())
	access.POST("/grant", guard.grantCompanyAccess(), identityProxy)
	access.POST("/revoke", identityProxy)

	feed := router.Group("/api/v1/feed")
	feed.Use(guard.session(), guard.feedAccess())
	feed.GET("", insightProxy)

	jobs := router.Group("/api/v1/insights/jobs")
	jobs.Use(guard.session())
	jobs.POST("", guard.jobCreateAccess(), insightProxy)
	jobs.GET("/:jobId", guard.jobReadAccess(), insightProxy)
	jobs.GET("/:jobId/events", guard.jobReadAccess(), insightProxy)

	documents := router.Group("/api/v1/documents")
	documents.Use(guard.session(), guard.requireAdmin())
	documents.POST("", contentProxy)
	documents.POST("/pdf", contentProxy)
	documents.GET("/:id/chunks", contentProxy)

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
		default:
			c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "Маршрут не найден"})
		}
	})

	return router
}

type guard struct {
	identity   *auth.Client
	insights   *insightClient
	companies  *companyClient
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
			g.failIdentity(c, "resolve session", err)
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
		if user.Role == auth.RoleAdmin && c.Request.URL.Path != "/api/v1/companies" {
			c.Next()
			return
		}

		allowed, ok := g.allowedCompanies(c)
		if !ok {
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

func (g *guard) companyManageAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := currentUser(c)
		if !ok {
			httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Требуется авторизация")
			return
		}

		companyID := companyIDFromPath(c.Request.URL.Path)
		if companyID == "" {
			httpx.Error(c, http.StatusNotFound, "company_not_found", "Компания не найдена")
			return
		}
		if !g.ensureCompanyExists(c, companyID) {
			return
		}
		if user.Role == auth.RoleAdmin {
			c.Next()
			return
		}

		allowed, ok := g.allowedCompanies(c)
		if !ok {
			return
		}
		if !slices.Contains(allowed, companyID) {
			httpx.Error(c, http.StatusNotFound, "company_not_found", "Компания не найдена")
			return
		}
		c.Next()
	}
}

func (g *guard) companyOwnerAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := currentUser(c)
		if !ok {
			httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Требуется авторизация")
			return
		}

		companyID := companyIDFromPath(c.Request.URL.Path)
		if companyID == "" {
			httpx.Error(c, http.StatusNotFound, "company_not_found", "Компания не найдена")
			return
		}
		if !g.ensureCompanyExists(c, companyID) {
			return
		}
		if user.Role == auth.RoleAdmin {
			c.Next()
			return
		}

		role, err := g.identity.Role(c.Request.Context(), c.GetString(contextToken), companyID)
		if errors.Is(err, auth.ErrRoleNotFound) {
			httpx.Error(c, http.StatusNotFound, "company_not_found", "Компания не найдена")
			return
		}
		if err != nil {
			g.failIdentity(c, "resolve company role", err)
			return
		}
		if role != auth.RoleOwner {
			httpx.Error(c, http.StatusForbidden, "forbidden", "Недостаточно прав")
			return
		}
		c.Next()
	}
}

func (g *guard) ensureCompanyExists(c *gin.Context, companyID string) bool {
	exists, err := g.companies.Exists(c.Request.Context(), companyID)
	if err != nil {
		g.logger.Error("check company", "error", err, "company_id", companyID, "request_id", c.GetString("request_id"))
		httpx.Error(c, http.StatusBadGateway, "company_unavailable", "Сервис компаний недоступен")
		return false
	}
	if !exists {
		httpx.Error(c, http.StatusNotFound, "company_not_found", "Компания не найдена")
		return false
	}
	return true
}

func (g *guard) grantCompanyAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := readBody(c, 4<<10)
		if err != nil {
			httpx.Error(c, http.StatusBadRequest, "invalid_request", "Некорректный запрос")
			return
		}
		var payload struct {
			CompanyID string `json:"company_id"`
		}
		if err := json.Unmarshal(body, &payload); err != nil || strings.TrimSpace(payload.CompanyID) == "" {
			httpx.Error(c, http.StatusBadRequest, "invalid_request", "Укажите company_id")
			return
		}
		if !g.ensureCompanyExists(c, strings.TrimSpace(payload.CompanyID)) {
			return
		}
		c.Next()
	}
}

func (g *guard) feedAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		companyID := strings.TrimSpace(c.Query("company_id"))
		if companyID == "" {
			httpx.Error(c, http.StatusBadRequest, "company_required", "Не указана компания")
			return
		}
		if !g.allowCompany(c, companyID) {
			return
		}
		c.Next()
	}
}

func (g *guard) jobCreateAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := readBody(c, maxJobBody)
		if err != nil {
			httpx.Error(c, http.StatusBadRequest, "invalid_request", "Некорректный запрос")
			return
		}

		var payload struct {
			CompanyID string `json:"company_id"`
		}
		if err := json.Unmarshal(body, &payload); err != nil || strings.TrimSpace(payload.CompanyID) == "" {
			httpx.Error(c, http.StatusBadRequest, "invalid_request", "Некорректный запрос")
			return
		}

		if !g.allowCompany(c, strings.TrimSpace(payload.CompanyID)) {
			return
		}
		c.Next()
	}
}

func (g *guard) jobReadAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		companyID, err := g.insights.CompanyID(c.Request.Context(), c.Param("jobId"))
		if errors.Is(err, errJobNotFound) {
			httpx.Error(c, http.StatusNotFound, "job_not_found", "Задача не найдена")
			return
		}
		if err != nil {
			g.failUpstream(c, "resolve job", err)
			return
		}
		if !g.allowCompany(c, companyID) {
			return
		}
		c.Next()
	}
}

func (g *guard) requireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := currentUser(c)
		if !ok {
			httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Требуется авторизация")
			return
		}
		if user.Role != auth.RoleAdmin {
			httpx.Error(c, http.StatusForbidden, "forbidden", "Недостаточно прав")
			return
		}
		c.Next()
	}
}

func (g *guard) allowCompany(c *gin.Context, companyID string) bool {
	if _, ok := currentUser(c); !ok {
		httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Требуется авторизация")
		return false
	}

	allowed, ok := g.allowedCompanies(c)
	if !ok {
		return false
	}
	if !slices.Contains(allowed, companyID) {
		httpx.Error(c, http.StatusNotFound, "company_not_found", "Компания не найдена")
		return false
	}
	return true
}

func (g *guard) allowedCompanies(c *gin.Context) ([]string, bool) {
	if cached, ok := c.Get(contextCompanies); ok {
		allowed, _ := cached.([]string)
		return allowed, true
	}

	allowed, err := g.identity.Companies(c.Request.Context(), c.GetString(contextToken))
	if errors.Is(err, auth.ErrUnauthorized) {
		httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Сессия недействительна")
		return nil, false
	}
	if err != nil {
		g.failIdentity(c, "resolve company access", err)
		return nil, false
	}

	c.Set(contextCompanies, allowed)
	return allowed, true
}

func (g *guard) failIdentity(c *gin.Context, action string, err error) {
	g.logger.Error(action, "error", err, "request_id", c.GetString("request_id"))
	httpx.Error(c, http.StatusServiceUnavailable, "identity_unavailable", "Сервис идентификации недоступен")
}

func (g *guard) failUpstream(c *gin.Context, action string, err error) {
	g.logger.Error(action, "error", err, "request_id", c.GetString("request_id"))
	httpx.Error(c, http.StatusServiceUnavailable, "upstream_unavailable", "Сервис временно недоступен")
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

func readBody(c *gin.Context, limit int64) ([]byte, error) {
	if c.Request.Body == nil {
		return nil, errors.New("empty body")
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, errors.New("body too large")
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}
