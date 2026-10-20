package insight

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/alfa-hack/alfa-hack/services/insight/internal/agentclient"
)

const (
	StatusQueued  = "queued"
	StatusRunning = "running"
	StatusDone    = "done"
	StatusFailed  = "failed"
)

type StoredEvent struct {
	Seq  int64
	Type string
	Data json.RawMessage
}

type Job struct {
	ID        string
	CompanyID string
	EventID   string
	Status    string
	Events    []StoredEvent
	Insight   *Insight
	Error     string
	CreatedAt time.Time
}

type JobSnapshot struct {
	ID        string
	CompanyID string
	EventID   string
	Status    string
	Insight   *Insight
	Error     string
	CreatedAt time.Time
}

type Manager struct {
	mu       sync.RWMutex
	jobs     map[string]*Job
	analyzer Analyzer
	logger   *slog.Logger
}

func NewManager(analyzer Analyzer, logger *slog.Logger) *Manager {
	return &Manager{
		jobs:     make(map[string]*Job),
		analyzer: analyzer,
		logger:   logger,
	}
}

func (m *Manager) Create(company Company, event EventContext, evidence []Evidence) JobSnapshot {
	job := &Job{
		ID:        newID(),
		CompanyID: company.ID,
		EventID:   event.ID,
		Status:    StatusQueued,
		CreatedAt: time.Now().UTC(),
	}

	m.mu.Lock()
	m.jobs[job.ID] = job
	m.mu.Unlock()

	_ = m.append(job.ID, "status", map[string]any{"status": StatusQueued})

	go m.run(job.ID, company, event, evidence)

	return m.snapshot(job.ID)
}

func (m *Manager) run(jobID string, company Company, event EventContext, evidence []Evidence) {
	m.setStatus(jobID, StatusRunning)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	req := agentclient.AnalysisRequest{
		RequestID: jobID,
		Company: agentclient.Company{
			ID:          company.ID,
			Name:        company.Name,
			INN:         company.INN,
			Industry:    company.Industry,
			Region:      company.Region,
			Description: company.Description,
		},
		Event: agentclient.EventContext{
			ID:          event.ID,
			Title:       event.Title,
			Summary:     event.Summary,
			PublishedAt: event.PublishedAt,
		},
		Evidence: make([]agentclient.Evidence, 0, len(evidence)),
		Locale:   "ru",
		Mode:     "live",
	}
	for _, item := range evidence {
		req.Evidence = append(req.Evidence, agentclient.Evidence{
			SourceID:    item.SourceID,
			Title:       item.Title,
			URL:         item.URL,
			Snippet:     item.Snippet,
			PublishedAt: item.PublishedAt,
		})
	}

	err := m.analyzer.Analyze(ctx, req, func(event agentclient.Event) error {
		switch event.Type {
		case "step":
			var payload map[string]any
			if err := json.Unmarshal(event.Data, &payload); err != nil {
				return err
			}
			delete(payload, "type")
			return m.append(jobID, "step", payload)
		case "result":
			var payload struct {
				Insight Insight `json:"insight"`
			}
			if err := json.Unmarshal(event.Data, &payload); err != nil {
				return err
			}
			m.complete(jobID, payload.Insight)
			return nil
		case "error":
			var payload struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(event.Data, &payload); err != nil {
				return err
			}
			m.fail(jobID, payload.Code, payload.Message)
			return nil
		default:
			return nil
		}
	})
	if err != nil {
		m.fail(jobID, "agent_error", err.Error())
	}
}

func (m *Manager) Snapshot(id string) (JobSnapshot, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	job, ok := m.jobs[id]
	if !ok {
		return JobSnapshot{}, false
	}
	return snapshotOf(job), true
}

func (m *Manager) EventsSince(id string, after int64) ([]StoredEvent, bool, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	job, ok := m.jobs[id]
	if !ok {
		return nil, false, false
	}
	events := make([]StoredEvent, 0, len(job.Events))
	for _, event := range job.Events {
		if event.Seq > after {
			events = append(events, event)
		}
	}
	terminal := job.Status == StatusDone || job.Status == StatusFailed
	return events, terminal, true
}

func (m *Manager) snapshot(id string) JobSnapshot {
	snapshot, _ := m.Snapshot(id)
	return snapshot
}

func (m *Manager) append(id, eventType string, extra map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[id]
	if !ok {
		return nil
	}

	seq := int64(len(job.Events) + 1)
	payload := map[string]any{
		"type":   eventType,
		"job_id": id,
		"seq":    seq,
		"ts":     time.Now().UTC().Format(time.RFC3339),
	}
	for key, value := range extra {
		payload[key] = value
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	job.Events = append(job.Events, StoredEvent{Seq: seq, Type: eventType, Data: data})
	return nil
}

func (m *Manager) setStatus(id, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if job, ok := m.jobs[id]; ok {
		job.Status = status
	}
}

func (m *Manager) complete(id string, insight Insight) {
	_ = m.append(id, "result", map[string]any{"insight": insight})

	m.mu.Lock()
	job, ok := m.jobs[id]
	if !ok {
		m.mu.Unlock()
		return
	}
	job.Insight = &insight
	job.Status = StatusDone
	m.mu.Unlock()

	m.logger.Info("analysis complete", "job_id", id)
}

func (m *Manager) fail(id, code, message string) {
	m.mu.Lock()
	job, ok := m.jobs[id]
	if !ok || job.Status == StatusDone || job.Status == StatusFailed {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	_ = m.append(id, "error", map[string]any{
		"code":      code,
		"message":   message,
		"retryable": false,
	})

	m.mu.Lock()
	if job, ok := m.jobs[id]; ok && job.Status != StatusDone {
		job.Status = StatusFailed
		job.Error = message
	}
	m.mu.Unlock()

	m.logger.Error("analysis failed", "job_id", id, "code", code, "message", message)
}

func snapshotOf(job *Job) JobSnapshot {
	return JobSnapshot{
		ID:        job.ID,
		CompanyID: job.CompanyID,
		EventID:   job.EventID,
		Status:    job.Status,
		Insight:   job.Insight,
		Error:     job.Error,
		CreatedAt: job.CreatedAt,
	}
}

func newID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().String()))
	}
	return hex.EncodeToString(buf)
}
