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

var (
	errChunkContentRequired = errors.New("chunk content is required")
	errChunkSourceRequired  = errors.New("chunk source is required")
	errChunkOrdinalNegative = errors.New("chunk ordinals must not be negative")
	errChunkPublishedFormat = errors.New("chunk published_at must use YYYY-MM-DD")
	errFilterDateFormat     = errors.New("filter date must use YYYY-MM-DD")
	errEmbeddingDimensions  = errors.New("embedding must have 384 dimensions")
	errEmbeddingValue       = errors.New("embedding contains invalid values")
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
			code, message := ragValidationError(err)
			httpx.Error(c, http.StatusBadRequest, code, message)
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
		code, message := ragValidationError(err)
		httpx.Error(c, http.StatusBadRequest, code, message)
		return
	}
	if err := validateRAGFilter(payload.RAGFilter); err != nil {
		code, message := ragValidationError(err)
		httpx.Error(c, http.StatusBadRequest, code, message)
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
		code, message := ragValidationError(err)
		httpx.Error(c, http.StatusBadRequest, code, message)
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
		return errChunkContentRequired
	}
	if strings.TrimSpace(chunk.Source) == "" {
		return errChunkSourceRequired
	}
	if chunk.Page < 0 || chunk.Chunk < 0 {
		return errChunkOrdinalNegative
	}
	if err := validateEmbedding(chunk.Embedding); err != nil {
		return err
	}
	if chunk.PublishedAt != nil && *chunk.PublishedAt != "" {
		if _, err := time.Parse("2006-01-02", *chunk.PublishedAt); err != nil {
			return errChunkPublishedFormat
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
			return errFilterDateFormat
		}
	}
	return nil
}

func validateEmbedding(values []float32) error {
	if len(values) != RAGEmbeddingDimensions {
		return errEmbeddingDimensions
	}
	for _, value := range values {
		number := float64(value)
		if math.IsNaN(number) || math.IsInf(number, 0) {
			return errEmbeddingValue
		}
	}
	return nil
}

func ragValidationError(err error) (string, string) {
	switch {
	case errors.Is(err, errChunkContentRequired):
		return "invalid_chunk", "Текст чанка обязателен"
	case errors.Is(err, errChunkSourceRequired):
		return "invalid_chunk", "Источник чанка обязателен"
	case errors.Is(err, errChunkOrdinalNegative):
		return "invalid_chunk", "Номер страницы и чанка не могут быть отрицательными"
	case errors.Is(err, errChunkPublishedFormat):
		return "invalid_chunk", "Дата публикации должна быть в формате YYYY-MM-DD"
	case errors.Is(err, errFilterDateFormat):
		return "invalid_filter", "Дата фильтра должна быть в формате YYYY-MM-DD"
	case errors.Is(err, errEmbeddingDimensions):
		return "invalid_embedding", fmt.Sprintf(
			"Размерность эмбеддинга должна быть %d",
			RAGEmbeddingDimensions,
		)
	case errors.Is(err, errEmbeddingValue):
		return "invalid_embedding", "Эмбеддинг содержит недопустимые значения"
	}
	return "invalid_request", "Некорректный запрос"
}
