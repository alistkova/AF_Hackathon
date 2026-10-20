package insight

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/alfa-hack/alfa-hack/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	jobs    *Manager
	company *CompanyClient
}

type CreateJobRequest struct {
	CompanyID string       `json:"company_id"`
	Event     EventContext `json:"event"`
	Evidence  []Evidence   `json:"evidence"`
}

func NewHandler(jobs *Manager, company *CompanyClient) *Handler {
	return &Handler{jobs: jobs, company: company}
}

func (h *Handler) Feed(c *gin.Context) {
	companyID := c.Query("company_id")
	items := []FeedItem{
		{
			EventID:     "news-001",
			CompanyID:   "magnit",
			Title:       "Магнит увеличивает инвестиции в логистику",
			Summary:     "Ритейлер объявил о расширении распределительных центров в Южном федеральном округе.",
			Source:      "Ведомости",
			URL:         "https://example.com/news/magnit-logistics",
			PublishedAt: "2026-10-18T09:00:00Z",
		},
		{
			EventID:     "news-002",
			CompanyID:   "severstal",
			Title:       "Северсталь ожидает роста спроса на плоский прокат",
			Summary:     "Компания видит восстановление спроса со стороны машиностроения и строительства.",
			Source:      "Коммерсантъ",
			URL:         "https://example.com/news/severstal-demand",
			PublishedAt: "2026-10-17T12:30:00Z",
		},
		{
			EventID:     "news-003",
			CompanyID:   "yandex",
			Title:       "Яндекс расширяет облачные сервисы для корпоративных клиентов",
			Summary:     "Новые продукты ориентированы на компании среднего и крупного бизнеса.",
			Source:      "РБК",
			URL:         "https://example.com/news/yandex-cloud",
			PublishedAt: "2026-10-16T15:10:00Z",
		},
	}

	if companyID == "" {
		c.JSON(http.StatusOK, gin.H{"items": items})
		return
	}

	filtered := make([]FeedItem, 0, len(items))
	for _, item := range items {
		if item.CompanyID == companyID {
			filtered = append(filtered, item)
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": filtered})
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
