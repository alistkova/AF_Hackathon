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
	method string
	path   string
	ids    []string
	scope  string
}

type companyRecorder struct {
	mu       sync.Mutex
	requests []companyRequest
}

func (r *companyRecorder) record(request *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, companyRequest{
		method: request.Method,
		path:   request.URL.Path,
		ids:    request.URL.Query()["ids"],
		scope:  request.URL.Query().Get("scope"),
	})
}

func (r *companyRecorder) snapshot() []companyRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]companyRequest(nil), r.requests...)
}

type membershipCall struct {
	UserID    string `json:"user_id"`
	CompanyID string `json:"company_id"`
	Role      string `json:"role"`
}

type membershipRecorder struct {
	mu    sync.Mutex
	calls []membershipCall
}

func (r *membershipRecorder) add(call membershipCall) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, call)
}

func (r *membershipRecorder) snapshot() []membershipCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]membershipCall(nil), r.calls...)
}

func bearer(request *http.Request) string {
	return strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
}

func newIdentityStub(t *testing.T, users map[string]auth.User, access map[string][]string) *httptest.Server {
	t.Helper()
	return newIdentityStubWithMembership(t, users, access, nil, false)
}

func newIdentityStubWithMembership(t *testing.T, users map[string]auth.User, access map[string][]string, recorder *membershipRecorder, failMembership bool) *httptest.Server {
	t.Helper()
	return newIdentityStubWithRoles(t, users, access, nil, recorder, failMembership)
}

func newIdentityStubWithRoles(t *testing.T, users map[string]auth.User, access map[string][]string, roles map[string]map[string]string, recorder *membershipRecorder, failMembership bool) *httptest.Server {
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
	mux.HandleFunc("/api/v1/me/memberships", func(w http.ResponseWriter, r *http.Request) {
		token := bearer(r)
		if _, ok := users[token]; !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		type membership struct {
			CompanyID string `json:"company_id"`
			Role      string `json:"role"`
		}
		items := []membership{}
		if roleByCompany, ok := roles[token]; ok {
			for companyID, role := range roleByCompany {
				items = append(items, membership{CompanyID: companyID, Role: role})
			}
		} else {
			for _, companyID := range access[token] {
				items = append(items, membership{CompanyID: companyID, Role: "owner"})
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
	})
	mux.HandleFunc("/api/v1/internal/memberships", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Token") != "test-internal-token" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if failMembership {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var call membershipCall
		if err := json.NewDecoder(r.Body).Decode(&call); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if recorder != nil {
			recorder.add(call)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/v1/access/grant", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/v1/access/revoke", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/v1/companies/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]string{
				"user_id": "usr_member",
				"email":   "member@example.com",
				"role":    "member",
			})
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case http.MethodPatch:
			w.WriteHeader(http.StatusNoContent)
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func newCompanyStub(t *testing.T, recorder *companyRecorder) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder.record(r)
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/companies" {
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "cmp_1", "name": "Ромашка"})
			return
		}
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"id":   strings.TrimPrefix(r.URL.Path, "/api/v1/companies/"),
				"name": "Обновлённая",
			})
			return
		}
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
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
		CompanyURL:    companyURL,
		ContentURL:    contentURL,
		InsightURL:    insightURL,
		IdentityURL:   identityURL,
		InternalToken: "test-internal-token",
		CookieName:    "alfa_session",
		Logger:        slog.New(slog.DiscardHandler),
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

