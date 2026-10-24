package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var errJobNotFound = errors.New("job not found")

type insightClient struct {
	baseURL string
	http    *http.Client
}

func newInsightClient(baseURL string) *insightClient {
	return &insightClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 3 * time.Second},
	}
}

func (c *insightClient) CompanyID(ctx context.Context, jobID string) (string, error) {
	if strings.TrimSpace(jobID) == "" {
		return "", errJobNotFound
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/insights/jobs/"+url.PathEscape(jobID), nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/json")

	response, err := c.http.Do(request)
	if err != nil {
		return "", fmt.Errorf("call insight job: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return "", errJobNotFound
	default:
		return "", fmt.Errorf("insight job returned %d", response.StatusCode)
	}

	var payload struct {
		CompanyID string `json:"company_id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode insight job: %w", err)
	}
	if strings.TrimSpace(payload.CompanyID) == "" {
		return "", fmt.Errorf("insight job has no company_id")
	}
	return payload.CompanyID, nil
}
