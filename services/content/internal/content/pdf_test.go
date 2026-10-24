package content

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func newPDFRouter(repo store, files Storage) *gin.Engine {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(repo, files, discardLogger())
	router := gin.New()
	router.POST("/api/v1/documents/pdf", handler.UploadPDF)
	return router
}

func pdfRequest(t *testing.T, fieldName string, content []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(fieldName, "report.pdf")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/documents/pdf", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func TestUploadPDFRequiresStorage(t *testing.T) {
	router := newPDFRouter(&fakeStore{}, nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, pdfRequest(t, "file", []byte("%PDF-1.4")))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

func TestUploadPDFRequiresFile(t *testing.T) {
	router := newPDFRouter(&fakeStore{}, NewLocalStorage(t.TempDir()))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/documents/pdf", strings.NewReader(""))
	request.Header.Set("Content-Type", "multipart/form-data; boundary=x")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestUploadPDFRejectsNonPDF(t *testing.T) {
	router := newPDFRouter(&fakeStore{}, NewLocalStorage(t.TempDir()))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, pdfRequest(t, "file", []byte("just text")))

	if recorder.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnsupportedMediaType)
	}
}

func TestUploadPDFRejectsBrokenPDF(t *testing.T) {
	router := newPDFRouter(&fakeStore{}, NewLocalStorage(t.TempDir()))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, pdfRequest(t, "file", []byte("%PDF-1.4 broken")))

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d body=%s", recorder.Code, http.StatusUnprocessableEntity, recorder.Body.String())
	}

	var payload struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Code != "pdf_unavailable" && payload.Code != "pdf_unreadable" && payload.Code != "pdf_empty" {
		t.Fatalf("code = %q", payload.Code)
	}
}

func TestExtractPDFTextRejectsGarbage(t *testing.T) {
	if _, err := ExtractPDFText(context.Background(), []byte("not a pdf")); err == nil {
		t.Fatal("ExtractPDFText succeeded on garbage")
	}
}

func TestDeriveTitle(t *testing.T) {
	text := "\n\n  Банк России сохранил ставку  \nВторой абзац"
	if got := deriveTitle(text, "file.pdf"); got != "Банк России сохранил ставку" {
		t.Fatalf("deriveTitle = %q", got)
	}
	if got := deriveTitle("", "Отчёт за квартал.pdf"); got != "Отчёт за квартал" {
		t.Fatalf("deriveTitle fallback = %q", got)
	}
	if got := deriveTitle("", ""); got != "PDF документ" {
		t.Fatalf("deriveTitle empty = %q", got)
	}
}

func TestSplitList(t *testing.T) {
	items := splitList(" ставка, банки ,, ")
	if len(items) != 2 || items[0] != "ставка" || items[1] != "банки" {
		t.Fatalf("splitList = %v", items)
	}
	if splitList("  ") != nil {
		t.Fatal("splitList should return nil for blank input")
	}
}

func TestParseFormTime(t *testing.T) {
	parsed, err := parseFormTime("2026-09-11")
	if err != nil || parsed.Format("2006-01-02") != "2026-09-11" {
		t.Fatalf("parseFormTime date = %v, %v", parsed, err)
	}
	parsed, err = parseFormTime("2026-09-11T13:30:00Z")
	if err != nil || parsed.IsZero() {
		t.Fatalf("parseFormTime rfc3339 = %v, %v", parsed, err)
	}
	if _, err := parseFormTime("not-a-date"); err == nil {
		t.Fatal("parseFormTime accepted an invalid date")
	}
	if parsed, err := parseFormTime(""); err != nil || !parsed.IsZero() {
		t.Fatalf("parseFormTime empty = %v, %v", parsed, err)
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("абвгда", 4); got != "абвг" {
		t.Fatalf("truncateRunes = %q", got)
	}
	if got := truncateRunes("abc", 10); got != "abc" {
		t.Fatalf("truncateRunes short = %q", got)
	}
}
