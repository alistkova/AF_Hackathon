package identity

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alfa-hack/alfa-hack/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

type Store interface {
	Register(ctx context.Context, email, password string) (User, error)
	Authenticate(ctx context.Context, email, password string) (Session, error)
	Logout(ctx context.Context, token string) error
	Me(ctx context.Context, token string) (User, error)
	UpdateProfile(ctx context.Context, userID string, input ProfileUpdate) (User, error)
	ChangePassword(ctx context.Context, userID, token, current, next string) error
	AccessibleCompanies(ctx context.Context, userID string) ([]string, error)
	Memberships(ctx context.Context, userID string) ([]Membership, error)
	Members(ctx context.Context, companyID string) ([]Member, error)
	MemberRole(ctx context.Context, companyID, userID string) (string, error)
	AddMember(ctx context.Context, companyID, email string) (Member, error)
	UpdateMemberRole(ctx context.Context, companyID, userID, role string) error
	RemoveMember(ctx context.Context, companyID, userID string) error
	GrantAccess(ctx context.Context, userID, companyID, role string) error
	RevokeAccess(ctx context.Context, userID, companyID string) error
	PruneSessions(ctx context.Context) (int64, error)
}

type User struct {
	ID               string `json:"id"`
	Email            string `json:"email"`
	Role             string `json:"role"`
	PrimaryCompanyID string `json:"primary_company_id"`
}

type ProfileUpdate struct {
	Email            *string
	PrimaryCompanyID *string
}

type Membership struct {
	CompanyID string `json:"company_id"`
	Role      string `json:"role"`
}

type Member struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
}

type Session struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      User      `json:"user"`
}

type Handler struct {
	store     Store
	limiter   *loginLimiter
	passwords *loginLimiter
	logger    *slog.Logger
}

func NewHandler(store Store, logger *slog.Logger) *Handler {
	return &Handler{
		store:     store,
		limiter:   newLoginLimiter(),
		passwords: newLoginLimiter(),
		logger:    logger,
	}
}

const maxRequestBody = 64 << 10

func limitBody(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBody)
}

type credentialsRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *Handler) Register(c *gin.Context) {
	limitBody(c)
	if !registrationEnabled() {
		httpx.Error(c, http.StatusForbidden, "registration_disabled", "Регистрация отключена")
		return
	}

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
	limitBody(c)
	var req credentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Укажите email и пароль")
		return
	}

	key := normalizeEmail(req.Email)
	if ok, wait := h.limiter.allow(key); !ok {
		seconds := int(wait.Seconds())
		if seconds < 1 {
			seconds = 1
		}
		c.Header("Retry-After", strconv.Itoa(seconds))
		httpx.Error(c, http.StatusTooManyRequests, "too_many_attempts", "Слишком много попыток входа, попробуйте позже")
		return
	}

	session, err := h.store.Authenticate(c.Request.Context(), req.Email, req.Password)
	if errors.Is(err, ErrInvalidCredentials) {
		h.limiter.fail(key)
		httpx.Error(c, http.StatusUnauthorized, "invalid_credentials", "Неверный email или пароль")
		return
	}
	if err != nil {
		h.internal(c, "authenticate user", err)
		return
	}

	h.limiter.reset(key)
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

