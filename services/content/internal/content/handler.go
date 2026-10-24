package content

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alfa-hack/alfa-hack/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

type store interface {
	ListDocuments(ctx context.Context, filter DocumentFilter) ([]NewsItem, int, error)
	GetDocument(ctx context.Context, id string) (NewsItem, error)
	SearchDocuments(ctx context.Context, query string, filter DocumentFilter) ([]SearchHit, int, error)
	IngestDocument(ctx context.Context, input IngestInput) (IngestResult, error)
	ListDocumentChunks(ctx context.Context, id string) ([]DocumentChunk, error)
	UpsertRAGChunks(ctx context.Context, chunks []RAGChunkInput) (int, error)
	SearchRAGChunks(ctx context.Context, query RAGSearchQuery) ([]RAGChunk, error)
	ListRAGChunks(ctx context.Context, query RAGListQuery) ([]RAGChunk, error)
	CountRAGChunks(ctx context.Context) (int64, error)
	DeleteRAGChunks(ctx context.Context) (int64, error)
}

type Handler struct {
	repo   store
	files  Storage
	logger *slog.Logger
}

func NewHandler(repo store, files Storage, logger *slog.Logger) *Handler {
	return &Handler{repo: repo, files: files, logger: logger}
}

func (h *Handler) List(c *gin.Context) {
	filter, ok := documentFilter(c)
	if !ok {
		return
	}

	items, total, err := h.repo.ListDocuments(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("list documents", "error", err, "request_id", c.GetString("request_id"))
		httpx.Error(c, http.StatusInternalServerError, "content_unavailable", "Не удалось загрузить новости")
		return
	}

	c.Header("X-Total-Count", strconv.Itoa(total))
	c.JSON(http.StatusOK, items)
}

func (h *Handler) Get(c *gin.Context) {
	item, err := h.repo.GetDocument(c.Request.Context(), c.Param("id"))
	if errors.Is(err, ErrNotFound) {
		httpx.Error(c, http.StatusNotFound, "document_not_found", "Документ не найден")
		return
	}
	if err != nil {
		h.logger.Error("get document", "error", err, "document_id", c.Param("id"))
		httpx.Error(c, http.StatusInternalServerError, "content_unavailable", "Не удалось загрузить документ")
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) Search(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	if query == "" {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Укажите поисковый запрос в параметре q")
		return
	}

	filter, ok := documentFilter(c)
	if !ok {
		return
	}

	hits, total, err := h.repo.SearchDocuments(c.Request.Context(), query, filter)
	if err != nil {
		h.logger.Error("search documents", "error", err, "request_id", c.GetString("request_id"))
		httpx.Error(c, http.StatusInternalServerError, "content_unavailable", "Не удалось выполнить поиск")
		return
	}

	c.Header("X-Total-Count", strconv.Itoa(total))
	c.JSON(http.StatusOK, gin.H{"items": hits, "total": total})
}

func (h *Handler) Chunks(c *gin.Context) {
	chunks, err := h.repo.ListDocumentChunks(c.Request.Context(), c.Param("id"))
	if errors.Is(err, ErrNotFound) {
		httpx.Error(c, http.StatusNotFound, "document_not_found", "Документ не найден")
		return
	}
	if err != nil {
		h.logger.Error("list document chunks", "error", err, "document_id", c.Param("id"))
		httpx.Error(c, http.StatusInternalServerError, "content_unavailable", "Не удалось загрузить фрагменты")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": chunks})
}

type ingestItem struct {
	DocumentID    string    `json:"document_id"`
	NewsID        string    `json:"news_id"`
	Kind          string    `json:"kind"`
	Title         string    `json:"title"`
	PublishedAt   time.Time `json:"published_at"`
	SourceName    string    `json:"source_name"`
	SourceURL     string    `json:"source_url"`
	FullText      string    `json:"full_text"`
	Tags          []string  `json:"tags"`
	TargetSectors []string  `json:"target_sectors"`
}

type ingestRequest struct {
	Items []ingestItem `json:"items" binding:"required"`
}

func (h *Handler) Ingest(c *gin.Context) {
	var request ingestRequest
	if err := c.ShouldBindJSON(&request); err != nil || len(request.Items) == 0 {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Передайте непустой массив items")
		return
	}

	results := make([]IngestResult, 0, len(request.Items))
	for _, item := range request.Items {
		if strings.TrimSpace(item.Title) == "" || strings.TrimSpace(item.FullText) == "" {
			httpx.Error(c, http.StatusBadRequest, "invalid_request", "У каждого документа должны быть title и full_text")
			return
		}
		result, err := h.repo.IngestDocument(c.Request.Context(), IngestInput{
			DocumentID:    item.DocumentID,
			NewsID:        item.NewsID,
			Kind:          item.Kind,
			Title:         item.Title,
			PublishedAt:   item.PublishedAt,
			SourceName:    item.SourceName,
			SourceURL:     item.SourceURL,
			FullText:      item.FullText,
			Tags:          item.Tags,
			TargetSectors: item.TargetSectors,
		})
		if err != nil {
			h.logger.Error("ingest document", "error", err, "news_id", item.NewsID, "document_id", item.DocumentID)
			httpx.Error(c, http.StatusInternalServerError, "content_unavailable", "Не удалось сохранить документ")
			return
		}
		results = append(results, result)
	}

	c.JSON(http.StatusOK, gin.H{"items": results})
}

func documentFilter(c *gin.Context) (DocumentFilter, bool) {
	filter := DocumentFilter{
		Kind:   strings.ToLower(strings.TrimSpace(c.Query("kind"))),
		Sector: strings.TrimSpace(c.Query("sector")),
		Tag:    strings.TrimSpace(c.Query("tag")),
		Source: strings.TrimSpace(c.Query("source")),
	}

	from, ok := parseDateParam(c, "from")
	if !ok {
		return DocumentFilter{}, false
	}
	to, ok := parseDateParam(c, "to")
	if !ok {
		return DocumentFilter{}, false
	}
	filter.From = from
	filter.To = to

	if value := c.Query("limit"); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil || limit <= 0 {
			httpx.Error(c, http.StatusBadRequest, "invalid_request", "Параметр limit должен быть положительным числом")
			return DocumentFilter{}, false
		}
		filter.Limit = limit
	}
	if value := c.Query("offset"); value != "" {
		offset, err := strconv.Atoi(value)
		if err != nil || offset < 0 {
			httpx.Error(c, http.StatusBadRequest, "invalid_request", "Параметр offset должен быть неотрицательным числом")
			return DocumentFilter{}, false
		}
		filter.Offset = offset
	}

	return filter, true
}

func parseDateParam(c *gin.Context, name string) (*time.Time, bool) {
	value := strings.TrimSpace(c.Query(name))
	if value == "" {
		return nil, true
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return &parsed, true
		}
	}
	httpx.Error(c, http.StatusBadRequest, "invalid_request", "Параметр "+name+" должен быть датой в формате 2006-01-02 или RFC3339")
	return nil, false
}
