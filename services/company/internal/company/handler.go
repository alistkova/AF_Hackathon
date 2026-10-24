package company

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/alfa-hack/alfa-hack/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

type Store interface {
	List(ctx context.Context, ids []string) ([]Company, error)
	Get(ctx context.Context, id string) (Company, error)
	GetWithPeople(ctx context.Context, id string) (CompanyDetails, error)
	ListPeople(ctx context.Context, companyID string) ([]CompanyPerson, error)
	ListRelations(ctx context.Context, companyID string) ([]CompanyRelation, error)
}

type Handler struct {
	repo   Store
	logger *slog.Logger
}

func NewHandler(repo Store, logger *slog.Logger) *Handler {
	return &Handler{repo: repo, logger: logger}
}

func (h *Handler) List(c *gin.Context) {
	items, err := h.repo.List(c.Request.Context(), companyFilter(c))
	if err != nil {
		h.logger.Error("list companies", "error", err)
		httpx.Error(c, http.StatusInternalServerError, "companies_unavailable", "Не удалось загрузить компании")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func companyFilter(c *gin.Context) []string {
	raw, ok := c.GetQueryArray("ids")
	if !ok {
		return nil
	}

	seen := make(map[string]struct{}, len(raw))
	ids := make([]string, 0, len(raw))
	for _, value := range raw {
		for _, id := range strings.Split(value, ",") {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, duplicate := seen[id]; duplicate {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	return ids
}

func (h *Handler) Get(c *gin.Context) {
	item, err := h.repo.GetWithPeople(c.Request.Context(), c.Param("id"))
	if IsNotFound(err) {
		httpx.Error(c, http.StatusNotFound, "company_not_found", "Компания не найдена")
		return
	}
	if err != nil {
		h.logger.Error("get company", "error", err, "company_id", c.Param("id"))
		httpx.Error(c, http.StatusInternalServerError, "company_unavailable", "Не удалось загрузить компанию")
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) Relations(c *gin.Context) {
	items, err := h.repo.ListRelations(c.Request.Context(), c.Param("id"))
	if IsNotFound(err) {
		httpx.Error(c, http.StatusNotFound, "company_not_found", "Компания не найдена")
		return
	}
	if err != nil {
		h.logger.Error("list company relations", "error", err, "company_id", c.Param("id"))
		httpx.Error(c, http.StatusInternalServerError, "company_unavailable", "Не удалось загрузить компанию")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) People(c *gin.Context) {
	items, err := h.repo.ListPeople(c.Request.Context(), c.Param("id"))
	if IsNotFound(err) {
		httpx.Error(c, http.StatusNotFound, "company_not_found", "Компания не найдена")
		return
	}
	if err != nil {
		h.logger.Error("list company people", "error", err, "company_id", c.Param("id"))
		httpx.Error(c, http.StatusInternalServerError, "company_unavailable", "Не удалось загрузить компанию")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
