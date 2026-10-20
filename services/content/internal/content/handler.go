package content

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
	items, err := h.repo.ListDocuments(c.Request.Context(), 50)
	if err != nil {
		h.logger.Error("list documents", "error", err)
		httpx.Error(c, http.StatusInternalServerError, "content_unavailable", "Не удалось загрузить новости")
		return
	}
	c.JSON(http.StatusOK, items)
}
