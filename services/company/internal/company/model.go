package company

import "time"

type Company struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	INN         string            `json:"inn"`
	Industry    string            `json:"industry"`
	Region      string            `json:"region"`
	Description string            `json:"description"`
	GroupID     string            `json:"group_id,omitempty"`
	OKVEDCode   string            `json:"okved_code,omitempty"`
	OKVEDName   string            `json:"okved_name,omitempty"`
	ClientType  string            `json:"client_type,omitempty"`
	Revenue     string            `json:"revenue,omitempty"`
	City        string            `json:"city,omitempty"`
	Products    map[string]string `json:"products,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

type Person struct {
	ID        string    `json:"id"`
	FullName  string    `json:"full_name"`
	Role      string    `json:"role"`
	City      string    `json:"city,omitempty"`
	Region    string    `json:"region,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type CompanyPerson struct {
	Person   Person `json:"person"`
	Relation string `json:"relation"`
}

type RelationFlow struct {
	Amount float64 `json:"amount"`
	Count  int64   `json:"count"`
}

type CompanyRelation struct {
	CompanyID    string        `json:"company_id"`
	CompanyName  string        `json:"company_name"`
	Type         string        `json:"type"`
	Types        []string      `json:"types"`
	Relation     string        `json:"relation"`
	Direction    string        `json:"direction,omitempty"`
	Amount       float64       `json:"amount,omitempty"`
	Count        int64         `json:"count,omitempty"`
	Inbound      *RelationFlow `json:"inbound,omitempty"`
	Outbound     *RelationFlow `json:"outbound,omitempty"`
	SharedPeople int64         `json:"shared_people,omitempty"`
}

type CompanyDetails struct {
	Company
	People    []CompanyPerson   `json:"people"`
	Relations []CompanyRelation `json:"relations"`
}
