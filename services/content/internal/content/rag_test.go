package content

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakeStore struct {
	searchQuery   RAGSearchQuery
	searchChunks  []RAGChunk
	upsertChunks  []RAGChunkInput
	count         int64
	documents     []NewsItem
	documentTotal int
	documentErr   error
	gotFilter     DocumentFilter
	document      NewsItem
	documentFound bool
	hits          []SearchHit
	ingested      []IngestInput
	chunks        []DocumentChunk
}

func (f *fakeStore) ListDocuments(_ context.Context, filter DocumentFilter) ([]NewsItem, int, error) {
	f.gotFilter = filter
	return f.documents, f.documentTotal, f.documentErr
}

func (f *fakeStore) GetDocument(_ context.Context, id string) (NewsItem, error) {
	if !f.documentFound {
		return NewsItem{}, ErrNotFound
	}
	item := f.document
	item.NewsID = id
	return item, nil
}

func (f *fakeStore) SearchDocuments(_ context.Context, _ string, filter DocumentFilter) ([]SearchHit, int, error) {
	f.gotFilter = filter
	return f.hits, len(f.hits), f.documentErr
}

func (f *fakeStore) IngestDocument(_ context.Context, input IngestInput) (IngestResult, error) {
	f.ingested = append(f.ingested, input)
	return IngestResult{Status: IngestCreated, DocumentID: input.DocumentID, NewsID: input.NewsID, Chunks: 1}, nil
}

func (f *fakeStore) ListDocumentChunks(context.Context, string) ([]DocumentChunk, error) {
	return f.chunks, nil
}

func (f *fakeStore) UpsertRAGChunks(_ context.Context, chunks []RAGChunkInput) (int, error) {
	f.upsertChunks = chunks
	return len(chunks), nil
}

func (f *fakeStore) SearchRAGChunks(_ context.Context, query RAGSearchQuery) ([]RAGChunk, error) {
	f.searchQuery = query
	return f.searchChunks, nil
}

func (f *fakeStore) ListRAGChunks(_ context.Context, query RAGListQuery) ([]RAGChunk, error) {
	return []RAGChunk{{Content: "text", CompanyID: query.CompanyID}}, nil
}

func (f *fakeStore) CountRAGChunks(context.Context) (int64, error) {
	return f.count, nil
}

func (f *fakeStore) DeleteRAGChunks(context.Context) (int64, error) {
	return f.count, nil
}

func newTestRouter(repo store) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(repo, nil, logger)
	router := gin.New()
	router.POST("/api/v1/rag/chunks", handler.UpsertRAGChunks)
	router.POST("/api/v1/rag/chunks/search", handler.SearchRAGChunks)
	router.POST("/api/v1/rag/chunks/list", handler.ListRAGChunks)
	router.GET("/api/v1/rag/chunks/count", handler.CountRAGChunks)
	router.DELETE("/api/v1/rag/chunks", handler.DeleteRAGChunks)
	return router
}

