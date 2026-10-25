package insight

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestContentClientListsNewsWithKindFilter(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/news" {
			t.Fatalf("path is %q, want /api/v1/news", r.URL.Path)
		}
		if got := r.URL.Query().Get("kind"); got != "news" {
			t.Fatalf("kind is %q, want news", got)
		}
		if got := r.URL.Query().Get("limit"); got != "25" {
			t.Fatalf("limit is %q, want 25", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"document_id":    "doc-1",
			"news_id":        "news-1",
			"title":          "Магнит расширяет логистику",
			"published_at":   "2026-10-20T10:00:00Z",
			"source_name":    "Ведомости",
			"source_url":     "https://example.com/news-1",
			"full_text":      "Ритейлер открывает новые центры.",
			"tags":           []string{"ритейл"},
			"target_sectors": []string{"Торговля"},
		}})
	}))
	defer server.Close()

	client := NewContentClient(server.URL)
	items, err := client.ListNews(context.Background(), 25)
	if err != nil {
		t.Fatalf("list news: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].NewsID != "news-1" || items[0].SourceName != "Ведомости" {
		t.Fatalf("unexpected item: %+v", items[0])
	}
	if !items[0].PublishedAt.Equal(time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected published_at: %v", items[0].PublishedAt)
	}
	if len(items[0].TargetSectors) != 1 || items[0].TargetSectors[0] != "Торговля" {
		t.Fatalf("unexpected target sectors: %v", items[0].TargetSectors)
	}
}

func TestContentClientCachesNews(t *testing.T) {
	t.Parallel()

	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"news_id":      "news-1",
			"title":        "Кэшированная новость",
			"published_at": "2026-10-20T10:00:00Z",
		}})
	}))
	defer server.Close()

	client := NewContentClient(server.URL)
	client.ttl = time.Minute

	first, err := client.ListNews(context.Background(), 10)
	if err != nil {
		t.Fatalf("first list news: %v", err)
	}
	second, err := client.ListNews(context.Background(), 10)
	if err != nil {
		t.Fatalf("second list news: %v", err)
	}

	if calls != 1 {
		t.Fatalf("content service calls = %d, want 1", calls)
	}
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("unexpected cached items: first=%d second=%d", len(first), len(second))
	}
}

func TestContentClientRejectsNonOKResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewContentClient(server.URL)
	if _, err := client.ListNews(context.Background(), 10); err == nil {
		t.Fatal("expected an error for a 500 response")
	}
}

func TestContentClientServesStaleCacheWhenContentFails(t *testing.T) {
	t.Parallel()

	var healthy atomic.Bool
	healthy.Store(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if !healthy.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"news_id":      "news-1",
			"title":        "Кэшированная новость",
			"published_at": "2026-10-20T10:00:00Z",
		}})
	}))
	defer server.Close()

	client := NewContentClient(server.URL)
	client.ttl = time.Minute
	if _, err := client.ListNews(context.Background(), 10); err != nil {
		t.Fatalf("first list news: %v", err)
	}

	client.mu.Lock()
	client.cachedAt = time.Now().Add(-2 * time.Minute)
	client.mu.Unlock()
	healthy.Store(false)

	items, err := client.ListNews(context.Background(), 10)
	if err != nil {
		t.Fatalf("stale list news: %v", err)
	}
	if len(items) != 1 || items[0].NewsID != "news-1" {
		t.Fatalf("unexpected stale items: %+v", items)
	}
}

func TestContentClientFetchesOnceForConcurrentCallers(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		time.Sleep(20 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"news_id": "news-1",
			"title":   "Новость",
		}})
	}))
	defer server.Close()

	client := NewContentClient(server.URL)
	client.ttl = time.Minute

	var wait sync.WaitGroup
	for index := 0; index < 5; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if _, err := client.ListNews(context.Background(), 10); err != nil {
				t.Errorf("list news: %v", err)
			}
		}()
	}
	wait.Wait()

	if got := calls.Load(); got != 1 {
		t.Fatalf("content service calls = %d, want 1", got)
	}
}
