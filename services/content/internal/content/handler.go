package content

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/alfa-hack/alfa-hack/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

type store interface {
	ListDocuments(ctx context.Context, limit int) ([]NewsItem, error)
	UpsertRAGChunks(ctx context.Context, chunks []RAGChunkInput) (int, error)
	SearchRAGChunks(ctx context.Context, query RAGSearchQuery) ([]RAGChunk, error)
	ListRAGChunks(ctx context.Context, query RAGListQuery) ([]RAGChunk, error)
	CountRAGChunks(ctx context.Context) (int64, error)
	DeleteRAGChunks(ctx context.Context) (int64, error)
}

type Handler struct {
	repo   store
	logger *slog.Logger
}

func NewHandler(repo store, logger *slog.Logger) *Handler {
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
