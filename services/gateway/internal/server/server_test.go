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
	return newTestRouterWith(t, identityURL, companyURL, companyURL, companyURL)
}

func newTestRouterWith(t *testing.T, identityURL, companyURL, contentURL, insightURL string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return New(Config{
		CompanyURL:  companyURL,
		ContentURL:  contentURL,
		InsightURL:  insightURL,
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

type insightRecorder struct {
	mu       sync.Mutex
	requests []string
}

func (r *insightRecorder) add(method, path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, method+" "+path)
}

func (r *insightRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.requests...)
}

func newInsightStub(t *testing.T, jobs map[string]string, recorder *insightRecorder) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/feed", func(w http.ResponseWriter, r *http.Request) {
		recorder.add(r.Method, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	})
	mux.HandleFunc("/api/v1/insights/jobs", func(w http.ResponseWriter, r *http.Request) {
		recorder.add(r.Method, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"job_id":"job-1","company_id":"magnit","status":"queued","events_url":"/api/v1/insights/jobs/job-1/events"}`))
	})
	mux.HandleFunc("/api/v1/insights/jobs/", func(w http.ResponseWriter, r *http.Request) {
		recorder.add(r.Method, r.URL.Path)
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/insights/jobs/")
		if strings.HasSuffix(id, "/events") {
			id = strings.TrimSuffix(id, "/events")
			if companyID, ok := jobs[id]; ok {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = w.Write([]byte("event: result\ndata: {\"type\":\"result\",\"job_id\":\"" + id + "\",\"company_id\":\"" + companyID + "\"}\n\n"))
				return
			}
			http.NotFound(w, r)
			return
		}
		companyID, ok := jobs[id]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"job_id":"` + id + `","company_id":"` + companyID + `","status":"done"}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func requestJSON(t *testing.T, router *gin.Engine, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := recorder{ResponseRecorder: httptest.NewRecorder()}
	router.ServeHTTP(rec, req)
	return rec.ResponseRecorder
}

func TestFeedRequiresSession(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	recorder := &insightRecorder{}
	insight := newInsightStub(t, map[string]string{"job-1": "magnit"}, recorder)
	router := newTestRouterWith(t, identity.URL, identity.URL, identity.URL, insight.URL)

	response := request(t, router, "/api/v1/feed?company_id=magnit", "", "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if calls := recorder.snapshot(); len(calls) != 0 {
		t.Fatalf("insight calls = %v, want none", calls)
	}
}

func TestFeedRequiresCompany(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	recorder := &insightRecorder{}
	insight := newInsightStub(t, nil, recorder)
	router := newTestRouterWith(t, identity.URL, identity.URL, identity.URL, insight.URL)

	response := request(t, router, "/api/v1/feed", "analyst-token", "")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestFeedRejectsForbiddenCompany(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	recorder := &insightRecorder{}
	insight := newInsightStub(t, nil, recorder)
	router := newTestRouterWith(t, identity.URL, identity.URL, identity.URL, insight.URL)

	response := request(t, router, "/api/v1/feed?company_id=sber", "analyst-token", "")
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	if calls := recorder.snapshot(); len(calls) != 0 {
		t.Fatalf("insight calls = %v, want none", calls)
	}
}

func TestFeedAllowsGrantedCompany(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	recorder := &insightRecorder{}
	insight := newInsightStub(t, nil, recorder)
	router := newTestRouterWith(t, identity.URL, identity.URL, identity.URL, insight.URL)

	response := request(t, router, "/api/v1/feed?company_id=magnit", "analyst-token", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if calls := recorder.snapshot(); len(calls) != 1 {
		t.Fatalf("insight calls = %v, want one", calls)
	}
}

func TestAdminFeedAllowsAnyCompany(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	recorder := &insightRecorder{}
	insight := newInsightStub(t, nil, recorder)
	router := newTestRouterWith(t, identity.URL, identity.URL, identity.URL, insight.URL)

	response := request(t, router, "/api/v1/feed?company_id=sber", "admin-token", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestJobCreateRejectsForbiddenCompany(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	recorder := &insightRecorder{}
	insight := newInsightStub(t, nil, recorder)
	router := newTestRouterWith(t, identity.URL, identity.URL, identity.URL, insight.URL)

	response := requestJSON(t, router, http.MethodPost, "/api/v1/insights/jobs",
		`{"company_id":"sber","event":{"id":"e1","title":"test"}}`, "analyst-token")
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	if calls := recorder.snapshot(); len(calls) != 0 {
		t.Fatalf("insight calls = %v, want none", calls)
	}
}

func TestJobCreateAllowsGrantedCompany(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	recorder := &insightRecorder{}
	insight := newInsightStub(t, nil, recorder)
	router := newTestRouterWith(t, identity.URL, identity.URL, identity.URL, insight.URL)

	response := requestJSON(t, router, http.MethodPost, "/api/v1/insights/jobs",
		`{"company_id":"magnit","event":{"id":"e1","title":"test"}}`, "analyst-token")
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusAccepted)
	}
	if calls := recorder.snapshot(); len(calls) != 1 {
		t.Fatalf("insight calls = %v, want one", calls)
	}
}

func TestJobReadRejectsForbiddenCompany(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	recorder := &insightRecorder{}
	insight := newInsightStub(t, map[string]string{"job-1": "sber"}, recorder)
	router := newTestRouterWith(t, identity.URL, identity.URL, identity.URL, insight.URL)

	response := request(t, router, "/api/v1/insights/jobs/job-1", "analyst-token", "")
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	if calls := recorder.snapshot(); len(calls) != 1 {
		t.Fatalf("insight calls = %v, want only the lookup", calls)
	}
}

func TestJobReadAllowsGrantedCompany(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	recorder := &insightRecorder{}
	insight := newInsightStub(t, map[string]string{"job-1": "magnit"}, recorder)
	router := newTestRouterWith(t, identity.URL, identity.URL, identity.URL, insight.URL)

	response := request(t, router, "/api/v1/insights/jobs/job-1", "analyst-token", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if calls := recorder.snapshot(); len(calls) != 2 {
		t.Fatalf("insight calls = %v, want lookup and proxy", calls)
	}
}

func TestJobStreamRejectsForbiddenCompany(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	recorder := &insightRecorder{}
	insight := newInsightStub(t, map[string]string{"job-1": "sber"}, recorder)
	router := newTestRouterWith(t, identity.URL, identity.URL, identity.URL, insight.URL)

	response := request(t, router, "/api/v1/insights/jobs/job-1/events", "analyst-token", "")
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	if calls := recorder.snapshot(); len(calls) != 1 {
		t.Fatalf("insight calls = %v, want only the lookup", calls)
	}
}

func TestJobNotFound(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	recorder := &insightRecorder{}
	insight := newInsightStub(t, nil, recorder)
	router := newTestRouterWith(t, identity.URL, identity.URL, identity.URL, insight.URL)

	response := request(t, router, "/api/v1/insights/jobs/missing", "admin-token", "")
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestDocumentIngestRequiresAdmin(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	content := newCompanyStub(t, &companyRecorder{})
	router := newTestRouterWith(t, identity.URL, identity.URL, content.URL, identity.URL)

	response := requestJSON(t, router, http.MethodPost, "/api/v1/documents",
		`{"items":[{"title":"t","full_text":"body"}]}`, "analyst-token")
	if response.Code != http.StatusForbidden {
		t.Fatalf("analyst status = %d, want %d", response.Code, http.StatusForbidden)
	}

	response = requestJSON(t, router, http.MethodPost, "/api/v1/documents",
		`{"items":[{"title":"t","full_text":"body"}]}`, "admin-token")
	if response.Code != http.StatusOK {
		t.Fatalf("admin status = %d, want %d", response.Code, http.StatusOK)
	}
}
