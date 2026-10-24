package company

import (
	"reflect"
	"testing"
)

func TestMergeRelationsKeepsSingleEntryPerCompany(t *testing.T) {
	input := []CompanyRelation{
		{CompanyID: "y-corp", CompanyName: "Y Corp", Type: "group", Relation: "same_group"},
		{CompanyID: "y-corp", CompanyName: "Y Corp", Type: "shared_person", Relation: "shared_person", SharedPeople: 2},
		{CompanyID: "y-corp", CompanyName: "Y Corp", Type: "counterparty", Relation: "payment", Direction: "outbound", Amount: 500, Count: 3},
	}

	merged := mergeRelations(input)

	if len(merged) != 1 {
		t.Fatalf("len = %d, want 1: %+v", len(merged), merged)
	}
	item := merged[0]
	if item.Type != "group" || item.Relation != "same_group" {
		t.Fatalf("primary relation = %s/%s, want group/same_group", item.Type, item.Relation)
	}
	if want := []string{"group", "shared_person", "counterparty"}; !reflect.DeepEqual(item.Types, want) {
		t.Fatalf("types = %v, want %v", item.Types, want)
	}
	if item.SharedPeople != 2 {
		t.Fatalf("shared_people = %d, want 2", item.SharedPeople)
	}
	if item.Amount != 500 || item.Count != 3 || item.Direction != "outbound" {
		t.Fatalf("payment details = %+v", item)
	}
}

func TestMergeRelationsKeepsDistinctCompanies(t *testing.T) {
	input := []CompanyRelation{
		{CompanyID: "b", CompanyName: "Bravo", Type: "counterparty", Relation: "payment", Direction: "inbound", Amount: 10},
		{CompanyID: "a", CompanyName: "Alpha", Type: "group", Relation: "same_group"},
	}

	merged := mergeRelations(input)

	if len(merged) != 2 {
		t.Fatalf("len = %d, want 2", len(merged))
	}
	if merged[0].CompanyID != "a" || merged[1].CompanyID != "b" {
		t.Fatalf("order = %s,%s, want a,b", merged[0].CompanyID, merged[1].CompanyID)
	}
}

func TestMergeRelationsAddsTypesForSingleEntry(t *testing.T) {
	merged := mergeRelations([]CompanyRelation{
		{CompanyID: "a", CompanyName: "Alpha", Type: "counterparty", Relation: "payment", Direction: "outbound"},
	})

	if len(merged) != 1 || !reflect.DeepEqual(merged[0].Types, []string{"counterparty"}) {
		t.Fatalf("unexpected merge result: %+v", merged)
	}
}

func TestMergeRelationsEmpty(t *testing.T) {
	if merged := mergeRelations(nil); len(merged) != 0 {
		t.Fatalf("len = %d, want 0", len(merged))
	}
}