func TestAdminWithoutMembershipsSeesNoCompanies(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	recorder := &companyRecorder{}
	company := newCompanyStub(t, recorder)
	router := newTestRouter(t, identity.URL, company.URL)

	response := request(t, router, "/api/v1/companies", "admin-token", "")
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

func TestAdminWithGrantsIsFiltered(t *testing.T) {
	users, access := fixture()
	access["admin-token"] = []string{"magnit"}
	identity := newIdentityStub(t, users, access)
	recorder := &companyRecorder{}
	company := newCompanyStub(t, recorder)
	router := newTestRouter(t, identity.URL, company.URL)

	response := request(t, router, "/api/v1/companies?ids=sber", "admin-token", "")
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

	response = request(t, router, "/api/v1/companies/sber", "admin-token", "")
	if response.Code != http.StatusOK {
		t.Fatalf("admin detail status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestScopeAllDoesNotBypassMemberships(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	recorder := &companyRecorder{}
	company := newCompanyStub(t, recorder)
	router := newTestRouter(t, identity.URL, company.URL)

	response := request(t, router, "/api/v1/companies?scope=all&ids=sber", "analyst-token", "")
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

func TestAdminFeedRequiresMembership(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	recorder := &insightRecorder{}
	insight := newInsightStub(t, nil, recorder)
	router := newTestRouterWith(t, identity.URL, identity.URL, identity.URL, insight.URL)

	response := request(t, router, "/api/v1/feed?company_id=sber", "admin-token", "")
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestAdminFeedUsesMemberships(t *testing.T) {
	users, access := fixture()
	access["admin-token"] = []string{"magnit"}
	identity := newIdentityStub(t, users, access)
	recorder := &insightRecorder{}
	insight := newInsightStub(t, nil, recorder)
	router := newTestRouterWith(t, identity.URL, identity.URL, identity.URL, insight.URL)

	response := request(t, router, "/api/v1/feed?company_id=sber", "admin-token", "")
	if response.Code != http.StatusNotFound {
		t.Fatalf("forbidden feed status = %d, want %d", response.Code, http.StatusNotFound)
	}

	response = request(t, router, "/api/v1/feed?company_id=magnit", "admin-token", "")
	if response.Code != http.StatusOK {
		t.Fatalf("allowed feed status = %d, want %d", response.Code, http.StatusOK)
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

func TestCreateCompanyGrantsOwnership(t *testing.T) {
	users, access := fixture()
	memberships := &membershipRecorder{}
	identity := newIdentityStubWithMembership(t, users, access, memberships, false)
	companyRecorder := &companyRecorder{}
	company := newCompanyStub(t, companyRecorder)
	router := newTestRouter(t, identity.URL, company.URL)

	response := requestJSON(t, router, http.MethodPost, "/api/v1/companies", `{"name":"Ромашка"}`, "analyst-token")
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d body %s", response.Code, http.StatusCreated, response.Body.String())
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created company: %v", err)
	}
	if created.ID != "cmp_1" {
		t.Fatalf("id = %q, want cmp_1", created.ID)
	}

	calls := companyRecorder.snapshot()
	if len(calls) != 1 || calls[0].method != http.MethodPost || calls[0].path != "/api/v1/companies" {
		t.Fatalf("company calls = %+v", calls)
	}

	membershipCalls := memberships.snapshot()
	if len(membershipCalls) != 1 {
		t.Fatalf("membership calls = %+v, want one", membershipCalls)
	}
	if membershipCalls[0].UserID != analystUser.ID || membershipCalls[0].CompanyID != "cmp_1" || membershipCalls[0].Role != "owner" {
		t.Fatalf("membership = %+v, want analyst owner of cmp_1", membershipCalls[0])
	}
}

func TestCreateCompanyRequiresSession(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	companyRecorder := &companyRecorder{}
	company := newCompanyStub(t, companyRecorder)
	router := newTestRouter(t, identity.URL, company.URL)

	response := requestJSON(t, router, http.MethodPost, "/api/v1/companies", `{"name":"Ромашка"}`, "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestCreateCompanyRollsBackOnIdentityFailure(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStubWithMembership(t, users, access, nil, true)
	companyRecorder := &companyRecorder{}
	company := newCompanyStub(t, companyRecorder)
	router := newTestRouter(t, identity.URL, company.URL)

	response := requestJSON(t, router, http.MethodPost, "/api/v1/companies", `{"name":"Ромашка"}`, "analyst-token")
	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadGateway)
	}

	calls := companyRecorder.snapshot()
	if len(calls) != 2 {
		t.Fatalf("company calls = %+v, want create and delete", calls)
	}
	if calls[0].method != http.MethodPost || calls[1].method != http.MethodDelete || calls[1].path != "/api/v1/companies/cmp_1" {
		t.Fatalf("company calls = %+v", calls)
	}
}

func TestMemberRoutesRequireMembership(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	companyRecorder := &companyRecorder{}
	company := newCompanyStub(t, companyRecorder)
	router := newTestRouter(t, identity.URL, company.URL)

	response := request(t, router, "/api/v1/companies/magnit/members", "analyst-token", "")
	if response.Code != http.StatusOK {
		t.Fatalf("member list status = %d, want %d", response.Code, http.StatusOK)
	}
	response = request(t, router, "/api/v1/companies/sber/members", "analyst-token", "")
	if response.Code != http.StatusNotFound {
		t.Fatalf("forbidden member list status = %d, want %d", response.Code, http.StatusNotFound)
	}

	response = request(t, router, "/api/v1/companies/sber/members", "admin-token", "")
	if response.Code != http.StatusOK {
		t.Fatalf("admin member list status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestAddCompanyMemberProxies(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	companyRecorder := &companyRecorder{}
	company := newCompanyStub(t, companyRecorder)
	router := newTestRouter(t, identity.URL, company.URL)

	response := requestJSON(t, router, http.MethodPost, "/api/v1/companies/magnit/members", `{"email":"member@example.com"}`, "analyst-token")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body %s", response.Code, http.StatusOK, response.Body.String())
	}
}

func TestUpdateCompanyMemberProxies(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	companyRecorder := &companyRecorder{}
	company := newCompanyStub(t, companyRecorder)
	router := newTestRouter(t, identity.URL, company.URL)

	response := requestJSON(t, router, http.MethodPatch, "/api/v1/companies/magnit/members/usr_member", `{"role":"owner"}`, "analyst-token")
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d body %s", response.Code, http.StatusNoContent, response.Body.String())
	}
}

func TestGrantAccessValidatesCompany(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	recorder := &companyRecorder{}
	company := newCompanyStub(t, recorder)
	router := newTestRouter(t, identity.URL, company.URL)

	response := requestJSON(t, router, http.MethodPost, "/api/v1/access/grant",
		`{"user_id":"usr_analyst","company_id":"magnit","role":"owner"}`, "admin-token")
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d body %s", response.Code, http.StatusNoContent, response.Body.String())
	}

	response = requestJSON(t, router, http.MethodPost, "/api/v1/access/grant",
		`{"user_id":"usr_analyst","company_id":"magnit","role":"owner"}`, "analyst-token")
	if response.Code != http.StatusForbidden {
		t.Fatalf("non-admin status = %d, want %d", response.Code, http.StatusForbidden)
	}

	missing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(missing.Close)
	missingRouter := newTestRouter(t, identity.URL, missing.URL)
	response = requestJSON(t, missingRouter, http.MethodPost, "/api/v1/access/grant",
		`{"user_id":"usr_analyst","company_id":"missing","role":"owner"}`, "admin-token")
	if response.Code != http.StatusNotFound {
		t.Fatalf("missing company status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestCreateCompanyPropagatesValidationError(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	company := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"code":    "invalid_company",
			"message": "Укажите название компании",
		})
	}))
	t.Cleanup(company.Close)
	router := newTestRouter(t, identity.URL, company.URL)

	response := requestJSON(t, router, http.MethodPost, "/api/v1/companies", `{"name":""}`, "analyst-token")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if !strings.Contains(response.Body.String(), "Укажите название компании") {
		t.Fatalf("body = %s, want validation message", response.Body.String())
	}
}

func TestOwnerCanUpdateCompany(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	recorder := &companyRecorder{}
	company := newCompanyStub(t, recorder)
	router := newTestRouter(t, identity.URL, company.URL)

	response := requestJSON(t, router, http.MethodPut, "/api/v1/companies/magnit", `{"name":"Магнит+"}`, "analyst-token")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body %s", response.Code, http.StatusOK, response.Body.String())
	}
	calls := recorder.snapshot()
	if len(calls) != 2 || calls[0].method != http.MethodGet || calls[1].method != http.MethodPut || calls[1].path != "/api/v1/companies/magnit" {
		t.Fatalf("company calls = %+v", calls)
	}
}

