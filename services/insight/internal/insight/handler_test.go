package insight

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type fakeCompanyProvider struct {
	item Company
	err  error
}

func (f fakeCompanyProvider) Get(context.Context, string) (Company, error) {
	if f.err != nil {
		return Company{}, f.err
	}
	return f.item, nil
}

type fakeNewsProvider struct {
	items []NewsItem
	err   error
}

func (f fakeNewsProvider) ListNews(context.Context, int) ([]NewsItem, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.items, nil
}

func TestFeedReturnsRankedCompanyNews(t *testing.T) {
	now := time.Now().UTC()
	router := feedRouter(
		staticCompany(Company{ID: "magnit", Name: "Магнит", Industry: "Ритейл", Region: "Краснодар"}),
		staticNews([]NewsItem{
			{
				NewsID:        "news-macro",
				Title:         "Банк России сохранил ключевую ставку",
				FullText:      "Регулятор сохранил ставку на прежнем уровне.",
				PublishedAt:   now.Add(-24 * time.Hour),
				SourceName:    "РБК",
				SourceURL:     "https://example.com/macro",
				TargetSectors: []string{"Финансы"},
			},
			{
				NewsID:        "news-magnit",
				Title:         "Магнит расширяет логистику",
				FullText:      "Ритейлер инвестирует в распределительные центры.",
				PublishedAt:   now.Add(-48 * time.Hour),
				SourceName:    "Ведомости",
				SourceURL:     "https://example.com/magnit",
				TargetSectors: []string{"Торговля"},
			},
		}),
	)

	response := performFeedRequest(t, router, "/api/v1/feed?company_id=magnit")
	if response.Code != http.StatusOK {
		t.Fatalf("status is %d, want 200: %s", response.Code, response.Body.String())
	}

	var payload struct {
		Items []FeedItem `json:"items"`
		Total int        `json:"total"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Total != 2 || len(payload.Items) != 2 {
		t.Fatalf("got %d items (total %d), want 2", len(payload.Items), payload.Total)
	}

	item := payload.Items[0]
	if item.EventID != "news-magnit" || item.CompanyID != "magnit" {
		t.Fatalf("unexpected item: %+v", item)
	}
	if item.Source != "Ведомости" || item.URL != "https://example.com/magnit" {
		t.Fatalf("source fields are not mapped: %+v", item)
	}
	if len(item.Reasons) == 0 || item.Relevance <= 0 {
		t.Fatalf("item has no ranking metadata: %+v", item)
	}
	if general := payload.Items[1]; general.EventID != "news-macro" || general.Relevance != genericFeedRelevance {
		t.Fatalf("general news is not ranked last: %+v", general)
	}
}

func TestFeedPaginates(t *testing.T) {
	now := time.Now().UTC()
	news := make([]NewsItem, 0, 3)
	for index := 0; index < 3; index++ {
		news = append(news, NewsItem{
			NewsID:        []string{"news-a", "news-b", "news-c"}[index],
			Title:         "Отраслевая новость",
			FullText:      "Компании обсуждают развитие.",
			PublishedAt:   now.Add(-time.Duration(index+1) * 24 * time.Hour),
			TargetSectors: []string{"Торговля"},
		})
	}

	router := feedRouter(
		staticCompany(Company{ID: "magnit", Name: "Магнит", Industry: "Ритейл"}),
		staticNews(news),
	)
	response := performFeedRequest(t, router, "/api/v1/feed?company_id=magnit&limit=1&offset=1")
	if response.Code != http.StatusOK {
		t.Fatalf("status is %d, want 200: %s", response.Code, response.Body.String())
	}

	var payload struct {
		Items []FeedItem `json:"items"`
		Total int        `json:"total"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Total != 3 {
		t.Fatalf("total is %d, want 3", payload.Total)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(payload.Items))
	}
	if payload.Items[0].EventID != "news-b" {
		t.Fatalf("first paginated item is %q, want news-b", payload.Items[0].EventID)
	}
}

