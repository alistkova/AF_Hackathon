package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/alfa-hack/alfa-hack/services/gateway/internal/auth"
	"github.com/gin-gonic/gin"
)

type companyRequest struct {
	path string
	ids  []string
}

type companyRecorder struct {
	mu       sync.Mutex
	requests []companyRequest
}

func (r *companyRecorder) record(request *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, companyRequest{
		path: request.URL.Path,
		ids:  request.URL.Query()["ids"],
	})
}

func (r *companyRecorder) snapshot() []companyRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]companyRequest(nil), r.requests...)
}

func bearer(request *http.Request) string {
	return strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
}

func newIdentityStub(t *testing.T, users map[string]auth.User, access map[string][]string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/me", func(w http.ResponseWriter, r *http.Request) {
		user, ok := users[bearer(r)]
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(user)
	})
	mux.HandleFunc("/api/v1/me/companies", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := users[bearer(r)]; !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		items := access[bearer(r)]
		if items == nil {
			items = []string{}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func newCompanyStub(t *testing.T, recorder *companyRecorder) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder.record(r)
		if r.URL.Path == "/api/v1/companies" {
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []string{}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"id": strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/companies/"), "/")[0]})
	}))
	t.Cleanup(server.Close)
	return server
}

func newTestRouter(t *testing.T, identityURL, companyURL string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return New(Config{
		CompanyURL:  companyURL,
		ContentURL:  companyURL,
		InsightURL:  companyURL,
		IdentityURL: identityURL,
		CookieName:  "alfa_session",
		Logger:      slog.New(slog.DiscardHandler),
	})
}

type recorder struct {
	*httptest.ResponseRecorder
}

func (r recorder) CloseNotify() <-chan bool {
	return make(chan bool)
}

func request(t *testing.T, router *gin.Engine, path, token, cookie string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "alfa_session", Value: cookie})
	}
	rec := recorder{ResponseRecorder: httptest.NewRecorder()}
	router.ServeHTTP(rec, req)
	return rec.ResponseRecorder
}

var (
	adminUser   = auth.User{ID: "usr_admin", Email: "admin@example.com", Role: "admin"}
	analystUser = auth.User{ID: "usr_analyst", Email: "analyst@example.com", Role: "analyst"}
)

func fixture() (map[string]auth.User, map[string][]string) {
	users := map[string]auth.User{"admin-token": adminUser, "analyst-token": analystUser}
	access := map[string][]string{"admin-token": nil, "analyst-token": {"magnit"}}
	return users, access
}

func TestCompaniesRequireSession(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	recorder := &companyRecorder{}
	company := newCompanyStub(t, recorder)
	router := newTestRouter(t, identity.URL, company.URL)

	recorderResponse := request(t, router, "/api/v1/companies", "", "")
	if recorderResponse.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorderResponse.Code, http.StatusUnauthorized)
	}
	if calls := recorder.snapshot(); len(calls) != 0 {
		t.Fatalf("company service was called %d times, want 0", len(calls))
	}
}

func TestInvalidSessionIsRejected(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	recorder := &companyRecorder{}
	company := newCompanyStub(t, recorder)
	router := newTestRouter(t, identity.URL, company.URL)

	response := request(t, router, "/api/v1/companies", "bogus", "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestAdminSeesAllCompanies(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	recorder := &companyRecorder{}
	company := newCompanyStub(t, recorder)
	router := newTestRouter(t, identity.URL, company.URL)

	response := request(t, router, "/api/v1/companies", "admin-token", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	calls := recorder.snapshot()
	if len(calls) != 1 {
		t.Fatalf("company calls = %d, want 1", len(calls))
	}
	if len(calls[0].ids) != 0 {
		t.Fatalf("ids = %v, want no filter", calls[0].ids)
	}
}

func TestAnalystListIsFiltered(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	recorder := &companyRecorder{}
	company := newCompanyStub(t, recorder)
	router := newTestRouter(t, identity.URL, company.URL)

	response := request(t, router, "/api/v1/companies?ids=sber", "analyst-token", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	calls := recorder.snapshot()
	if len(calls) != 1 {
		t.Fatalf("company calls = %d, want 1", len(calls))
	}
	if len(calls[0].ids) != 1 || calls[0].ids[0] != "magnit" {
		t.Fatalf("ids = %v, want [magnit]", calls[0].ids)
	}
}

func TestAnalystWithoutCompaniesGetsEmptyList(t *testing.T) {
	users := map[string]auth.User{"analyst-token": analystUser}
	access := map[string][]string{"analyst-token": {}}
	identity := newIdentityStub(t, users, access)
	recorder := &companyRecorder{}
	company := newCompanyStub(t, recorder)
	router := newTestRouter(t, identity.URL, company.URL)

	response := request(t, router, "/api/v1/companies", "analyst-token", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if body := strings.TrimSpace(response.Body.String()); body != `{"items":[]}` {
		t.Fatalf("body = %s, want empty items", body)
	}
	if calls := recorder.snapshot(); len(calls) != 0 {
		t.Fatalf("company calls = %d, want 0", len(calls))
	}
}

func TestAnalystCannotOpenForbiddenCompany(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	recorder := &companyRecorder{}
	company := newCompanyStub(t, recorder)
	router := newTestRouter(t, identity.URL, company.URL)

	response := request(t, router, "/api/v1/companies/sber", "analyst-token", "")
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	if calls := recorder.snapshot(); len(calls) != 0 {
		t.Fatalf("company calls = %d, want 0", len(calls))
	}
}

func TestAnalystCanOpenGrantedCompany(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	recorder := &companyRecorder{}
	company := newCompanyStub(t, recorder)
	router := newTestRouter(t, identity.URL, company.URL)

	response := request(t, router, "/api/v1/companies/magnit", "analyst-token", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	calls := recorder.snapshot()
	if len(calls) != 1 || calls[0].path != "/api/v1/companies/magnit" {
		t.Fatalf("company calls = %+v", calls)
	}
}

func TestCookieSessionIsAccepted(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	recorder := &companyRecorder{}
	company := newCompanyStub(t, recorder)
	router := newTestRouter(t, identity.URL, company.URL)

	response := request(t, router, "/api/v1/companies/magnit", "", "analyst-token")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestIdentityUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	recorder := &companyRecorder{}
	company := newCompanyStub(t, recorder)
	router := newTestRouter(t, server.URL, company.URL)

	response := request(t, router, "/api/v1/companies", "admin-token", "")
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

func TestUnknownRoute(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	recorder := &companyRecorder{}
	company := newCompanyStub(t, recorder)
	router := newTestRouter(t, identity.URL, company.URL)

	response := request(t, router, "/api/v1/companies/magnit/unknown", "admin-token", "")
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	if calls := recorder.snapshot(); len(calls) != 0 {
		t.Fatalf("company calls = %d, want 0", len(calls))
	}
}
