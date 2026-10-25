package insight

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alfa-hack/alfa-hack/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

const (
	defaultFeedLimit = 30
	maxFeedLimit     = 50
	feedNewsWindow   = 200
)

type companyProvider interface {
	Get(ctx context.Context, id string) (Company, error)
}

type newsProvider interface {
	ListNews(ctx context.Context, limit int) ([]NewsItem, error)
}

type Handler struct {
	jobs    *Manager
	company companyProvider
	news    newsProvider
	logger  *slog.Logger
}

type CreateJobRequest struct {
	CompanyID string       `json:"company_id"`
	Event     EventContext `json:"event"`
	Evidence  []Evidence   `json:"evidence"`
}

func NewHandler(jobs *Manager, company companyProvider, news newsProvider, logger *slog.Logger) *Handler {
	return &Handler{jobs: jobs, company: company, news: news, logger: logger}
}

func (h *Handler) Feed(c *gin.Context) {
	companyID := strings.TrimSpace(c.Query("company_id"))
	if companyID == "" {
		httpx.Error(c, http.StatusBadRequest, "company_required", "Не указана компания")
		return
	}
	limit, offset, ok := feedPagination(c)
	if !ok {
		return
	}

	company, err := h.company.Get(c.Request.Context(), companyID)
	if errors.Is(err, ErrCompanyNotFound) {
		httpx.Error(c, http.StatusNotFound, "company_not_found", "Компания не найдена")
		return
	}
	if err != nil {
		h.logger.Error("load feed company", "error", err, "company_id", companyID, "request_id", c.GetString("request_id"))
		httpx.Error(c, http.StatusBadGateway, "company_unavailable", "Не удалось получить профиль компании")
		return
	}

	news, err := h.news.ListNews(c.Request.Context(), feedNewsWindow)
	if err != nil {
		h.logger.Error("load feed news", "error", err, "company_id", companyID, "request_id", c.GetString("request_id"))
		httpx.Error(c, http.StatusBadGateway, "content_unavailable", "Не удалось загрузить ленту новостей")
		return
	}

	items := RankFeed(company, news, time.Now().UTC())
	total := len(items)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items[offset:end],
		"total": total,
	})
}

func feedPagination(c *gin.Context) (int, int, bool) {
	limit := defaultFeedLimit
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			httpx.Error(c, http.StatusBadRequest, "invalid_request", "Параметр limit должен быть положительным числом")
			return 0, 0, false
		}
		if parsed > maxFeedLimit {
			parsed = maxFeedLimit
		}
		limit = parsed
	}

	offset := 0
	if raw := strings.TrimSpace(c.Query("offset")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			httpx.Error(c, http.StatusBadRequest, "invalid_request", "Параметр offset должен быть неотрицательным числом")
			return 0, 0, false
		}
		offset = parsed
	}

	return limit, offset, true
}

func (h *Handler) CreateJob(c *gin.Context) {
	var req CreateJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Некорректный запрос")
		return
	}
	if req.CompanyID == "" {
		httpx.Error(c, http.StatusBadRequest, "company_required", "Не указана компания")
		return
	}
	if req.Event.Title == "" {
		httpx.Error(c, http.StatusBadRequest, "event_required", "Не указано событие")
		return
	}
	if req.Event.ID == "" {
		req.Event.ID = "event-" + newID()
	}
	if req.Event.URL == "" {
		req.Event.URL = "https://example.com/news/" + req.Event.ID
	}

	company, err := h.company.Get(c.Request.Context(), req.CompanyID)
	if err != nil {
		httpx.Error(c, http.StatusBadGateway, "company_unavailable", "Не удалось получить профиль компании")
		return
	}

	evidence := req.Evidence
	if len(evidence) == 0 {
		evidence = []Evidence{{
			SourceID:    req.Event.ID,
			Title:       req.Event.Title,
			URL:         req.Event.URL,
			Snippet:     req.Event.Summary,
			PublishedAt: req.Event.PublishedAt,
		}}
	}

	job := h.jobs.Create(company, req.Event, evidence)
	c.JSON(http.StatusAccepted, gin.H{
		"job_id":     job.ID,
		"company_id": job.CompanyID,
		"status":     job.Status,
		"events_url": "/api/v1/insights/jobs/" + job.ID + "/events",
		"result_url": "/api/v1/insights/jobs/" + job.ID,
	})
}

func (h *Handler) GetJob(c *gin.Context) {
	job, ok := h.jobs.Snapshot(c.Param("jobId"))
	if !ok {
		httpx.Error(c, http.StatusNotFound, "job_not_found", "Задача не найдена")
		return
	}

	response := gin.H{
		"job_id":     job.ID,
		"company_id": job.CompanyID,
		"event_id":   job.EventID,
		"status":     job.Status,
		"created_at": job.CreatedAt,
		"events_url": "/api/v1/insights/jobs/" + job.ID + "/events",
	}
	if job.Insight != nil {
		response["insight"] = job.Insight
	}
	if job.Error != "" {
		response["error"] = job.Error
	}
	c.JSON(http.StatusOK, response)
}

func (h *Handler) StreamJob(c *gin.Context) {
	jobID := c.Param("jobId")
	if _, ok := h.jobs.Snapshot(jobID); !ok {
		httpx.Error(c, http.StatusNotFound, "job_not_found", "Задача не найдена")
		return
	}

	after := int64(0)
	if raw := c.GetHeader("Last-Event-ID"); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed > 0 {
			after = parsed
		}
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		httpx.Error(c, http.StatusInternalServerError, "sse_unavailable", "SSE недоступен")
		return
	}

	if _, err := fmt.Fprint(c.Writer, "retry: 3000\n\n"); err != nil {
		return
	}
	flusher.Flush()

	last := after
	writeNew := func() bool {
		events, terminal, ok := h.jobs.EventsSince(jobID, last)
		if !ok {
			return true
		}
		for _, event := range events {
			if _, err := fmt.Fprintf(c.Writer, "id: %d\nevent: %s\ndata: %s\n\n", event.Seq, event.Type, event.Data); err != nil {
				return true
			}
			last = event.Seq
		}
		flusher.Flush()
		return terminal
	}

	if writeNew() {
		return
	}

	events := time.NewTicker(300 * time.Millisecond)
	defer events.Stop()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-heartbeat.C:
			if _, err := fmt.Fprint(c.Writer, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-events.C:
			if writeNew() {
				return
			}
		}
	}
}
