package company

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type PersonLink struct {
	Person    Person
	CompanyID string
	Relation  string
}

type Payment struct {
	PayerID       string
	BeneficiaryID string
	Amount        float64
	Count         int64
}

var productColumns = map[string]string{
	"cl_product_rko_mark": "rko",
	"cl_product_gar_mark": "guarantees",
	"cl_product_loa_mark": "loans",
	"cl_product_lea_mark": "leasing",
	"cl_product_dep_mark": "deposits",
	"cl_product_zpp_mark": "payroll",
	"factoring_mark":      "factoring",
}

var attributeColumns = map[string]string{
	"org_type":                       "org_type",
	"okfs_name":                      "okfs_name",
	"company_age_years_gradation":    "company_age_years",
	"eco_mark_label":                 "eco_mark",
	"negativemark":                   "negative_mark",
	"amount_workers_fns_gradation":   "workers_fns",
	"amount_workers_spark_gradation": "workers_spark",
	"fts_flag":                       "fts_flag",
	"fts_country_name":               "fts_country",
}

func ParseCSV(r io.Reader) ([]Company, error) {
	reader, columns, err := openCSV(r)
	if err != nil {
		return nil, err
	}
	if len(columns) == 0 {
		return []Company{}, nil
	}

	idColumn := firstColumn(columns, "id", "synthetic_u")
	if idColumn == "" {
		return nil, fmt.Errorf("missing required column %q", "id")
	}
	hasName := hasColumn(columns, "name")
	dataset := !hasName && hasColumn(columns, "synthetic_u")
	if !hasName && !dataset {
		return nil, fmt.Errorf("missing required column %q", "name")
	}

	items := make([]Company, 0)
	for row := 2; ; row++ {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row %d: %w", row, err)
		}

		id := value(columns, record, idColumn)
		name := value(columns, record, "name")
		if name == "" {
			name = id
		}
		industry := value(columns, record, "industry")
		if industry == "" {
			industry = value(columns, record, "mainokved_name")
		}
		if id == "" || name == "" {
			return nil, fmt.Errorf("row %d: id and name are required", row)
		}

		item := Company{
			ID:          id,
			Name:        name,
			INN:         value(columns, record, "inn"),
			Industry:    industry,
			Region:      value(columns, record, "region"),
			Description: value(columns, record, "description"),
			GroupID:     value(columns, record, "group_id"),
			OKVEDCode:   value(columns, record, "mainokved_code"),
			OKVEDName:   value(columns, record, "mainokved_name"),
			ClientType:  value(columns, record, "cl_type"),
			Revenue:     value(columns, record, "revenue_company", "revenue"),
			City:        value(columns, record, "city"),
			Products:    collect(columns, record, productColumns),
			Attributes:  collect(columns, record, attributeColumns),
		}
		items = append(items, item)
	}

	return items, nil
}

func ParsePeopleCSV(r io.Reader) ([]PersonLink, error) {
	reader, columns, err := openCSV(r)
	if err != nil {
		return nil, err
	}

	personColumn := firstColumn(columns, "synthetic_f", "person_id", "id")
	if personColumn == "" {
		return nil, fmt.Errorf("missing required column %q", "synthetic_f")
	}
	companyColumn := firstColumn(columns, "synthetic_u", "company_id")
	if companyColumn == "" {
		return nil, fmt.Errorf("missing required column %q", "synthetic_u")
	}

	items := make([]PersonLink, 0)
	for row := 2; ; row++ {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row %d: %w", row, err)
		}

		personID := value(columns, record, personColumn)
		companyID := value(columns, record, companyColumn)
		if personID == "" || companyID == "" {
			return nil, fmt.Errorf("row %d: person and company ids are required", row)
		}

		fullName := value(columns, record, "full_name")
		if fullName == "" {
			fullName = personID
		}
		relation := value(columns, record, "relation")
		if relation == "" {
			relation = "linked_person"
		}

		items = append(items, PersonLink{
			Person: Person{
				ID:       personID,
				FullName: fullName,
				Role:     value(columns, record, "role"),
				City:     value(columns, record, "city_name", "city"),
				Region:   value(columns, record, "rc_name", "region"),
			},
			CompanyID: companyID,
			Relation:  relation,
		})
	}

	return items, nil
}

func ParsePaymentsCSV(r io.Reader) ([]Payment, error) {
	reader, columns, err := openCSV(r)
	if err != nil {
		return nil, err
	}

	payerColumn := firstColumn(columns, "payer", "payer_id")
	if payerColumn == "" {
		return nil, fmt.Errorf("missing required column %q", "payer")
	}
	beneficiaryColumn := firstColumn(columns, "beneficiary", "beneficiary_id")
	if beneficiaryColumn == "" {
		return nil, fmt.Errorf("missing required column %q", "beneficiary")
	}

	items := make([]Payment, 0)
	for row := 2; ; row++ {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row %d: %w", row, err)
		}

		payer := value(columns, record, payerColumn)
		beneficiary := value(columns, record, beneficiaryColumn)
		if payer == "" || beneficiary == "" {
			return nil, fmt.Errorf("row %d: payer and beneficiary are required", row)
		}

		amount, err := parseAmount(value(columns, record, "pay_amt", "amount"))
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", row, err)
		}
		count, err := parseCount(value(columns, record, "pay_cnt", "count", "payment_count"))
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", row, err)
		}

		items = append(items, Payment{
			PayerID:       payer,
			BeneficiaryID: beneficiary,
			Amount:        amount,
			Count:         count,
		})
	}

	return items, nil
}

func openCSV(r io.Reader) (*csv.Reader, map[string]int, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err == io.EOF {
		return reader, map[string]int{}, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read header: %w", err)
	}

	columns := make(map[string]int, len(header))
	for i, name := range header {
		name = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, "\ufeff")))
		if name != "" {
			columns[name] = i
		}
	}
	return reader, columns, nil
}

func hasColumn(columns map[string]int, name string) bool {
	_, ok := columns[name]
	return ok
}

func firstColumn(columns map[string]int, names ...string) string {
	for _, name := range names {
		if hasColumn(columns, name) {
			return name
		}
	}
	return ""
}

func value(columns map[string]int, record []string, names ...string) string {
	for _, name := range names {
		index, ok := columns[name]
		if !ok || index >= len(record) {
			continue
		}
		if v := strings.TrimSpace(record[index]); v != "" {
			return v
		}
	}
	return ""
}

func collect(columns map[string]int, record []string, mapping map[string]string) map[string]string {
	values := make(map[string]string)
	for column, key := range mapping {
		if v := value(columns, record, column); v != "" {
			values[key] = v
		}
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func parseAmount(raw string) (float64, error) {
	clean := normalizeNumber(raw)
	if clean == "" {
		return 0, nil
	}
	amount, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q", raw)
	}
	return amount, nil
}

func parseCount(raw string) (int64, error) {
	clean := normalizeNumber(raw)
	if clean == "" {
		return 0, nil
	}
	count, err := strconv.ParseInt(clean, 10, 64)
	if err == nil {
		return count, nil
	}
	value, floatErr := strconv.ParseFloat(clean, 64)
	if floatErr != nil {
		return 0, fmt.Errorf("invalid count %q", raw)
	}
	return int64(value), nil
}

func normalizeNumber(raw string) string {
	clean := strings.TrimSpace(raw)
	clean = strings.ReplaceAll(clean, "\u00a0", "")
	clean = strings.ReplaceAll(clean, " ", "")
	clean = strings.ReplaceAll(clean, ",", ".")
	return clean
}
