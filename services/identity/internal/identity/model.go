package identity

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/alfa-hack/alfa-hack/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

type Store interface {
	Register(ctx context.Context, email, password string) (User, error)
	Authenticate(ctx context.Context, email, password string) (Session, error)
	Logout(ctx context.Context, token string) error
	Me(ctx context.Context, token string) (User, error)
	AccessibleCompanies(ctx context.Context, userID string) ([]string, error)
	GrantAccess(ctx context.Context, userID, companyID string) error
	RevokeAccess(ctx context.Context, userID, companyID string) error
}

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type Session struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      User      `json:"user"`
}

type Handler struct {
	store  Store
	logger *slog.Logger
}

func NewHandler(store Store, logger *slog.Logger) *Handler {
	return &Handler{store: store, logger: logger}
}

type credentialsRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *Handler) Register(c *gin.Context) {
	var req credentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Укажите email и пароль")
		return
	}

	user, err := h.store.Register(c.Request.Context(), req.Email, req.Password)
	switch {
	case errors.Is(err, ErrInvalidInput):
		httpx.Error(c, http.StatusBadRequest, "invalid_credentials", "Email или пароль не соответствуют требованиям")
		return
	case errors.Is(err, ErrEmailExists):
		httpx.Error(c, http.StatusConflict, "email_exists", "Пользователь с таким email уже существует")
		return
	case err != nil:
		h.internal(c, "register user", err)
		return
	}

	c.JSON(http.StatusCreated, user)
}

func (h *Handler) Login(c *gin.Context) {
	var req credentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Укажите email и пароль")
		return
	}

	session, err := h.store.Authenticate(c.Request.Context(), req.Email, req.Password)
	if errors.Is(err, ErrInvalidCredentials) {
		httpx.Error(c, http.StatusUnauthorized, "invalid_credentials", "Неверный email или пароль")
		return
	}
	if err != nil {
		h.internal(c, "authenticate user", err)
		return
	}

	setSessionCookie(c, session)
	c.JSON(http.StatusOK, session)
}

func (h *Handler) Logout(c *gin.Context) {
	if token := extractToken(c); token != "" {
		if err := h.store.Logout(c.Request.Context(), token); err != nil {
			h.internal(c, "logout user", err)
			return
		}
	}
	clearSessionCookie(c)
	c.Status(http.StatusNoContent)
}

func (h *Handler) Me(c *gin.Context) {
	c.JSON(http.StatusOK, currentUser(c))
}

func (h *Handler) AccessibleCompanies(c *gin.Context) {
	ids, err := h.store.AccessibleCompanies(c.Request.Context(), c.GetString("userID"))
	if err != nil {
		h.internal(c, "list company access", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": ids})
}

func (h *Handler) GrantAccess(c *gin.Context) {
	var req struct {
		UserID    string `json:"user_id" binding:"required"`
		CompanyID string `json:"company_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Укажите user_id и company_id")
		return
	}

	err := h.store.GrantAccess(c.Request.Context(), req.UserID, req.CompanyID)
	if errors.Is(err, ErrUserNotFound) {
		httpx.Error(c, http.StatusNotFound, "user_not_found", "Пользователь не найден")
		return
	}
	if err != nil {
		h.internal(c, "grant company access", err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) RevokeAccess(c *gin.Context) {
	var req struct {
		UserID    string `json:"user_id" binding:"required"`
		CompanyID string `json:"company_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Укажите user_id и company_id")
		return
	}

	if err := h.store.RevokeAccess(c.Request.Context(), req.UserID, req.CompanyID); err != nil {
		h.internal(c, "revoke company access", err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) internal(c *gin.Context, action string, err error) {
	h.logger.Error(action, "error", err, "request_id", c.GetString("request_id"))
	httpx.Error(c, http.StatusInternalServerError, "identity_unavailable", "Сервис идентификации недоступен")
}

func currentUser(c *gin.Context) User {
	return User{
		ID:    c.GetString("userID"),
		Email: c.GetString("userEmail"),
		Role:  c.GetString("role"),
	}
}