func TestFeedFiltersAndReturnsUnfilteredFacets(t *testing.T) {
	now := time.Now().UTC()
	router := feedRouter(
		staticCompany(Company{ID: "magnit", Name: "Магнит", Industry: "Ритейл"}),
		staticNews([]NewsItem{
			{
				NewsID:        "news-vedomosti",
				Title:         "Магнит расширяет логистику",
				FullText:      "Ритейлер инвестирует в распределительные центры.",
				PublishedAt:   now.Add(-2 * 24 * time.Hour),
				SourceName:    "Ведомости",
				SourceURL:     "https://example.com/vedomosti",
				Tags:          []string{"ритейл", "Магнит"},
				TargetSectors: []string{"Торговля"},
			},
			{
				NewsID:        "news-rbc",
				Title:         "Ритейлеры наращивают инвестиции",
				FullText:      "Сеть открывает новые распределительные центры.",
				PublishedAt:   now.Add(-3 * 24 * time.Hour),
				SourceName:    "РБК",
				SourceURL:     "https://example.com/rbc",
				Tags:          []string{"ритейл"},
				TargetSectors: []string{"Торговля"},
			},
		}),
	)

	query := url.Values{"company_id": {"magnit"}, "source": {"Ведомости"}}
	response := performFeedRequest(t, router, "/api/v1/feed?"+query.Encode())
	if response.Code != http.StatusOK {
		t.Fatalf("status is %d, want 200: %s", response.Code, response.Body.String())
	}

	var payload struct {
		Items  []FeedItem `json:"items"`
		Total  int        `json:"total"`
		Facets FeedFacets `json:"facets"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.Total != 1 || len(payload.Items) != 1 {
		t.Fatalf("got %d items (total %d), want 1", len(payload.Items), payload.Total)
	}
	if payload.Items[0].Source != "Ведомости" {
		t.Fatalf("item source is %q, want Ведомости", payload.Items[0].Source)
	}
	if len(payload.Items[0].Tags) == 0 || len(payload.Items[0].TargetSectors) == 0 {
		t.Fatalf("item does not expose filter metadata: %+v", payload.Items[0])
	}
	if len(payload.Facets.Sources) != 2 {
		t.Fatalf("got %d source facets, want 2", len(payload.Facets.Sources))
	}
	if len(payload.Facets.Tags) != 2 {
		t.Fatalf("got %d tag facets, want 2", len(payload.Facets.Tags))
	}
}

func TestFeedSortsByDate(t *testing.T) {
	now := time.Now().UTC()
	router := feedRouter(
		staticCompany(Company{ID: "magnit", Name: "Магнит", Industry: "Ритейл"}),
		staticNews([]NewsItem{
			{
				NewsID:        "news-old",
				Title:         "Магнит расширяет логистику",
				FullText:      "Ритейлер инвестирует в центры.",
				PublishedAt:   now.Add(-10 * 24 * time.Hour),
				SourceName:    "Ведомости",
				TargetSectors: []string{"Торговля"},
			},
			{
				NewsID:        "news-new",
				Title:         "Ритейлеры открывают новые магазины",
				FullText:      "Сеть расширяет присутствие.",
				PublishedAt:   now.Add(-24 * time.Hour),
				SourceName:    "РБК",
				TargetSectors: []string{"Торговля"},
			},
		}),
	)

	response := performFeedRequest(t, router, "/api/v1/feed?company_id=magnit&sort=date")
	if response.Code != http.StatusOK {
		t.Fatalf("status is %d, want 200: %s", response.Code, response.Body.String())
	}

	var payload struct {
		Items []FeedItem `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Items) != 2 {
		t.Fatalf("got %d items, want 2", len(payload.Items))
	}
	if payload.Items[0].EventID != "news-new" {
		t.Fatalf("first item is %q, want news-new", payload.Items[0].EventID)
	}
}

