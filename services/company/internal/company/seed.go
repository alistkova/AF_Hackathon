package company

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

func ParseCSV(r io.Reader) ([]Company, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err == io.EOF {
		return []Company{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	columns := make(map[string]int, len(header))
	for i, name := range header {
		name = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, "\ufeff")))
		if name != "" {
			columns[name] = i
		}
	}
	for _, required := range []string{"id", "name"} {
		if _, ok := columns[required]; !ok {
			return nil, fmt.Errorf("missing required column %q", required)
		}
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

		value := func(column string) string {
			index, ok := columns[column]
			if !ok || index >= len(record) {
				return ""
			}
			return strings.TrimSpace(record[index])
		}

		item := Company{
			ID:          value("id"),
			Name:        value("name"),
			INN:         value("inn"),
			Industry:    value("industry"),
			Region:      value("region"),
			Description: value("description"),
		}
		if item.ID == "" || item.Name == "" {
			return nil, fmt.Errorf("row %d: id and name are required", row)
		}
		items = append(items, item)
	}

	return items, nil
}
