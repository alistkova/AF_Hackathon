package company

import (
	"reflect"
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
	if !reflect.DeepEqual(items[0], want) {
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

func TestParseHackathonCompaniesCSV(t *testing.T) {
	input := "SYNTHETIC_U,MAINOKVED_CODE,MAINOKVED_NAME,REGION,CITY,GROUP_ID,REVENUE_COMPANY,CL_TYPE,CL_PRODUCT_LOA_MARK,CL_PRODUCT_RKO_MARK,ORG_TYPE\n" +
		"U-1,46.90,Торговля,Москва,Москва,G-1,\"1 000 000\",Действующий,1,0,ООО\n"

	items, err := ParseCSV(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	item := items[0]
	if item.ID != "U-1" || item.Name != "U-1" {
		t.Fatalf("unexpected id/name: %+v", item)
	}
	if item.Industry != "Торговля" || item.OKVEDCode != "46.90" || item.GroupID != "G-1" {
		t.Fatalf("unexpected profile: %+v", item)
	}
	if item.Revenue != "1 000 000" || item.ClientType != "Действующий" || item.City != "Москва" {
		t.Fatalf("unexpected attributes: %+v", item)
	}
	if item.Products["loans"] != "1" || item.Products["rko"] != "0" {
		t.Fatalf("unexpected products: %+v", item.Products)
	}
	if item.Attributes["org_type"] != "ООО" {
		t.Fatalf("unexpected attributes: %+v", item.Attributes)
	}
}

func TestParsePeopleCSV(t *testing.T) {
	input := "SYNTHETIC_F,SYNTHETIC_U,CITY_NAME,RC_NAME\n" +
		"F-1,U-1,Москва,Московская область\n"

	links, err := ParsePeopleCSV(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParsePeopleCSV: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("len(links) = %d, want 1", len(links))
	}
	link := links[0]
	if link.Person.ID != "F-1" || link.Person.FullName != "F-1" || link.CompanyID != "U-1" {
		t.Fatalf("unexpected link: %+v", link)
	}
	if link.Person.City != "Москва" || link.Person.Region != "Московская область" || link.Relation != "linked_person" {
		t.Fatalf("unexpected person: %+v", link)
	}
}

func TestParsePaymentsCSV(t *testing.T) {
	input := "PAYER,BENEFICIARY,PAY_AMT,PAY_CNT\n" +
		"U-1,U-2,\"1 250 000,50\",12\n"

	payments, err := ParsePaymentsCSV(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParsePaymentsCSV: %v", err)
	}
	if len(payments) != 1 {
		t.Fatalf("len(payments) = %d, want 1", len(payments))
	}
	payment := payments[0]
	if payment.PayerID != "U-1" || payment.BeneficiaryID != "U-2" {
		t.Fatalf("unexpected payment: %+v", payment)
	}
	if payment.Amount != 1250000.50 || payment.Count != 12 {
		t.Fatalf("unexpected amount/count: %+v", payment)
	}
}

func TestParsePaymentsCSVInvalidAmount(t *testing.T) {
	input := "PAYER,BENEFICIARY,PAY_AMT\nU-1,U-2,not-a-number\n"

	if _, err := ParsePaymentsCSV(strings.NewReader(input)); err == nil {
		t.Fatal("ParsePaymentsCSV returned nil error, want invalid amount error")
	}
}

func TestParsePeopleCSVMissingCompany(t *testing.T) {
	input := "SYNTHETIC_F,CITY_NAME\nF-1,Москва\n"

	if _, err := ParsePeopleCSV(strings.NewReader(input)); err == nil {
		t.Fatal("ParsePeopleCSV returned nil error, want missing company column error")
	}
}
