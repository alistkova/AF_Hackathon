package content

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

var errTestDatabase = errors.New("database unavailable")

func newDocumentRouter(repo store) *gin.Engine {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(repo, nil, slog.New(slog.DiscardHandler))
	router := gin.New()
	router.GET("/api/v1/news", handler.List)
	router.GET("/api/v1/news/:id", handler.Get)
	router.GET("/api/v1/search", handler.Search)
	router.POST("/api/v1/documents", handler.Ingest)
	router.GET("/api/v1/documents/:id/chunks", handler.Chunks)
	return router
}

func perform(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestListDocumentsPassesFilter(t *testing.T) {
	store := &fakeStore{documents: []NewsItem{{NewsID: "n1"}}, documentTotal: 7}

	response := perform(newDocumentRouter(store), http.MethodGet,
		"/api/v1/news?sector=Ритейл&tag=ставка&source=РБК&from=2026-09-01&limit=10&offset=5", "")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("X-Total-Count"); got != "7" {
		t.Fatalf("X-Total-Count = %q, want 7", got)
	}
	if store.gotFilter.Sector != "Ритейл" || store.gotFilter.Tag != "ставка" || store.gotFilter.Source != "РБК" {
		t.Fatalf("filter = %+v", store.gotFilter)
	}
	if store.gotFilter.Limit != 10 || store.gotFilter.Offset != 5 {
		t.Fatalf("limit/offset = %d/%d", store.gotFilter.Limit, store.gotFilter.Offset)
	}
	if store.gotFilter.From == nil || store.gotFilter.From.Format("2006-01-02") != "2026-09-01" {
		t.Fatalf("from = %v", store.gotFilter.From)
	}

	var items []NewsItem
	if err := json.Unmarshal(response.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(items) != 1 || items[0].NewsID != "n1" {
		t.Fatalf("items = %+v", items)
	}
}

func TestListDocumentsRejectsBadParams(t *testing.T) {
	cases := []string{
		"/api/v1/news?from=not-a-date",
		"/api/v1/news?limit=0",
		"/api/v1/news?limit=abc",
		"/api/v1/news?offset=-1",
	}

	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			response := perform(newDocumentRouter(&fakeStore{}), http.MethodGet, path, "")
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestListDocumentsError(t *testing.T) {
	store := &fakeStore{documentErr: errTestDatabase}
	response := perform(newDocumentRouter(store), http.MethodGet, "/api/v1/news", "")

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
}

func TestSearchRequiresQuery(t *testing.T) {
	response := perform(newDocumentRouter(&fakeStore{}), http.MethodGet, "/api/v1/search", "")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestSearchReturnsHits(t *testing.T) {
	store := &fakeStore{hits: []SearchHit{{NewsItem: NewsItem{NewsID: "n1"}, Rank: 0.5}}}

	response := perform(newDocumentRouter(store), http.MethodGet, "/api/v1/search?q=ставка&sector=Ритейл", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var body struct {
		Items []SearchHit `json:"items"`
		Total int         `json:"total"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 1 || body.Total != 1 {
		t.Fatalf("body = %+v", body)
	}
	if store.gotFilter.Sector != "Ритейл" {
		t.Fatalf("sector filter = %q", store.gotFilter.Sector)
	}
}

func TestGetDocumentNotFound(t *testing.T) {
	response := perform(newDocumentRouter(&fakeStore{}), http.MethodGet, "/api/v1/news/missing", "")
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestGetDocument(t *testing.T) {
	store := &fakeStore{
		documentFound: true,
		document:      NewsItem{NewsID: "n1", Title: "Ставка"},
	}

	response := perform(newDocumentRouter(store), http.MethodGet, "/api/v1/news/n1", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var item NewsItem
	if err := json.Unmarshal(response.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if item.Title != "Ставка" {
		t.Fatalf("item = %+v", item)
	}
}

func TestIngestRejectsEmptyItems(t *testing.T) {
	for _, body := range []string{`{}`, `{"items":[]}`} {
		response := perform(newDocumentRouter(&fakeStore{}), http.MethodPost, "/api/v1/documents", body)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("body %s: status = %d, want %d", body, response.Code, http.StatusBadRequest)
		}
	}
}

func TestIngestRejectsMissingFields(t *testing.T) {
	body := `{"items":[{"news_id":"n1","full_text":"текст"}]}`
	response := perform(newDocumentRouter(&fakeStore{}), http.MethodPost, "/api/v1/documents", body)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestIngestDocuments(t *testing.T) {
	store := &fakeStore{}
	body := `{"items":[{"news_id":"n1","title":"Ставка","full_text":"Банк России сохранил ставку","source_url":"https://rbc.ru/a","tags":["ставка"]}]}`

	response := perform(newDocumentRouter(store), http.MethodPost, "/api/v1/documents", body)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", response.Code, http.StatusOK, response.Body.String())
	}

	var payload struct {
		Items []IngestResult `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].Status != IngestCreated {
		t.Fatalf("items = %+v", payload.Items)
	}
	if len(store.ingested) != 1 || store.ingested[0].NewsID != "n1" {
		t.Fatalf("ingested = %+v", store.ingested)
	}
}

func TestListDocumentChunks(t *testing.T) {
	store := &fakeStore{chunks: []DocumentChunk{{Ordinal: 0, Content: "первый"}, {Ordinal: 1, Content: "второй"}}}

	response := perform(newDocumentRouter(store), http.MethodGet, "/api/v1/documents/n1/chunks", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var payload struct {
		Items []DocumentChunk `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Items) != 2 {
		t.Fatalf("items = %+v", payload.Items)
	}
}
