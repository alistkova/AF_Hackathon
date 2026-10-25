package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/alfa-hack/alfa-hack/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

const maxCompanyBody = 1 << 16

type companyClient struct {
	baseURL string
	http    *http.Client
}

func newCompanyClient(baseURL string) *companyClient {
	return &companyClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

type companyWriteError struct {
	Status  int
	Code    string
	Message string
}

func (e *companyWriteError) Error() string {
	return fmt.Sprintf("company request returned %d: %s", e.Status, e.Message)
}

func (c *companyClient) write(ctx context.Context, method, path string, body []byte, expected int) (json.RawMessage, error) {
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call company %s: %w", strings.ToLower(method), err)
	}
	defer func() { _ = response.Body.Close() }()

	payload, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read company %s: %w", strings.ToLower(method), err)
	}
	if response.StatusCode != expected {
		failure := companyWriteError{Status: response.StatusCode, Message: strings.TrimSpace(string(payload))}
		var parsed struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		if json.Unmarshal(payload, &parsed) == nil && parsed.Message != "" {
			failure.Code = parsed.Code
			failure.Message = parsed.Message
		}
		return nil, &failure
	}
	return payload, nil
}

func (c *companyClient) Create(ctx context.Context, body []byte) (json.RawMessage, error) {
	return c.write(ctx, http.MethodPost, "/api/v1/companies", body, http.StatusCreated)
}

func (c *companyClient) Exists(ctx context.Context, id string) (bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/companies/"+url.PathEscape(id), nil)
	if err != nil {
		return false, err
	}
	request.Header.Set("Accept", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return false, fmt.Errorf("call company get: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	switch response.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("company get returned %d", response.StatusCode)
	}
}

func (c *companyClient) Update(ctx context.Context, id string, body []byte) (json.RawMessage, error) {
	return c.write(ctx, http.MethodPut, "/api/v1/companies/"+url.PathEscape(id), body, http.StatusOK)
}

func (c *companyClient) Delete(ctx context.Context, id string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/api/v1/companies/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("call company delete: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusNoContent && response.StatusCode != http.StatusNotFound {
		return fmt.Errorf("company delete returned %d", response.StatusCode)
	}
	return nil
}

func (g *guard) createCompany(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Требуется авторизация")
		return
	}

	body, err := readBody(c, maxCompanyBody)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Некорректный запрос")
		return
	}

	raw, err := g.companies.Create(c.Request.Context(), body)
	if err != nil {
		var failure *companyWriteError
		if errors.As(err, &failure) && failure.Status == http.StatusBadRequest {
			code := failure.Code
			if code == "" {
				code = "invalid_company"
			}
			message := failure.Message
			if message == "" {
				message = "Проверьте данные компании"
			}
			httpx.Error(c, http.StatusBadRequest, code, message)
			return
		}
		g.logger.Error("create company", "error", err, "request_id", c.GetString("request_id"))
		httpx.Error(c, http.StatusBadGateway, "company_unavailable", "Не удалось создать компанию")
		return
	}

	var payload struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil || strings.TrimSpace(payload.ID) == "" {
		g.rollbackCompany(c, payload.ID)
		g.logger.Error("decode created company", "error", err, "request_id", c.GetString("request_id"))
		httpx.Error(c, http.StatusBadGateway, "company_unavailable", "Не удалось создать компанию")
		return
	}

	if err := g.identity.AddMembership(c.Request.Context(), user.ID, payload.ID, "owner"); err != nil {
		g.rollbackCompany(c, payload.ID)
		g.logger.Error("grant company ownership", "error", err, "request_id", c.GetString("request_id"))
		httpx.Error(c, http.StatusBadGateway, "identity_unavailable", "Не удалось создать компанию")
		return
	}

	c.Data(http.StatusCreated, "application/json", raw)
}

func (g *guard) rollbackCompany(c *gin.Context, id string) {
	if strings.TrimSpace(id) == "" {
		return
	}
	if err := g.companies.Delete(c.Request.Context(), id); err != nil {
		g.logger.Error("rollback company", "error", err, "company_id", id, "request_id", c.GetString("request_id"))
	}
}

func (g *guard) updateCompany(c *gin.Context) {
	body, err := readBody(c, maxCompanyBody)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Некорректный запрос")
		return
	}

	raw, err := g.companies.Update(c.Request.Context(), c.Param("id"), body)
	if err != nil {
		var failure *companyWriteError
		if errors.As(err, &failure) && (failure.Status == http.StatusBadRequest || failure.Status == http.StatusNotFound) {
			code := failure.Code
			if code == "" {
				if failure.Status == http.StatusNotFound {
					code = "company_not_found"
				} else {
					code = "invalid_company"
				}
			}
			message := failure.Message
			if message == "" {
				message = "Проверьте данные компании"
			}
			httpx.Error(c, failure.Status, code, message)
			return
		}
		g.logger.Error("update company", "error", err, "request_id", c.GetString("request_id"))
		httpx.Error(c, http.StatusBadGateway, "company_unavailable", "Не удалось обновить компанию")
		return
	}

	c.Data(http.StatusOK, "application/json", raw)
}
