package content

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/alfa-hack/alfa-hack/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

func (h *Handler) UpsertRAGChunks(c *gin.Context) {
	var payload RAGUpsertRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Некорректный запрос")
		return
	}
	if len(payload.Chunks) == 0 {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Список чанков пуст")
		return
	}
	for _, chunk := range payload.Chunks {
		if err := validateRAGChunk(chunk); err != nil {
			httpx.Error(c, http.StatusBadRequest, "invalid_chunk", err.Error())
			return
		}
	}

	upserted, err := h.repo.UpsertRAGChunks(c.Request.Context(), payload.Chunks)
	if err != nil {
		h.logger.Error("upsert rag chunks", "error", err)
		httpx.Error(c, http.StatusInternalServerError, "content_unavailable", "Не удалось сохранить чанки")
		return
	}
	c.JSON(http.StatusOK, RAGUpsertResponse{Upserted: upserted})
}

func (h *Handler) SearchRAGChunks(c *gin.Context) {
	var payload RAGSearchRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Некорректный запрос")
		return
	}
	if err := validateEmbedding(payload.Embedding); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_embedding", err.Error())
		return
	}
	if err := validateRAGFilter(payload.RAGFilter); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_filter", err.Error())
		return
	}
	if payload.MaxDistance != nil && *payload.MaxDistance < 0 {
		httpx.Error(c, http.StatusBadRequest, "invalid_filter", "Максимальная дистанция не может быть отрицательной")
		return
	}

	chunks, err := h.repo.SearchRAGChunks(c.Request.Context(), RAGSearchQuery{
		RAGFilter:   payload.RAGFilter,
		Embedding:   payload.Embedding,
		TopK:        normalizeTopK(payload.TopK, ragDefaultSearchTopK),
		MaxDistance: payload.MaxDistance,
	})
	if err != nil {
		h.logger.Error("search rag chunks", "error", err)
		httpx.Error(c, http.StatusInternalServerError, "content_unavailable", "Не удалось найти чанки")
		return
	}
	c.JSON(http.StatusOK, RAGChunksResponse{Chunks: chunks})
}

func (h *Handler) ListRAGChunks(c *gin.Context) {
	var payload RAGListRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Некорректный запрос")
		return
	}
	if err := validateRAGFilter(payload.RAGFilter); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_filter", err.Error())
		return
	}

	chunks, err := h.repo.ListRAGChunks(c.Request.Context(), RAGListQuery{
		RAGFilter: payload.RAGFilter,
		TopK:      normalizeTopK(payload.TopK, ragDefaultListTopK),
	})
	if err != nil {
		h.logger.Error("list rag chunks", "error", err)
		httpx.Error(c, http.StatusInternalServerError, "content_unavailable", "Не удалось загрузить чанки")
		return
	}
	c.JSON(http.StatusOK, RAGChunksResponse{Chunks: chunks})
}

func (h *Handler) CountRAGChunks(c *gin.Context) {
	count, err := h.repo.CountRAGChunks(c.Request.Context())
	if err != nil {
		h.logger.Error("count rag chunks", "error", err)
		httpx.Error(c, http.StatusInternalServerError, "content_unavailable", "Не удалось посчитать чанки")
		return
	}
	c.JSON(http.StatusOK, RAGCountResponse{Count: count})
}

func (h *Handler) DeleteRAGChunks(c *gin.Context) {
	deleted, err := h.repo.DeleteRAGChunks(c.Request.Context())
	if err != nil {
		h.logger.Error("delete rag chunks", "error", err)
		httpx.Error(c, http.StatusInternalServerError, "content_unavailable", "Не удалось удалить чанки")
		return
	}
	c.JSON(http.StatusOK, RAGDeleteResponse{Deleted: deleted})
}

func validateRAGChunk(chunk RAGChunkInput) error {
	if strings.TrimSpace(chunk.Content) == "" {
		return errors.New("Текст чанка обязателен")
	}
	if strings.TrimSpace(chunk.Source) == "" {
		return errors.New("Источник чанка обязателен")
	}
	if chunk.Page < 0 || chunk.Chunk < 0 {
		return errors.New("Номер страницы и чанка не могут быть отрицательными")
	}
	if err := validateEmbedding(chunk.Embedding); err != nil {
		return err
	}
	if chunk.PublishedAt != nil && *chunk.PublishedAt != "" {
		if _, err := time.Parse("2006-01-02", *chunk.PublishedAt); err != nil {
			return errors.New("Дата публикации должна быть в формате YYYY-MM-DD")
		}
	}
	return nil
}

func validateRAGFilter(filter RAGFilter) error {
	for _, raw := range []string{filter.PublishedAfter, filter.PublishedBefore} {
		if raw == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", raw); err != nil {
			return errors.New("Дата фильтра должна быть в формате YYYY-MM-DD")
		}
	}
	return nil
}

func validateEmbedding(values []float32) error {
	if len(values) != RAGEmbeddingDimensions {
		return fmt.Errorf("Размерность эмбеддинга должна быть %d", RAGEmbeddingDimensions)
	}
	for _, value := range values {
		number := float64(value)
		if math.IsNaN(number) || math.IsInf(number, 0) {
			return errors.New("Эмбеддинг содержит недопустимые значения")
		}
	}
	return nil
}
