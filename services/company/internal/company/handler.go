package company

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"

	"github.com/alfa-hack/alfa-hack/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

const maxRequestBytes = 64 << 10

type Store interface {
	List(ctx context.Context, ids []string) ([]Company, error)
	Get(ctx context.Context, id string) (Company, error)
	GetWithPeople(ctx context.Context, id string) (CompanyDetails, error)
	ListPeople(ctx context.Context, companyID string) ([]CompanyPerson, error)
	ListRelations(ctx context.Context, companyID string) ([]CompanyRelation, error)
	Create(ctx context.Context, item Company) (Company, error)
	Update(ctx context.Context, item Company) (Company, error)
	Delete(ctx context.Context, id string) error
}

type Handler struct {
	repo   Store
	logger *slog.Logger
}

func NewHandler(repo Store, logger *slog.Logger) *Handler {
	return &Handler{repo: repo, logger: logger}
}

func (h *Handler) List(c *gin.Context) {
	items, err := h.repo.List(c.Request.Context(), companyFilter(c))
	if err != nil {
		h.logger.Error("list companies", "error", err)
		httpx.Error(c, http.StatusInternalServerError, "companies_unavailable", "Не удалось загрузить компании")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func companyFilter(c *gin.Context) []string {
	raw, ok := c.GetQueryArray("ids")
	if !ok {
		return nil
	}

	seen := make(map[string]struct{}, len(raw))
	ids := make([]string, 0, len(raw))
	for _, value := range raw {
		for _, id := range strings.Split(value, ",") {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, duplicate := seen[id]; duplicate {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	return ids
}

func (h *Handler) Get(c *gin.Context) {
	item, err := h.repo.GetWithPeople(c.Request.Context(), c.Param("id"))
	if IsNotFound(err) {
		httpx.Error(c, http.StatusNotFound, "company_not_found", "Компания не найдена")
		return
	}
	if err != nil {
		h.logger.Error("get company", "error", err, "company_id", c.Param("id"))
		httpx.Error(c, http.StatusInternalServerError, "company_unavailable", "Не удалось загрузить компанию")
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) Create(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBytes)
	var req createCompanyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Некорректный запрос")
		return
	}

	item, problem := validateCompany(req)
	if problem != "" {
		httpx.Error(c, http.StatusBadRequest, "invalid_company", problem)
		return
	}
	id, err := newCompanyID()
	if err != nil {
		h.logger.Error("generate company id", "error", err)
		httpx.Error(c, http.StatusInternalServerError, "company_unavailable", "Не удалось создать компанию")
		return
	}
	item.ID = id

	created, err := h.repo.Create(c.Request.Context(), item)
	if err != nil {
		h.logger.Error("create company", "error", err)
		httpx.Error(c, http.StatusInternalServerError, "company_unavailable", "Не удалось создать компанию")
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *Handler) Update(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBytes)
	var req createCompanyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Некорректный запрос")
		return
	}

	item, problem := validateCompany(req)
	if problem != "" {
		httpx.Error(c, http.StatusBadRequest, "invalid_company", problem)
		return
	}
	item.ID = c.Param("id")

	updated, err := h.repo.Update(c.Request.Context(), item)
	if IsNotFound(err) {
		httpx.Error(c, http.StatusNotFound, "company_not_found", "Компания не найдена")
		return
	}
	if err != nil {
		h.logger.Error("update company", "error", err, "company_id", c.Param("id"))
		httpx.Error(c, http.StatusInternalServerError, "company_unavailable", "Не удалось обновить компанию")
		return
	}
	c.JSON(http.StatusOK, updated)
}

type createCompanyRequest struct {
	Name        string `json:"name"`
	INN         string `json:"inn"`
	Industry    string `json:"industry"`
	Region      string `json:"region"`
	City        string `json:"city"`
	Description string `json:"description"`
	Revenue     string `json:"revenue"`
	ClientType  string `json:"client_type"`
	OKVEDCode   string `json:"okved_code"`
	OKVEDName   string `json:"okved_name"`
}

func validateCompany(req createCompanyRequest) (Company, string) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return Company{}, "Укажите название компании"
	}
	if len(name) > 200 {
		return Company{}, "Название компании слишком длинное"
	}

	inn := strings.TrimSpace(req.INN)
	if inn != "" && !digitsOnly(inn) {
		return Company{}, "ИНН должен состоять только из цифр"
	}
	if inn != "" && len(inn) != 10 && len(inn) != 12 {
		return Company{}, "ИНН должен содержать 10 или 12 цифр"
	}

	revenue := strings.TrimSpace(req.Revenue)
	if revenue != "" && !digitsOnly(revenue) {
		return Company{}, "Выручка должна быть числом"
	}

	return Company{
		Name:        name,
		INN:         inn,
		Industry:    strings.TrimSpace(req.Industry),
		Region:      strings.TrimSpace(req.Region),
		City:        strings.TrimSpace(req.City),
		Description: strings.TrimSpace(req.Description),
		Revenue:     revenue,
		ClientType:  strings.TrimSpace(req.ClientType),
		OKVEDCode:   strings.TrimSpace(req.OKVEDCode),
		OKVEDName:   strings.TrimSpace(req.OKVEDName),
	}, ""
}

func digitsOnly(value string) bool {
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func newCompanyID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "cmp_" + hex.EncodeToString(buf), nil
}

func (h *Handler) Delete(c *gin.Context) {
	err := h.repo.Delete(c.Request.Context(), c.Param("id"))
	if IsNotFound(err) {
		httpx.Error(c, http.StatusNotFound, "company_not_found", "Компания не найдена")
		return
	}
	if err != nil {
		h.logger.Error("delete company", "error", err, "company_id", c.Param("id"))
		httpx.Error(c, http.StatusInternalServerError, "company_unavailable", "Не удалось удалить компанию")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Relations(c *gin.Context) {
	items, err := h.repo.ListRelations(c.Request.Context(), c.Param("id"))
	if IsNotFound(err) {
		httpx.Error(c, http.StatusNotFound, "company_not_found", "Компания не найдена")
		return
	}
	if err != nil {
		h.logger.Error("list company relations", "error", err, "company_id", c.Param("id"))
		httpx.Error(c, http.StatusInternalServerError, "company_unavailable", "Не удалось загрузить компанию")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) People(c *gin.Context) {
	items, err := h.repo.ListPeople(c.Request.Context(), c.Param("id"))
	if IsNotFound(err) {
		httpx.Error(c, http.StatusNotFound, "company_not_found", "Компания не найдена")
		return
	}
	if err != nil {
		h.logger.Error("list company people", "error", err, "company_id", c.Param("id"))
		httpx.Error(c, http.StatusInternalServerError, "company_unavailable", "Не удалось загрузить компанию")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