func TestMemberCannotUpdateCompany(t *testing.T) {
	users, access := fixture()
	roles := map[string]map[string]string{"analyst-token": {"magnit": "member"}}
	identity := newIdentityStubWithRoles(t, users, access, roles, nil, false)
	recorder := &companyRecorder{}
	company := newCompanyStub(t, recorder)
	router := newTestRouter(t, identity.URL, company.URL)

	response := requestJSON(t, router, http.MethodPut, "/api/v1/companies/magnit", `{"name":"Магнит+"}`, "analyst-token")
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
	if calls := recorder.snapshot(); len(calls) != 1 || calls[0].method != http.MethodGet {
		t.Fatalf("company calls = %+v, want only the existence check", calls)
	}
}

func TestOutsiderCannotUpdateCompany(t *testing.T) {
	users, access := fixture()
	access["analyst-token"] = nil
	identity := newIdentityStub(t, users, access)
	recorder := &companyRecorder{}
	company := newCompanyStub(t, recorder)
	router := newTestRouter(t, identity.URL, company.URL)

	response := requestJSON(t, router, http.MethodPut, "/api/v1/companies/magnit", `{"name":"Магнит+"}`, "analyst-token")
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestAdminCanUpdateCompany(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	recorder := &companyRecorder{}
	company := newCompanyStub(t, recorder)
	router := newTestRouter(t, identity.URL, company.URL)

	response := requestJSON(t, router, http.MethodPut, "/api/v1/companies/sber", `{"name":"Сбер"}`, "admin-token")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestUpdateCompanyPropagatesNotFound(t *testing.T) {
	users, access := fixture()
	identity := newIdentityStub(t, users, access)
	company := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"code":    "company_not_found",
			"message": "Компания не найдена",
		})
	}))
	t.Cleanup(company.Close)
	router := newTestRouter(t, identity.URL, company.URL)

	response := requestJSON(t, router, http.MethodPut, "/api/v1/companies/missing", `{"name":"Ромашка"}`, "analyst-token")
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}
