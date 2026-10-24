package identity

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

var ErrNotFound = errors.New("not found")

type Store interface {
	Register(string, string) (User, error)
	Authenticate(string, string) (Session, error)
	Logout(string) error
	Me(string) (User, error)
	AccessibleCompanies(string) ([]string, error)
	GrantAccess(string, string) error
	RevokeAccess(string, string) error
}

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type Session struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type Handler struct {
	store Store
}

func NewHandler(store Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) Register(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request"})
		return
	}
	u, err := h.store.Register(req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "register_failed"})
		return
	}
	c.JSON(http.StatusCreated, u)
}

func (h *Handler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request"})
		return
	}
	s, err := h.store.Authenticate(req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "invalid_credentials"})
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h *Handler) Logout(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if err := h.store.Logout(token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "logout_failed"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Me(c *gin.Context) {
	token := c.GetHeader("Authorization")
	u, err := h.store.Me(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized"})
		return
	}
	c.JSON(http.StatusOK, u)
}

func (h *Handler) AccessibleCompanies(c *gin.Context) {
	token := c.GetHeader("Authorization")
	u, err := h.store.Me(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized"})
		return
	}
	ids, err := h.store.AccessibleCompanies(u.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "companies_unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": ids})
}

func (h *Handler) GrantAccess(c *gin.Context) {
	var req struct {
		UserID    string `json:"user_id"`
		CompanyID string `json:"company_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request"})
		return
	}
	if err := h.store.GrantAccess(req.UserID, req.CompanyID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "grant_failed"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) RevokeAccess(c *gin.Context) {
	var req struct {
		UserID    string `json:"user_id"`
		CompanyID string `json:"company_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request"})
		return
	}
	if err := h.store.RevokeAccess(req.UserID, req.CompanyID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "revoke_failed"})
		return
	}
	c.Status(http.StatusNoContent)
}
