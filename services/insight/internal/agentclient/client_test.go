package agentclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnalyzeParsesSSE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/analyses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Accept"); got != "text/event-stream" {
			t.Fatalf("unexpected accept header: %s", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var payload AnalysisRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if payload.RequestID != "job-1" {
			t.Fatalf("unexpected request id: %s", payload.RequestID)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response writer does not support flushing")
		}
		_, _ = fmt.Fprint(w, "event: step\ndata: {\"type\":\"step\",\"name\":\"retrieving\",\"status\":\"running\"}\n\n")
		flusher.Flush()
		_, _ = fmt.Fprint(w, "event: result\ndata: {\"type\":\"result\",\"insight\":{\"summary\":\"ok\"}}\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client := New(server.URL)
	events := make([]Event, 0, 2)
	err := client.Analyze(context.Background(), AnalysisRequest{
		RequestID: "job-1",
		Company:   Company{ID: "magnit", Name: "Магнит"},
		Event:     EventContext{ID: "news-001", Title: "Событие"},
	}, func(event Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != "step" || events[1].Type != "result" {
		t.Fatalf("unexpected event types: %s, %s", events[0].Type, events[1].Type)
	}
}
