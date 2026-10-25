package insight

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const contentCacheTTL = 30 * time.Second

type NewsItem struct {
	DocumentID    string    `json:"document_id"`
	NewsID        string    `json:"news_id"`
	Title         string    `json:"title"`
	PublishedAt   time.Time `json:"published_at"`
	SourceName    string    `json:"source_name"`
	SourceURL     string    `json:"source_url"`
	FullText      string    `json:"full_text"`
	Tags          []string  `json:"tags"`
	TargetSectors []string  `json:"target_sectors"`
}

type ContentClient struct {
	baseURL string
	client  *http.Client

	mu         sync.Mutex
	ttl        time.Duration
	cachedAt   time.Time
	cacheLimit int
	cache      []NewsItem
	hasCache   bool

	fetchMu sync.Mutex
}

func NewContentClient(baseURL string) *ContentClient {
	return &ContentClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 5 * time.Second},
		ttl:     contentCacheTTL,
	}
}

func (c *ContentClient) ListNews(ctx context.Context, limit int) ([]NewsItem, error) {
	if limit <= 0 {
		limit = 200
	}

	if items, ok := c.fresh(limit); ok {
		return items, nil
	}

	c.fetchMu.Lock()
	defer c.fetchMu.Unlock()

	if items, ok := c.fresh(limit); ok {
		return items, nil
	}

	items, err := c.fetch(ctx, limit)
	if err != nil {
		if stale, ok := c.stale(limit); ok {
			return stale, nil
		}
		return nil, err
	}

	c.store(items, limit)
	return items, nil
}

func (c *ContentClient) fresh(limit int) ([]NewsItem, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.hasCache || c.ttl <= 0 || limit > c.cacheLimit || time.Since(c.cachedAt) >= c.ttl {
		return nil, false
	}
	return truncateNews(c.cache, limit), true
}

func (c *ContentClient) stale(limit int) ([]NewsItem, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.hasCache {
		return nil, false
	}
	return truncateNews(c.cache, limit), true
}

func (c *ContentClient) store(items []NewsItem, limit int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = cloneNews(items)
	c.cacheLimit = limit
	c.cachedAt = time.Now()
	c.hasCache = true
}

func (c *ContentClient) fetch(ctx context.Context, limit int) ([]NewsItem, error) {
	query := url.Values{}
	query.Set("kind", "news")
	query.Set("limit", strconv.Itoa(limit))

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/news?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")

	response, err := c.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call content service: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("content service returned %d", response.StatusCode)
	}

	var items []NewsItem
	if err := json.NewDecoder(response.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("decode content news: %w", err)
	}
	return items, nil
}

func truncateNews(items []NewsItem, limit int) []NewsItem {
	cloned := cloneNews(items)
	if len(cloned) > limit {
		cloned = cloned[:limit]
	}
	return cloned
}

func cloneNews(items []NewsItem) []NewsItem {
	cloned := make([]NewsItem, len(items))
	copy(cloned, items)
	return cloned
}
