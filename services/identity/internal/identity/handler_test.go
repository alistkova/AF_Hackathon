package identity

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func newTestRouter(store Store) *gin.Engine {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(store)
	router := gin.New()
	router.POST("/api/v1/auth/register", handler.Register)
	router.POST("/api/v1/auth/login", handler.Login)
	router.GET("/api/v1/me", handler.Me)
	return router
}

func TestRegister(t *testing.T) {
	store := NewMemStore()
	router := newTestRouter(store)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", nil)
	req.Header.Set("Content-Type", "application/json")
	// body
	b := []byte(`{"email":"a@b.com","password":"x"}`)
	req.Body = io.NopCloser(bytes.NewReader(b))
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d", rec.Code)
	}
	var u User
	json.Unmarshal(rec.Body.Bytes(), &u)
	if u.Email != "a@b.com" {
		t.Fatalf("email %s", u.Email)
	}

}
