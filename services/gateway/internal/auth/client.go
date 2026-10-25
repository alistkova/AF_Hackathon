package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const RoleAdmin = "admin"
const RoleOwner = "owner"

var ErrUnauthorized = errors.New("unauthorized")
var ErrRoleNotFound = errors.New("membership not found")

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type Client struct {
	baseURL       string
	internalToken string
	http          *http.Client
}

func NewClient(baseURL, internalToken string) *Client {
	return &Client{
		baseURL:       strings.TrimRight(baseURL, "/"),
		internalToken: internalToken,
		http:          &http.Client{Timeout: 3 * time.Second},
	}
}

func (c *Client) Me(ctx context.Context, token string) (User, error) {
	var user User
	if err := c.get(ctx, "/api/v1/me", token, &user); err != nil {
		return User{}, err
	}
	return user, nil
}

func (c *Client) Companies(ctx context.Context, token string) ([]string, error) {
	var body struct {
		Items []string `json:"items"`
	}
	if err := c.get(ctx, "/api/v1/me/companies", token, &body); err != nil {
		return nil, err
	}
	if body.Items == nil {
		return []string{}, nil
	}
	return body.Items, nil
}

func (c *Client) Role(ctx context.Context, token, companyID string) (string, error) {
	var body struct {
		Items []struct {
			CompanyID string `json:"company_id"`
			Role      string `json:"role"`
		} `json:"items"`
	}
	if err := c.get(ctx, "/api/v1/me/memberships", token, &body); err != nil {
		return "", err
	}
	for _, item := range body.Items {
		if item.CompanyID == companyID {
			return item.Role, nil
		}
	}
	return "", ErrRoleNotFound
}

func (c *Client) AddMembership(ctx context.Context, userID, companyID, role string) error {
	payload, err := json.Marshal(map[string]string{
		"user_id":    userID,
		"company_id": companyID,
		"role":       role,
	})
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/internal/memberships", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if c.internalToken != "" {
		request.Header.Set("X-Internal-Token", c.internalToken)
	}

	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("call identity membership: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("identity membership returned %d", response.StatusCode)
	}
	return nil
}

func (c *Client) get(ctx context.Context, path, token string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")

	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("call identity %s: %w", path, err)
	}
	defer func() { _ = response.Body.Close() }()

	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return ErrUnauthorized
	default:
		return fmt.Errorf("identity %s returned %d", path, response.StatusCode)
	}

	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("decode identity %s: %w", path, err)
	}
	return nil
}