func (h *Handler) UpdateMe(c *gin.Context) {
	limitBody(c)
	var req struct {
		Email            *string `json:"email"`
		PrimaryCompanyID *string `json:"primary_company_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Некорректный запрос")
		return
	}
	if req.Email == nil && req.PrimaryCompanyID == nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Нет данных для обновления")
		return
	}

	userID := c.GetString("userID")
	input := ProfileUpdate{}
	if req.Email != nil {
		email, err := validateEmail(*req.Email)
		if err != nil {
			httpx.Error(c, http.StatusBadRequest, "invalid_email", "Укажите корректный email")
			return
		}
		input.Email = &email
	}
	if req.PrimaryCompanyID != nil {
		companyID := strings.TrimSpace(*req.PrimaryCompanyID)
		if len(companyID) > maxCompanyIDBytes {
			httpx.Error(c, http.StatusBadRequest, "invalid_company", "Некорректный идентификатор компании")
			return
		}
		if companyID != "" {
			if _, err := h.store.MemberRole(c.Request.Context(), companyID, userID); errors.Is(err, ErrMemberNotFound) {
				httpx.Error(c, http.StatusBadRequest, "invalid_company", "Вы не являетесь участником компании")
				return
			} else if err != nil {
				h.internal(c, "resolve primary company", err)
				return
			}
		}
		input.PrimaryCompanyID = &companyID
	}

	user, err := h.store.UpdateProfile(c.Request.Context(), userID, input)
	switch {
	case errors.Is(err, ErrEmailExists):
		httpx.Error(c, http.StatusConflict, "email_exists", "Пользователь с таким email уже существует")
		return
	case errors.Is(err, ErrInvalidInput):
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Некорректный запрос")
		return
	case err != nil:
		h.internal(c, "update profile", err)
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *Handler) ChangePassword(c *gin.Context) {
	limitBody(c)
	var req struct {
		CurrentPassword string `json:"current_password" binding:"required"`
		NewPassword     string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Укажите текущий и новый пароль")
		return
	}
	if err := validatePassword(req.NewPassword); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_password", "Пароль должен быть не короче 8 символов")
		return
	}

	userID := c.GetString("userID")
	if ok, wait := h.passwords.allow(userID); !ok {
		seconds := int(wait.Seconds())
		if seconds < 1 {
			seconds = 1
		}
		c.Header("Retry-After", strconv.Itoa(seconds))
		httpx.Error(c, http.StatusTooManyRequests, "too_many_attempts", "Слишком много попыток смены пароля, попробуйте позже")
		return
	}

	err := h.store.ChangePassword(c.Request.Context(), userID, c.GetString("token"), req.CurrentPassword, req.NewPassword)
	if errors.Is(err, ErrInvalidCredentials) {
		h.passwords.fail(userID)
		httpx.Error(c, http.StatusBadRequest, "invalid_current_password", "Текущий пароль неверен")
		return
	}
	if err != nil {
		h.internal(c, "change password", err)
		return
	}
	h.passwords.reset(userID)
	c.Status(http.StatusNoContent)
}

func (h *Handler) Memberships(c *gin.Context) {
	items, err := h.store.Memberships(c.Request.Context(), c.GetString("userID"))
	if err != nil {
		h.internal(c, "list memberships", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) CompanyMembers(c *gin.Context) {
	companyID := strings.TrimSpace(c.Param("companyId"))
	if _, ok := h.requireCompanyRole(c, companyID, ""); !ok {
		return
	}

	items, err := h.store.Members(c.Request.Context(), companyID)
	if err != nil {
		h.internal(c, "list company members", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) AddCompanyMember(c *gin.Context) {
	limitBody(c)
	companyID := strings.TrimSpace(c.Param("companyId"))
	if _, ok := h.requireCompanyRole(c, companyID, CompanyRoleOwner); !ok {
		return
	}

	var req struct {
		Email string `json:"email" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Укажите email пользователя")
		return
	}

	member, err := h.store.AddMember(c.Request.Context(), companyID, req.Email)
	switch {
	case errors.Is(err, ErrUserNotFound):
		httpx.Error(c, http.StatusNotFound, "user_not_found", "Пользователь с таким email не найден")
		return
	case errors.Is(err, ErrInvalidInput):
		httpx.Error(c, http.StatusBadRequest, "invalid_email", "Укажите корректный email")
		return
	case err != nil:
		h.internal(c, "add company member", err)
		return
	}
	c.JSON(http.StatusOK, member)
}

