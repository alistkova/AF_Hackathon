package company

import "time"

type Company struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	INN         string    `json:"inn"`
	Industry    string    `json:"industry"`
	Region      string    `json:"region"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}
