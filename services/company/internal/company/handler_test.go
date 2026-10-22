package company

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakeStore struct {
	items   []Company
	listErr error
	item    Company
	getErr  error
	gotID   string
}

func (f *fakeStore) List(context.Context) ([]Company, error) {
	return f.items, f.listErr
}

func (f *fakeStore) Get(_ context.Context, id string) (Company, error) {
	f.gotID = id
	return f.item, f.getErr
}

func newTestRouter(store Store) *gin.Engine {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	router := gin.New()
	router.GET("/api/v1/companies", handler.List)
	router.GET("/api/v1/companies/:id", handler.Get)
	return router
}

func performRequest(t *testing.T, router *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestListCompanies(t *testing.T) {
	store := &fakeStore{items: []Company{
		{ID: "magnit", Name: "Магнит"},
		{ID: "yandex", Name: "Яндекс"},
	}}

	recorder := performRequest(t, newTestRouter(store), "/api/v1/companies")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var body struct {
		Items []Company `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Items) != 2 || body.Items[0].ID != "magnit" || body.Items[1].ID != "yandex" {
		t.Fatalf("unexpected items: %+v", body.Items)
	}
}

func TestListCompaniesEmpty(t *testing.T) {
	recorder := performRequest(t, newTestRouter(&fakeStore{items: []Company{}}), "/api/v1/companies")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if body := recorder.Body.String(); body != `{"items":[]}` {
		t.Fatalf("body = %s, want empty items array", body)
	}
}

func TestListCompaniesError(t *testing.T) {
	store := &fakeStore{listErr: errors.New("database unavailable")}

	recorder := performRequest(t, newTestRouter(store), "/api/v1/companies")

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "companies_unavailable" {
		t.Fatalf("code = %q, want companies_unavailable", body.Code)
	}
}

func TestGetCompany(t *testing.T) {
	store := &fakeStore{item: Company{ID: "severstal", Name: "Северсталь", Industry: "Металлургия"}}

	recorder := performRequest(t, newTestRouter(store), "/api/v1/companies/severstal")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if store.gotID != "severstal" {
		t.Fatalf("repository got id %q, want severstal", store.gotID)
	}
	var body Company
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Name != "Северсталь" || body.Industry != "Металлургия" {
		t.Fatalf("unexpected company: %+v", body)
	}
}

func TestGetCompanyNotFound(t *testing.T) {
	store := &fakeStore{getErr: ErrNotFound}

	recorder := performRequest(t, newTestRouter(store), "/api/v1/companies/missing")

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "company_not_found" {
		t.Fatalf("code = %q, want company_not_found", body.Code)
	}
}

func TestGetCompanyError(t *testing.T) {
	store := &fakeStore{getErr: errors.New("database unavailable")}

	recorder := performRequest(t, newTestRouter(store), "/api/v1/companies/severstal")

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "company_unavailable" {
		t.Fatalf("code = %q, want company_unavailable", body.Code)
	}
}
