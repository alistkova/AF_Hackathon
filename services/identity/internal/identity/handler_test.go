package identity

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func TestMain(m *testing.M) {
	passwordCost = bcrypt.MinCost
	os.Exit(m.Run())
}

func newTestRouter(store Store) *gin.Engine {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(store, slog.New(slog.DiscardHandler))
	router := gin.New()
	auth := AuthMiddleware(store)

	v1 := router.Group("/api/v1")
	v1.POST("/auth/register", handler.Register)
	v1.POST("/auth/login", handler.Login)
	v1.POST("/auth/logout", handler.Logout)

	protected := v1.Group("")
	protected.Use(auth)
	protected.GET("/me", handler.Me)
	protected.GET("/me/companies", handler.AccessibleCompanies)

	admin := protected.Group("/access")
	admin.Use(RequireAdmin())
	admin.POST("/grant", handler.GrantAccess)
	admin.POST("/revoke", handler.RevokeAccess)

	return router
}

func perform(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func performWithToken(router *gin.Engine, method, path, body, token string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func registerUser(t *testing.T, router *gin.Engine, email, password string) User {
	t.Helper()
	rec := perform(router, http.MethodPost, "/api/v1/auth/register", fmt.Sprintf(`{"email":%q,"password":%q}`, email, password))
	if rec.Code != http.StatusCreated {
		t.Fatalf("register %s: status %d body %s", email, rec.Code, rec.Body.String())
	}
	var user User
	if err := json.Unmarshal(rec.Body.Bytes(), &user); err != nil {
		t.Fatalf("decode user: %v", err)
	}
	return user
}

func loginUser(t *testing.T, router *gin.Engine, email, password string) Session {
	t.Helper()
	rec := perform(router, http.MethodPost, "/api/v1/auth/login", fmt.Sprintf(`{"email":%q,"password":%q}`, email, password))
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s: status %d body %s", email, rec.Code, rec.Body.String())
	}
	var session Session
	if err := json.Unmarshal(rec.Body.Bytes(), &session); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	if session.Token == "" {
		t.Fatalf("login %s: empty token", email)
	}
	return session
}

func TestRegisterFirstUserBecomesAdmin(t *testing.T) {
	router := newTestRouter(NewMemStore())

	first := registerUser(t, router, "owner@example.com", "secret-pass-1")
	if first.Role != RoleAdmin {
		t.Fatalf("first user role = %q, want %q", first.Role, RoleAdmin)
	}
	if first.ID == "" {
		t.Fatal("first user id is empty")
	}

	second := registerUser(t, router, "analyst@example.com", "secret-pass-2")
	if second.Role != RoleAnalyst {
		t.Fatalf("second user role = %q, want %q", second.Role, RoleAnalyst)
	}
}

func TestRegisterRejectsInvalidInput(t *testing.T) {
	router := newTestRouter(NewMemStore())

	cases := []struct {
		name string
		body string
	}{
		{"short password", `{"email":"a@b.com","password":"short"}`},
		{"invalid email", `{"email":"not-an-email","password":"secret-pass"}`},
		{"missing password", `{"email":"a@b.com"}`},
		{"empty body", ``},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := perform(router, http.MethodPost, "/api/v1/auth/register", tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status %d, want %d", rec.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestRegisterRejectsDuplicateEmail(t *testing.T) {
	router := newTestRouter(NewMemStore())
	registerUser(t, router, "dup@example.com", "secret-pass")

	rec := perform(router, http.MethodPost, "/api/v1/auth/register", `{"email":"dup@example.com","password":"secret-pass"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestLoginSetsSessionCookie(t *testing.T) {
	router := newTestRouter(NewMemStore())
	registerUser(t, router, "user@example.com", "secret-pass")

	rec := perform(router, http.MethodPost, "/api/v1/auth/login", `{"email":"user@example.com","password":"secret-pass"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want %d", rec.Code, http.StatusOK)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != defaultCookieName {
		t.Fatalf("cookie name = %q, want %q", cookie.Name, defaultCookieName)
	}
	if !cookie.HttpOnly {
		t.Fatal("cookie is not http-only")
	}
	if cookie.Value == "" {
		t.Fatal("cookie value is empty")
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	router := newTestRouter(NewMemStore())
	registerUser(t, router, "user@example.com", "secret-pass")

	rec := perform(router, http.MethodPost, "/api/v1/auth/login", `{"email":"user@example.com","password":"wrong-pass"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	rec = perform(router, http.MethodPost, "/api/v1/auth/login", `{"email":"missing@example.com","password":"secret-pass"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMeRequiresAuthentication(t *testing.T) {
	router := newTestRouter(NewMemStore())

	rec := performWithToken(router, http.MethodGet, "/api/v1/me", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	rec = performWithToken(router, http.MethodGet, "/api/v1/me", "", "bogus-token")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMeReturnsCurrentUser(t *testing.T) {
	router := newTestRouter(NewMemStore())
	user := registerUser(t, router, "user@example.com", "secret-pass")
	session := loginUser(t, router, "user@example.com", "secret-pass")

	rec := performWithToken(router, http.MethodGet, "/api/v1/me", "", session.Token)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want %d", rec.Code, http.StatusOK)
	}

	var got User
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode user: %v", err)
	}
	if got.ID != user.ID || got.Email != user.Email || got.Role != user.Role {
		t.Fatalf("me = %+v, want %+v", got, user)
	}
}

func TestLogoutInvalidatesSession(t *testing.T) {
	router := newTestRouter(NewMemStore())
	registerUser(t, router, "user@example.com", "secret-pass")
	session := loginUser(t, router, "user@example.com", "secret-pass")

	rec := performWithToken(router, http.MethodPost, "/api/v1/auth/logout", "", session.Token)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout status %d, want %d", rec.Code, http.StatusNoContent)
	}

	rec = performWithToken(router, http.MethodGet, "/api/v1/me", "", session.Token)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout status %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestGrantAccessRequiresAdmin(t *testing.T) {
	router := newTestRouter(NewMemStore())
	registerUser(t, router, "owner@example.com", "secret-pass")
	analyst := registerUser(t, router, "analyst@example.com", "secret-pass")
	session := loginUser(t, router, "analyst@example.com", "secret-pass")

	body := fmt.Sprintf(`{"user_id":%q,"company_id":"c1"}`, analyst.ID)
	rec := performWithToken(router, http.MethodPost, "/api/v1/access/grant", body, session.Token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestGrantAndRevokeAccess(t *testing.T) {
	router := newTestRouter(NewMemStore())
	registerUser(t, router, "owner@example.com", "secret-pass")
	adminSession := loginUser(t, router, "owner@example.com", "secret-pass")
	analyst := registerUser(t, router, "analyst@example.com", "secret-pass")
	analystSession := loginUser(t, router, "analyst@example.com", "secret-pass")

	grant := fmt.Sprintf(`{"user_id":%q,"company_id":"c1"}`, analyst.ID)
	rec := performWithToken(router, http.MethodPost, "/api/v1/access/grant", grant, adminSession.Token)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("grant status %d, want %d", rec.Code, http.StatusNoContent)
	}

	rec = performWithToken(router, http.MethodGet, "/api/v1/me/companies", "", analystSession.Token)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status %d, want %d", rec.Code, http.StatusOK)
	}
	var list struct {
		Items []string `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0] != "c1" {
		t.Fatalf("items = %v, want [c1]", list.Items)
	}

	rec = performWithToken(router, http.MethodPost, "/api/v1/access/revoke", grant, adminSession.Token)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("revoke status %d, want %d", rec.Code, http.StatusNoContent)
	}

	rec = performWithToken(router, http.MethodGet, "/api/v1/me/companies", "", analystSession.Token)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status %d, want %d", rec.Code, http.StatusOK)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("items = %v, want empty", list.Items)
	}
}

func TestGrantAccessUnknownUser(t *testing.T) {
	router := newTestRouter(NewMemStore())
	registerUser(t, router, "owner@example.com", "secret-pass")
	adminSession := loginUser(t, router, "owner@example.com", "secret-pass")

	rec := performWithToken(router, http.MethodPost, "/api/v1/access/grant", `{"user_id":"missing","company_id":"c1"}`, adminSession.Token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want %d", rec.Code, http.StatusNotFound)
	}
}
