package agentclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Company struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	INN         string `json:"inn,omitempty"`
	Industry    string `json:"industry,omitempty"`
	Region      string `json:"region,omitempty"`
	Description string `json:"description,omitempty"`
}

type EventContext struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Summary     string `json:"summary,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
}

type Evidence struct {
	SourceID    string `json:"source_id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Snippet     string `json:"snippet,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
}

type AnalysisRequest struct {
	RequestID string       `json:"request_id"`
	Company   Company      `json:"company"`
	Event     EventContext `json:"event"`
	Evidence  []Evidence   `json:"evidence"`
	Locale    string       `json:"locale"`
	Mode      string       `json:"mode"`
}

type Event struct {
	Type string
	Data json.RawMessage
}

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{},
	}
}

func (c *Client) Analyze(ctx context.Context, payload AnalysisRequest, emit func(Event) error) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal analysis request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/analyses", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create analysis request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("call agent: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		return fmt.Errorf("agent returned %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64<<10), 2<<20)

	var eventType string
	dataLines := make([]string, 0, 1)
	flush := func() error {
		if eventType == "" && len(dataLines) == 0 {
			return nil
		}
		event := Event{Type: eventType, Data: json.RawMessage(strings.Join(dataLines, "\n"))}
		if event.Type == "" {
			event.Type = "message"
		}
		eventType = ""
		dataLines = dataLines[:0]
		if len(event.Data) == 0 || !json.Valid(event.Data) {
			return fmt.Errorf("invalid SSE data for %q", event.Type)
		}
		return emit(event)
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "event":
			eventType = value
		case "data":
			dataLines = append(dataLines, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read agent stream: %w", err)
	}
	return flush()
}
