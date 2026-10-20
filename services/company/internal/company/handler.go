package company

import (
	"log/slog"
	"net/http"

	"github.com/alfa-hack/alfa-hack/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	repo   *Repository
	logger *slog.Logger
}

func NewHandler(repo *Repository, logger *slog.Logger) *Handler {
	return &Handler{repo: repo, logger: logger}
}

func (h *Handler) List(c *gin.Context) {
	items, err := h.repo.List(c.Request.Context())
	if err != nil {
		h.logger.Error("list companies", "error", err)
		httpx.Error(c, http.StatusInternalServerError, "companies_unavailable", "Не удалось загрузить компании")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) Get(c *gin.Context) {
	item, err := h.repo.Get(c.Request.Context(), c.Param("id"))
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
