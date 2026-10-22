package company

import (
	"strings"
	"testing"
)

func TestParseCSV(t *testing.T) {
	input := "name,id,region,inn,industry,description,extra\n" +
		"Магнит,magnit,Краснодар,5063471190,Ритейл,\"Розничная сеть, магазины у дома\",ignored\n"

	items, err := ParseCSV(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	want := Company{
		ID:          "magnit",
		Name:        "Магнит",
		INN:         "5063471190",
		Industry:    "Ритейл",
		Region:      "Краснодар",
		Description: "Розничная сеть, магазины у дома",
	}
	if items[0] != want {
		t.Fatalf("company = %+v, want %+v", items[0], want)
	}
}

func TestParseCSVBOMAndCRLF(t *testing.T) {
	input := "\ufeffid,name\r\nmagnit,Магнит\r\n"

	items, err := ParseCSV(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if len(items) != 1 || items[0].ID != "magnit" || items[0].Name != "Магнит" {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestParseCSVOptionalColumns(t *testing.T) {
	items, err := ParseCSV(strings.NewReader("id,name\nmagnit,Магнит\n"))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if len(items) != 1 || items[0].INN != "" || items[0].Industry != "" || items[0].Region != "" || items[0].Description != "" {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestParseCSVEmpty(t *testing.T) {
	items, err := ParseCSV(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("len(items) = %d, want 0", len(items))
	}
}

func TestParseCSVMissingRequiredColumn(t *testing.T) {
	if _, err := ParseCSV(strings.NewReader("id,inn\nmagnit,5063471190\n")); err == nil {
		t.Fatal("ParseCSV returned nil error, want missing column error")
	}
}

func TestParseCSVMissingRequiredValue(t *testing.T) {
	if _, err := ParseCSV(strings.NewReader("id,name\n,magnit\n")); err == nil {
		t.Fatal("ParseCSV returned nil error, want missing value error")
	}
}
