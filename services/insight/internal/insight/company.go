package insight

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type CompanyClient struct {
	baseURL string
	client  *http.Client
}

func NewCompanyClient(baseURL string) *CompanyClient {
	return &CompanyClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *CompanyClient) Get(ctx context.Context, id string) (Company, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/companies/"+id, nil)
	if err != nil {
		return Company{}, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return Company{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return Company{}, fmt.Errorf("company service returned %d", resp.StatusCode)
	}

	var item Company
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return Company{}, err
	}
	return item, nil
}