func TestFeedErrors(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		company    companyProvider
		news       newsProvider
		wantStatus int
		wantCode   string
	}{
		{
			name:       "missing company",
			query:      "/api/v1/feed",
			company:    staticCompany(Company{ID: "magnit"}),
			news:       staticNews(nil),
			wantStatus: http.StatusBadRequest,
			wantCode:   "company_required",
		},
		{
			name:       "invalid limit",
			query:      "/api/v1/feed?company_id=magnit&limit=0",
			company:    staticCompany(Company{ID: "magnit"}),
			news:       staticNews(nil),
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "invalid sort",
			query:      "/api/v1/feed?company_id=magnit&sort=popular",
			company:    staticCompany(Company{ID: "magnit"}),
			news:       staticNews(nil),
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "invalid date",
			query:      "/api/v1/feed?company_id=magnit&from=not-a-date",
			company:    staticCompany(Company{ID: "magnit"}),
			news:       staticNews(nil),
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "reversed period",
			query:      "/api/v1/feed?company_id=magnit&from=2026-09-10&to=2026-09-01",
			company:    staticCompany(Company{ID: "magnit"}),
			news:       staticNews(nil),
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "unknown company",
			query:      "/api/v1/feed?company_id=unknown",
			company:    failingCompany(ErrCompanyNotFound),
			news:       staticNews(nil),
			wantStatus: http.StatusNotFound,
			wantCode:   "company_not_found",
		},
		{
			name:       "company unavailable",
			query:      "/api/v1/feed?company_id=magnit",
			company:    failingCompany(errors.New("boom")),
			news:       staticNews(nil),
			wantStatus: http.StatusBadGateway,
			wantCode:   "company_unavailable",
		},
		{
			name:       "content unavailable",
			query:      "/api/v1/feed?company_id=magnit",
			company:    staticCompany(Company{ID: "magnit", Industry: "Ритейл"}),
			news:       failingNews(errors.New("boom")),
			wantStatus: http.StatusBadGateway,
			wantCode:   "content_unavailable",
		},
		{
			name:       "no news",
			query:      "/api/v1/feed?company_id=magnit",
			company:    staticCompany(Company{ID: "magnit", Industry: "Ритейл"}),
			news:       staticNews(nil),
			wantStatus: http.StatusOK,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			router := feedRouter(testCase.company, testCase.news)
			response := performFeedRequest(t, router, testCase.query)
			if response.Code != testCase.wantStatus {
				t.Fatalf("status is %d, want %d: %s", response.Code, testCase.wantStatus, response.Body.String())
			}
			if testCase.wantCode == "" {
				return
			}

			var payload struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if payload.Code != testCase.wantCode {
				t.Fatalf("code is %q, want %q", payload.Code, testCase.wantCode)
			}
		})
	}
}

func TestCreateJobReturnsNotFoundForUnknownCompany(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(nil, failingCompany(ErrCompanyNotFound), staticNews(nil), logger)
	router := gin.New()
	router.POST("/api/v1/insights/jobs", handler.CreateJob)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/insights/jobs", strings.NewReader(`{"company_id":"unknown","event":{"title":"Событие"}}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status is %d, want 404: %s", response.Code, response.Body.String())
	}

	var payload struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if payload.Code != "company_not_found" {
		t.Fatalf("code is %q, want company_not_found", payload.Code)
	}
}

func feedRouter(company companyProvider, news newsProvider) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(nil, company, news, logger)
	router := gin.New()
	router.GET("/api/v1/feed", handler.Feed)
	return router
}

func performFeedRequest(t *testing.T, router *gin.Engine, target string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func staticCompany(company Company) companyProvider {
	return fakeCompanyProvider{item: company}
}

func failingCompany(err error) companyProvider {
	return fakeCompanyProvider{err: err}
}

func staticNews(items []NewsItem) newsProvider {
	return fakeNewsProvider{items: items}
}

func failingNews(err error) newsProvider {
	return fakeNewsProvider{err: err}
}
