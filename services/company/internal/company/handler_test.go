package company

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakeStore struct {
	items      []Company
	listErr    error
	gotIDs     []string
	gotFilter  bool
	item       Company
	getErr     error
	gotID      string
	details    CompanyDetails
	detailsErr error
	people     []CompanyPerson
	peopleErr  error
	relations  []CompanyRelation
	relErr     error
}

func (f *fakeStore) List(_ context.Context, ids []string) ([]Company, error) {
	f.gotIDs = ids
	f.gotFilter = ids != nil
	return f.items, f.listErr
}

func (f *fakeStore) Get(_ context.Context, id string) (Company, error) {
	f.gotID = id
	return f.item, f.getErr
}

func (f *fakeStore) GetWithPeople(_ context.Context, id string) (CompanyDetails, error) {
	f.gotID = id
	if f.detailsErr != nil {
		return CompanyDetails{}, f.detailsErr
	}
	if f.getErr != nil {
		return CompanyDetails{}, f.getErr
	}
	if f.details.ID == "" && f.item.ID != "" {
		return CompanyDetails{Company: f.item}, nil
	}
	return f.details, nil
}

func (f *fakeStore) ListPeople(_ context.Context, id string) ([]CompanyPerson, error) {
	f.gotID = id
	return f.people, f.peopleErr
}

func (f *fakeStore) ListRelations(_ context.Context, id string) ([]CompanyRelation, error) {
	f.gotID = id
	return f.relations, f.relErr
}

func newTestRouter(store Store) *gin.Engine {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	router := gin.New()
	router.GET("/api/v1/companies", handler.List)
	router.GET("/api/v1/companies/:id", handler.Get)
	router.GET("/api/v1/companies/:id/people", handler.People)
	router.GET("/api/v1/companies/:id/relations", handler.Relations)
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

func TestListCompaniesWithoutFilter(t *testing.T) {
	store := &fakeStore{items: []Company{{ID: "magnit", Name: "Магнит"}}}

	performRequest(t, newTestRouter(store), "/api/v1/companies")

	if store.gotFilter {
		t.Fatalf("store got filter %v, want no filter", store.gotIDs)
	}
}

func TestListCompaniesWithFilter(t *testing.T) {
	store := &fakeStore{items: []Company{{ID: "magnit", Name: "Магнит"}}}

	performRequest(t, newTestRouter(store), "/api/v1/companies?ids=magnit&ids=yandex,magnit")

	if !store.gotFilter {
		t.Fatal("store got no filter, want filter")
	}
	want := []string{"magnit", "yandex"}
	if !reflect.DeepEqual(store.gotIDs, want) {
		t.Fatalf("ids = %v, want %v", store.gotIDs, want)
	}
}

func TestListCompaniesWithEmptyFilter(t *testing.T) {
	store := &fakeStore{items: []Company{}}

	recorder := performRequest(t, newTestRouter(store), "/api/v1/companies?ids=")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !store.gotFilter {
		t.Fatal("store got no filter, want empty filter")
	}
	if len(store.gotIDs) != 0 {
		t.Fatalf("ids = %v, want empty", store.gotIDs)
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

func TestGetCompanyIncludesRelations(t *testing.T) {
	store := &fakeStore{details: CompanyDetails{
		Company: Company{ID: "magnit", Name: "Магнит"},
		People:  []CompanyPerson{{Person: Person{ID: "f1", FullName: "f1"}, Relation: "linked_person"}},
		Relations: []CompanyRelation{
			{CompanyID: "tander", CompanyName: "Тандер", Type: "group", Relation: "same_group"},
		},
	}}

	recorder := performRequest(t, newTestRouter(store), "/api/v1/companies/magnit")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var body CompanyDetails
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.People) != 1 || body.People[0].Person.ID != "f1" {
		t.Fatalf("unexpected people: %+v", body.People)
	}
	if len(body.Relations) != 1 || body.Relations[0].CompanyID != "tander" {
		t.Fatalf("unexpected relations: %+v", body.Relations)
	}
}

func TestListCompanyRelations(t *testing.T) {
	store := &fakeStore{relations: []CompanyRelation{
		{CompanyID: "tander", CompanyName: "Тандер", Type: "counterparty", Relation: "payment", Direction: "outbound", Amount: 1000, Count: 3},
	}}

	recorder := performRequest(t, newTestRouter(store), "/api/v1/companies/magnit/relations")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if store.gotID != "magnit" {
		t.Fatalf("repository got id %q, want magnit", store.gotID)
	}
	var body struct {
		Items []CompanyRelation `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Items) != 1 || body.Items[0].Direction != "outbound" || body.Items[0].Count != 3 {
		t.Fatalf("unexpected relations: %+v", body.Items)
	}
}

func TestListCompanyRelationsNotFound(t *testing.T) {
	store := &fakeStore{relErr: ErrNotFound}

	recorder := performRequest(t, newTestRouter(store), "/api/v1/companies/missing/relations")

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestListCompanyRelationsError(t *testing.T) {
	store := &fakeStore{relErr: errors.New("database unavailable")}

	recorder := performRequest(t, newTestRouter(store), "/api/v1/companies/missing/relations")

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}