func (h *Handler) UpdateCompanyMember(c *gin.Context) {
	limitBody(c)
	companyID := strings.TrimSpace(c.Param("companyId"))
	if _, ok := h.requireCompanyRole(c, companyID, CompanyRoleOwner); !ok {
		return
	}

	var req struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Укажите роль участника")
		return
	}
	if !validCompanyRole(req.Role) {
		httpx.Error(c, http.StatusBadRequest, "invalid_role", "Некорректная роль")
		return
	}

	err := h.store.UpdateMemberRole(c.Request.Context(), companyID, strings.TrimSpace(c.Param("userId")), req.Role)
	switch {
	case errors.Is(err, ErrMemberNotFound):
		httpx.Error(c, http.StatusNotFound, "member_not_found", "Участник не найден")
		return
	case errors.Is(err, ErrLastOwner):
		httpx.Error(c, http.StatusConflict, "last_owner", "Нельзя понизить единственного владельца компании")
		return
	case err != nil:
		h.internal(c, "update company member", err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) RemoveCompanyMember(c *gin.Context) {
	companyID := strings.TrimSpace(c.Param("companyId"))
	if _, ok := h.requireCompanyRole(c, companyID, CompanyRoleOwner); !ok {
		return
	}

	err := h.store.RemoveMember(c.Request.Context(), companyID, strings.TrimSpace(c.Param("userId")))
	switch {
	case errors.Is(err, ErrMemberNotFound):
		httpx.Error(c, http.StatusNotFound, "member_not_found", "Участник не найден")
		return
	case errors.Is(err, ErrLastOwner):
		httpx.Error(c, http.StatusConflict, "last_owner", "Нельзя удалить единственного владельца компании")
		return
	case err != nil:
		h.internal(c, "remove company member", err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) CreateMembership(c *gin.Context) {
	limitBody(c)
	var req struct {
		UserID    string `json:"user_id" binding:"required"`
		CompanyID string `json:"company_id" binding:"required"`
		Role      string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Укажите user_id и company_id")
		return
	}
	if req.Role == "" {
		req.Role = CompanyRoleMember
	}
	if !validCompanyRole(req.Role) {
		httpx.Error(c, http.StatusBadRequest, "invalid_role", "Некорректная роль")
		return
	}

	err := h.store.GrantAccess(c.Request.Context(), req.UserID, req.CompanyID, req.Role)
	if errors.Is(err, ErrUserNotFound) {
		httpx.Error(c, http.StatusNotFound, "user_not_found", "Пользователь не найден")
		return
	}
	if errors.Is(err, ErrLastOwner) {
		httpx.Error(c, http.StatusConflict, "last_owner", "Нельзя понизить единственного владельца компании")
		return
	}
	if err != nil {
		h.internal(c, "create membership", err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) requireCompanyRole(c *gin.Context, companyID, required string) (string, bool) {
	if companyID == "" || len(companyID) > maxCompanyIDBytes {
		httpx.Error(c, http.StatusNotFound, "company_not_found", "Компания не найдена")
		return "", false
	}

	role, err := h.store.MemberRole(c.Request.Context(), companyID, c.GetString("userID"))
	if errors.Is(err, ErrMemberNotFound) {
		if c.GetString("role") == RoleAdmin {
			return RoleAdmin, true
		}
		httpx.Error(c, http.StatusNotFound, "company_not_found", "Компания не найдена")
		return "", false
	}
	if err != nil {
		h.internal(c, "resolve company role", err)
		return "", false
	}
	if required != "" && role != required && c.GetString("role") != RoleAdmin {
		httpx.Error(c, http.StatusForbidden, "forbidden", "Недостаточно прав")
		return "", false
	}
	return role, true
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
	limitBody(c)
	var req struct {
		UserID    string `json:"user_id" binding:"required"`
		CompanyID string `json:"company_id" binding:"required"`
		Role      string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Укажите user_id и company_id")
		return
	}
	if req.Role == "" {
		req.Role = CompanyRoleMember
	}
	if !validCompanyRole(req.Role) {
		httpx.Error(c, http.StatusBadRequest, "invalid_role", "Некорректная роль")
		return
	}

	err := h.store.GrantAccess(c.Request.Context(), req.UserID, req.CompanyID, req.Role)
	if errors.Is(err, ErrUserNotFound) {
		httpx.Error(c, http.StatusNotFound, "user_not_found", "Пользователь не найден")
		return
	}
	if errors.Is(err, ErrLastOwner) {
		httpx.Error(c, http.StatusConflict, "last_owner", "Нельзя понизить единственного владельца компании")
		return
	}
	if err != nil {
		h.internal(c, "grant company access", err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) RevokeAccess(c *gin.Context) {
	limitBody(c)
	var req struct {
		UserID    string `json:"user_id" binding:"required"`
		CompanyID string `json:"company_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Укажите user_id и company_id")
		return
	}

	if err := h.store.RevokeAccess(c.Request.Context(), req.UserID, req.CompanyID); err != nil {
		if errors.Is(err, ErrLastOwner) {
			httpx.Error(c, http.StatusConflict, "last_owner", "Нельзя удалить единственного владельца компании")
			return
		}
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
		ID:               c.GetString("userID"),
		Email:            c.GetString("userEmail"),
		Role:             c.GetString("role"),
		PrimaryCompanyID: c.GetString("primaryCompanyID"),
	}
}
