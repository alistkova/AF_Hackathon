package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func TestMain(m *testing.M) {
	passwordCost = bcrypt.MinCost
	_ = os.Setenv("IDENTITY_REGISTRATION_ENABLED", "true")
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
	protected.PATCH("/me", handler.UpdateMe)
	protected.POST("/me/password", handler.ChangePassword)
	protected.GET("/me/companies", handler.AccessibleCompanies)
	protected.GET("/me/memberships", handler.Memberships)
	protected.GET("/companies/:companyId/members", handler.CompanyMembers)
	protected.POST("/companies/:companyId/members", handler.AddCompanyMember)
	protected.DELETE("/companies/:companyId/members/:userId", handler.RemoveCompanyMember)

	internal := v1.Group("/internal")
	internal.POST("/memberships", handler.CreateMembership)

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

func TestRegisterCanBeDisabled(t *testing.T) {
	t.Setenv("IDENTITY_REGISTRATION_ENABLED", "false")
	router := newTestRouter(NewMemStore())

	rec := perform(router, http.MethodPost, "/api/v1/auth/register", `{"email":"user@example.com","password":"secret-pass"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestLoginIsRateLimited(t *testing.T) {
	t.Setenv("IDENTITY_LOGIN_MAX_ATTEMPTS", "2")
	t.Setenv("IDENTITY_LOGIN_WINDOW", "1m")

	router := newTestRouter(NewMemStore())
	registerUser(t, router, "user@example.com", "secret-pass")

	for attempt := 0; attempt < 2; attempt++ {
		rec := perform(router, http.MethodPost, "/api/v1/auth/login", `{"email":"user@example.com","password":"wrong-pass"}`)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status %d, want %d", attempt, rec.Code, http.StatusUnauthorized)
		}
	}

	rec := perform(router, http.MethodPost, "/api/v1/auth/login", `{"email":"user@example.com","password":"wrong-pass"}`)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("blocked status %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("Retry-After header is missing")
	}

	rec = perform(router, http.MethodPost, "/api/v1/auth/login", `{"email":"user@example.com","password":"secret-pass"}`)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("correct password while blocked: status %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestUpdateProfileEmail(t *testing.T) {
	router := newTestRouter(NewMemStore())
	registerUser(t, router, "user@example.com", "secret-pass")
	session := loginUser(t, router, "user@example.com", "secret-pass")

	rec := performWithToken(router, http.MethodPatch, "/api/v1/me", `{"email":"New@Example.com"}`, session.Token)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want %d body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got User
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode user: %v", err)
	}
	if got.Email != "new@example.com" {
		t.Fatalf("email = %q, want new@example.com", got.Email)
	}

	rec = performWithToken(router, http.MethodGet, "/api/v1/me", "", session.Token)
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode me: %v", err)
	}
	if got.Email != "new@example.com" {
		t.Fatalf("me email = %q, want new@example.com", got.Email)
	}
}

func TestUpdateProfileRejectsDuplicateEmail(t *testing.T) {
	router := newTestRouter(NewMemStore())
	registerUser(t, router, "first@example.com", "secret-pass")
	registerUser(t, router, "second@example.com", "secret-pass")
	session := loginUser(t, router, "first@example.com", "secret-pass")

	rec := performWithToken(router, http.MethodPatch, "/api/v1/me", `{"email":"second@example.com"}`, session.Token)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestUpdateProfileRejectsInvalidInput(t *testing.T) {
	router := newTestRouter(NewMemStore())
	registerUser(t, router, "user@example.com", "secret-pass")
	session := loginUser(t, router, "user@example.com", "secret-pass")

	cases := []struct {
		name string
		body string
	}{
		{"invalid email", `{"email":"not-an-email"}`},
		{"empty body", `{}`},
		{"long company id", fmt.Sprintf(`{"primary_company_id":%q}`, strings.Repeat("x", 200))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := performWithToken(router, http.MethodPatch, "/api/v1/me", tc.body, session.Token)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status %d, want %d", rec.Code, http.StatusBadRequest)
			}
		})
	}

	rec := performWithToken(router, http.MethodPatch, "/api/v1/me", `{"email":"a@b.com"}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestUpdatePrimaryCompany(t *testing.T) {
	router := newTestRouter(NewMemStore())
	registerUser(t, router, "user@example.com", "secret-pass")
	session := loginUser(t, router, "user@example.com", "secret-pass")

	rec := performWithToken(router, http.MethodPatch, "/api/v1/me", `{"primary_company_id":"magnit"}`, session.Token)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want %d", rec.Code, http.StatusOK)
	}

	rec = performWithToken(router, http.MethodGet, "/api/v1/me", "", session.Token)
	var got User
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode me: %v", err)
	}
	if got.PrimaryCompanyID != "magnit" {
		t.Fatalf("primary company = %q, want magnit", got.PrimaryCompanyID)
	}

	reopened := loginUser(t, router, "user@example.com", "secret-pass")
	if reopened.User.PrimaryCompanyID != "magnit" {
		t.Fatalf("login primary company = %q, want magnit", reopened.User.PrimaryCompanyID)
	}

	rec = performWithToken(router, http.MethodPatch, "/api/v1/me", `{"primary_company_id":""}`, session.Token)
	if rec.Code != http.StatusOK {
		t.Fatalf("clear status %d, want %d", rec.Code, http.StatusOK)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode user: %v", err)
	}
	if got.PrimaryCompanyID != "" {
		t.Fatalf("primary company = %q, want empty", got.PrimaryCompanyID)
	}
}

func TestChangePassword(t *testing.T) {
	router := newTestRouter(NewMemStore())
	registerUser(t, router, "user@example.com", "secret-pass")
	session := loginUser(t, router, "user@example.com", "secret-pass")
	other := loginUser(t, router, "user@example.com", "secret-pass")

	rec := performWithToken(router, http.MethodPost, "/api/v1/me/password",
		`{"current_password":"wrong-pass","new_password":"next-secret"}`, session.Token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("wrong current status %d, want %d", rec.Code, http.StatusBadRequest)
	}

	rec = performWithToken(router, http.MethodPost, "/api/v1/me/password",
		`{"current_password":"secret-pass","new_password":"short"}`, session.Token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("short new password status %d, want %d", rec.Code, http.StatusBadRequest)
	}

	rec = performWithToken(router, http.MethodPost, "/api/v1/me/password",
		`{"current_password":"secret-pass","new_password":"next-secret"}`, session.Token)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d, want %d body %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	rec = perform(router, http.MethodPost, "/api/v1/auth/login", `{"email":"user@example.com","password":"secret-pass"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("old password status %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	loginUser(t, router, "user@example.com", "next-secret")

	rec = performWithToken(router, http.MethodGet, "/api/v1/me", "", session.Token)
	if rec.Code != http.StatusOK {
		t.Fatalf("current session status %d, want %d", rec.Code, http.StatusOK)
	}
	rec = performWithToken(router, http.MethodGet, "/api/v1/me", "", other.Token)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("other session status %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMembershipsAndMembers(t *testing.T) {
	router := newTestRouter(NewMemStore())
	admin := registerUser(t, router, "owner@example.com", "secret-pass")
	owner := registerUser(t, router, "analyst@example.com", "secret-pass")
	member := registerUser(t, router, "member@example.com", "secret-pass")
	adminSession := loginUser(t, router, "owner@example.com", "secret-pass")
	ownerSession := loginUser(t, router, "analyst@example.com", "secret-pass")
	memberSession := loginUser(t, router, "member@example.com", "secret-pass")

	rec := performWithToken(router, http.MethodPost, "/api/v1/access/grant",
		fmt.Sprintf(`{"user_id":%q,"company_id":"c1","role":"owner"}`, owner.ID), adminSession.Token)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("grant owner status %d, want %d", rec.Code, http.StatusNoContent)
	}
	rec = performWithToken(router, http.MethodPost, "/api/v1/access/grant",
		fmt.Sprintf(`{"user_id":%q,"company_id":"c1"}`, member.ID), adminSession.Token)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("grant member status %d, want %d", rec.Code, http.StatusNoContent)
	}

	rec = performWithToken(router, http.MethodGet, "/api/v1/me/memberships", "", ownerSession.Token)
	if rec.Code != http.StatusOK {
		t.Fatalf("memberships status %d, want %d", rec.Code, http.StatusOK)
	}
	var memberships struct {
		Items []Membership `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &memberships); err != nil {
		t.Fatalf("decode memberships: %v", err)
	}
	if len(memberships.Items) != 1 || memberships.Items[0].CompanyID != "c1" || memberships.Items[0].Role != CompanyRoleOwner {
		t.Fatalf("memberships = %+v, want c1 owner", memberships.Items)
	}

	rec = performWithToken(router, http.MethodGet, "/api/v1/companies/c1/members", "", memberSession.Token)
	if rec.Code != http.StatusOK {
		t.Fatalf("members status %d, want %d", rec.Code, http.StatusOK)
	}
	var members struct {
		Items []Member `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &members); err != nil {
		t.Fatalf("decode members: %v", err)
	}
	if len(members.Items) != 2 {
		t.Fatalf("members = %+v, want 2", members.Items)
	}

	rec = performWithToken(router, http.MethodPost, "/api/v1/companies/c1/members",
		fmt.Sprintf(`{"email":%q}`, member.Email), ownerSession.Token)
	if rec.Code != http.StatusOK {
		t.Fatalf("duplicate add status %d, want %d", rec.Code, http.StatusOK)
	}
	rec = performWithToken(router, http.MethodPost, "/api/v1/companies/c1/members",
		`{"email":"missing@example.com"}`, ownerSession.Token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown email status %d, want %d", rec.Code, http.StatusNotFound)
	}

	rec = performWithToken(router, http.MethodDelete, "/api/v1/companies/c1/members/"+member.ID, "", ownerSession.Token)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("remove member status %d, want %d", rec.Code, http.StatusNoContent)
	}
	rec = performWithToken(router, http.MethodDelete, "/api/v1/companies/c1/members/"+member.ID, "", ownerSession.Token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("second remove status %d, want %d", rec.Code, http.StatusNotFound)
	}

	rec = performWithToken(router, http.MethodDelete, "/api/v1/companies/c1/members/"+owner.ID, "", ownerSession.Token)
	if rec.Code != http.StatusConflict {
		t.Fatalf("last owner status %d, want %d", rec.Code, http.StatusConflict)
	}

	if admin.ID == "" {
		t.Fatal("admin id is empty")
	}
}

func TestCompanyMemberRequiresOwner(t *testing.T) {
	router := newTestRouter(NewMemStore())
	registerUser(t, router, "owner@example.com", "secret-pass")
	owner := registerUser(t, router, "analyst@example.com", "secret-pass")
	member := registerUser(t, router, "member@example.com", "secret-pass")
	outsider := registerUser(t, router, "outsider@example.com", "secret-pass")
	adminSession := loginUser(t, router, "owner@example.com", "secret-pass")
	memberSession := loginUser(t, router, "member@example.com", "secret-pass")

	for _, grant := range []struct {
		userID string
		role   string
	}{
		{owner.ID, CompanyRoleOwner},
		{member.ID, CompanyRoleMember},
	} {
		body := fmt.Sprintf(`{"user_id":%q,"company_id":"c1","role":%q}`, grant.userID, grant.role)
		rec := performWithToken(router, http.MethodPost, "/api/v1/access/grant", body, adminSession.Token)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("grant %s status %d", grant.role, rec.Code)
		}
	}

	rec := performWithToken(router, http.MethodPost, "/api/v1/companies/c1/members",
		fmt.Sprintf(`{"email":%q}`, outsider.Email), memberSession.Token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("member add status %d, want %d", rec.Code, http.StatusForbidden)
	}

	outsiderSession := loginUser(t, router, "outsider@example.com", "secret-pass")
	rec = performWithToken(router, http.MethodGet, "/api/v1/companies/c1/members", "", outsiderSession.Token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("outsider read status %d, want %d", rec.Code, http.StatusNotFound)
	}

	rec = performWithToken(router, http.MethodPost, "/api/v1/companies/c1/members",
		fmt.Sprintf(`{"email":%q}`, outsider.Email), adminSession.Token)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin add status %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRemoveMemberClearsPrimary(t *testing.T) {
	router := newTestRouter(NewMemStore())
	registerUser(t, router, "owner@example.com", "secret-pass")
	owner := registerUser(t, router, "analyst@example.com", "secret-pass")
	member := registerUser(t, router, "member@example.com", "secret-pass")
	adminSession := loginUser(t, router, "owner@example.com", "secret-pass")
	ownerSession := loginUser(t, router, "analyst@example.com", "secret-pass")
	memberSession := loginUser(t, router, "member@example.com", "secret-pass")

	for _, grant := range []struct {
		userID string
		role   string
	}{
		{owner.ID, CompanyRoleOwner},
		{member.ID, CompanyRoleMember},
	} {
		body := fmt.Sprintf(`{"user_id":%q,"company_id":"c1","role":%q}`, grant.userID, grant.role)
		rec := performWithToken(router, http.MethodPost, "/api/v1/access/grant", body, adminSession.Token)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("grant status %d", rec.Code)
		}
	}

	rec := performWithToken(router, http.MethodPatch, "/api/v1/me", `{"primary_company_id":"c1"}`, memberSession.Token)
	if rec.Code != http.StatusOK {
		t.Fatalf("profile status %d", rec.Code)
	}

	rec = performWithToken(router, http.MethodDelete, "/api/v1/companies/c1/members/"+member.ID, "", ownerSession.Token)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("remove status %d", rec.Code)
	}

	rec = performWithToken(router, http.MethodGet, "/api/v1/me", "", memberSession.Token)
	var me User
	if err := json.Unmarshal(rec.Body.Bytes(), &me); err != nil {
		t.Fatalf("decode me: %v", err)
	}
	if me.PrimaryCompanyID != "" {
		t.Fatalf("primary company = %q, want empty", me.PrimaryCompanyID)
	}
}

func TestInternalMembership(t *testing.T) {
	router := newTestRouter(NewMemStore())
	registerUser(t, router, "owner@example.com", "secret-pass")
	user := registerUser(t, router, "analyst@example.com", "secret-pass")

	rec := perform(router, http.MethodPost, "/api/v1/internal/memberships",
		fmt.Sprintf(`{"user_id":%q,"company_id":"c1","role":"owner"}`, user.ID))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d, want %d body %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	session := loginUser(t, router, "analyst@example.com", "secret-pass")
	rec = performWithToken(router, http.MethodGet, "/api/v1/me/memberships", "", session.Token)
	var memberships struct {
		Items []Membership `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &memberships); err != nil {
		t.Fatalf("decode memberships: %v", err)
	}
	if len(memberships.Items) != 1 || memberships.Items[0].Role != CompanyRoleOwner {
		t.Fatalf("memberships = %+v, want c1 owner", memberships.Items)
	}

	rec = perform(router, http.MethodPost, "/api/v1/internal/memberships",
		fmt.Sprintf(`{"user_id":%q,"company_id":"c1","role":"bad"}`, user.ID))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid role status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRevokeClearsPrimary(t *testing.T) {
	router := newTestRouter(NewMemStore())
	registerUser(t, router, "owner@example.com", "secret-pass")
	user := registerUser(t, router, "analyst@example.com", "secret-pass")
	adminSession := loginUser(t, router, "owner@example.com", "secret-pass")
	userSession := loginUser(t, router, "analyst@example.com", "secret-pass")

	rec := performWithToken(router, http.MethodPost, "/api/v1/access/grant",
		fmt.Sprintf(`{"user_id":%q,"company_id":"c1","role":"owner"}`, user.ID), adminSession.Token)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("grant status %d", rec.Code)
	}
	rec = performWithToken(router, http.MethodPatch, "/api/v1/me", `{"primary_company_id":"c1"}`, userSession.Token)
	if rec.Code != http.StatusOK {
		t.Fatalf("profile status %d", rec.Code)
	}

	rec = performWithToken(router, http.MethodPost, "/api/v1/access/revoke",
		fmt.Sprintf(`{"user_id":%q,"company_id":"c1"}`, user.ID), adminSession.Token)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("revoke status %d", rec.Code)
	}

	rec = performWithToken(router, http.MethodGet, "/api/v1/me", "", userSession.Token)
	var me User
	if err := json.Unmarshal(rec.Body.Bytes(), &me); err != nil {
		t.Fatalf("decode me: %v", err)
	}
	if me.PrimaryCompanyID != "" {
		t.Fatalf("primary company = %q, want empty", me.PrimaryCompanyID)
	}
}

func TestPruneSessionsRemovesExpired(t *testing.T) {
	t.Setenv("IDENTITY_SESSION_TTL", "1ms")
	store := NewMemStore()
	router := newTestRouter(store)
	registerUser(t, router, "user@example.com", "secret-pass")
	loginUser(t, router, "user@example.com", "secret-pass")

	time.Sleep(10 * time.Millisecond)

	removed, err := store.PruneSessions(context.Background())
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
}