func performRequest(router *gin.Engine, method, path string, payload any) *httptest.ResponseRecorder {
	var body io.Reader
	if payload != nil {
		raw, _ := json.Marshal(payload)
		body = bytes.NewReader(raw)
	}
	request := httptest.NewRequest(method, path, body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func validEmbedding() []float32 {
	values := make([]float32, RAGEmbeddingDimensions)
	values[0] = 0.5
	return values
}

func TestRAGFilterClauses(t *testing.T) {
	clauses, args := ragFilterClauses(RAGFilter{
		CompanyID:       "magnit",
		Sector:          "retail",
		PublishedAfter:  "2026-01-01",
		PublishedBefore: "2026-12-31",
	}, 2)

	expected := []string{
		"company_id = $2",
		"sector = $3",
		"published_at >= $4::text::date",
		"published_at <= $5::text::date",
	}
	if len(clauses) != len(expected) {
		t.Fatalf("expected %d clauses, got %d", len(expected), len(clauses))
	}
	for i := range expected {
		if clauses[i] != expected[i] {
			t.Fatalf("clause %d: expected %q, got %q", i, expected[i], clauses[i])
		}
	}
	if len(args) != 4 || args[0] != "magnit" || args[3] != "2026-12-31" {
		t.Fatalf("unexpected args: %v", args)
	}
}

func TestRAGFilterClausesEmpty(t *testing.T) {
	clauses, args := ragFilterClauses(RAGFilter{}, 1)
	if len(clauses) != 0 || len(args) != 0 {
		t.Fatalf("expected empty filter, got %v %v", clauses, args)
	}
	if where := ragWhere(clauses); where != "" {
		t.Fatalf("expected empty where, got %q", where)
	}
}

func TestRAGWhereJoinsClauses(t *testing.T) {
	where := ragWhere([]string{"company_id = $1", "sector = $2"})
	if where != " WHERE company_id = $1 AND sector = $2" {
		t.Fatalf("unexpected where: %q", where)
	}
}

func TestFormatVector(t *testing.T) {
	if got := formatVector([]float32{1, 0.5, -2.25}); got != "[1,0.5,-2.25]" {
		t.Fatalf("unexpected vector: %s", got)
	}
}

func TestNormalizeTopK(t *testing.T) {
	if got := normalizeTopK(0, 10); got != 10 {
		t.Fatalf("expected fallback, got %d", got)
	}
	if got := normalizeTopK(500, 10); got != ragMaxTopK {
		t.Fatalf("expected clamp, got %d", got)
	}
	if got := normalizeTopK(7, 10); got != 7 {
		t.Fatalf("expected 7, got %d", got)
	}
}

func TestValidateEmbedding(t *testing.T) {
	if err := validateEmbedding(validEmbedding()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := validateEmbedding([]float32{1}); err == nil {
		t.Fatal("expected dimension error")
	}
	values := validEmbedding()
	values[1] = float32(math.NaN())
	if err := validateEmbedding(values); err == nil {
		t.Fatal("expected NaN error")
	}
}

func TestRAGValidationErrorMapping(t *testing.T) {
	cases := []struct {
		err  error
		code string
	}{
		{errChunkContentRequired, "invalid_chunk"},
		{errChunkSourceRequired, "invalid_chunk"},
		{errChunkOrdinalNegative, "invalid_chunk"},
		{errChunkPublishedFormat, "invalid_chunk"},
		{errFilterDateFormat, "invalid_filter"},
		{errEmbeddingDimensions, "invalid_embedding"},
		{errEmbeddingValue, "invalid_embedding"},
	}
	for _, tc := range cases {
		code, message := ragValidationError(tc.err)
		if code != tc.code {
			t.Fatalf("expected code %q, got %q", tc.code, code)
		}
		if message == "" {
			t.Fatalf("expected a message for %v", tc.err)
		}
	}
}

func TestSearchRAGChunksHandlerPassesFilters(t *testing.T) {
	repo := &fakeStore{searchChunks: []RAGChunk{{Content: "chunk", CompanyID: "magnit"}}}
	router := newTestRouter(repo)

	recorder := performRequest(router, http.MethodPost, "/api/v1/rag/chunks/search", map[string]any{
		"embedding":        validEmbedding(),
		"top_k":            500,
		"company_id":       "magnit",
		"published_after":  "2026-01-01",
		"published_before": "2026-12-31",
		"max_distance":     0.8,
	})

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if repo.searchQuery.CompanyID != "magnit" {
		t.Fatalf("unexpected company filter: %+v", repo.searchQuery)
	}
	if repo.searchQuery.TopK != ragMaxTopK {
		t.Fatalf("expected top_k clamp, got %d", repo.searchQuery.TopK)
	}
	if repo.searchQuery.MaxDistance == nil || *repo.searchQuery.MaxDistance != 0.8 {
		t.Fatalf("unexpected max distance: %+v", repo.searchQuery)
	}
}

func TestSearchRAGChunksHandlerRejectsBadEmbedding(t *testing.T) {
	repo := &fakeStore{}
	router := newTestRouter(repo)

	recorder := performRequest(router, http.MethodPost, "/api/v1/rag/chunks/search", map[string]any{
		"embedding": []float32{1, 2, 3},
	})

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	if repo.searchQuery.CompanyID != "" {
		t.Fatal("repo must not be called")
	}
}

func TestSearchRAGChunksHandlerRejectsBadDate(t *testing.T) {
	router := newTestRouter(&fakeStore{})

	recorder := performRequest(router, http.MethodPost, "/api/v1/rag/chunks/search", map[string]any{
		"embedding":        validEmbedding(),
		"published_before": "31.12.2026",
	})

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestUpsertRAGChunksHandler(t *testing.T) {
	repo := &fakeStore{}
	router := newTestRouter(repo)

	recorder := performRequest(router, http.MethodPost, "/api/v1/rag/chunks", map[string]any{
		"chunks": []map[string]any{
			{
				"content":      "Магнит расширяет логистику",
				"source":       "news-1.pdf",
				"page":         1,
				"chunk":        0,
				"company_id":   "magnit",
				"sector":       "retail",
				"published_at": "2026-03-15",
				"embedding":    validEmbedding(),
			},
		},
	})

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if len(repo.upsertChunks) != 1 || repo.upsertChunks[0].PublishedAt == nil {
		t.Fatalf("unexpected upsert payload: %+v", repo.upsertChunks)
	}
	var response RAGUpsertResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Upserted != 1 {
		t.Fatalf("unexpected upserted count: %d", response.Upserted)
	}
}

func TestUpsertRAGChunksHandlerRejectsBadDate(t *testing.T) {
	router := newTestRouter(&fakeStore{})

	recorder := performRequest(router, http.MethodPost, "/api/v1/rag/chunks", map[string]any{
		"chunks": []map[string]any{
			{
				"content":      "chunk",
				"source":       "news-1.pdf",
				"published_at": "2026-13-40",
				"embedding":    validEmbedding(),
			},
		},
	})

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestCountAndDeleteRAGChunksHandlers(t *testing.T) {
	router := newTestRouter(&fakeStore{count: 7})

	countRecorder := performRequest(router, http.MethodGet, "/api/v1/rag/chunks/count", nil)
	if countRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", countRecorder.Code)
	}
	var count RAGCountResponse
	if err := json.Unmarshal(countRecorder.Body.Bytes(), &count); err != nil {
		t.Fatalf("decode count: %v", err)
	}
	if count.Count != 7 {
		t.Fatalf("unexpected count: %d", count.Count)
	}

	deleteRecorder := performRequest(router, http.MethodDelete, "/api/v1/rag/chunks", nil)
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", deleteRecorder.Code)
	}
	var deleted RAGDeleteResponse
	if err := json.Unmarshal(deleteRecorder.Body.Bytes(), &deleted); err != nil {
		t.Fatalf("decode delete: %v", err)
	}
	if deleted.Deleted != 7 {
		t.Fatalf("unexpected deleted count: %d", deleted.Deleted)
	}
}

func TestListRAGChunksHandlerUsesFilter(t *testing.T) {
	router := newTestRouter(&fakeStore{})

	recorder := performRequest(router, http.MethodPost, "/api/v1/rag/chunks/list", map[string]any{
		"company_id": "x5",
		"top_k":      3,
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response RAGChunksResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Chunks) != 1 || response.Chunks[0].CompanyID != "x5" {
		t.Fatalf("unexpected chunks: %+v", response.Chunks)
	}
}

func TestRAGChunkJSONShape(t *testing.T) {
	published := "2026-03-15"
	chunk := RAGChunk{Content: "text", Source: "news-1.pdf", PublishedAt: &published}

	body, err := json.Marshal(chunk)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["published_at"] != published {
		t.Fatalf("unexpected published_at: %v", payload["published_at"])
	}
	if _, ok := payload["distance"]; ok {
		t.Fatalf("distance must be omitted when empty: %s", body)
	}
}
