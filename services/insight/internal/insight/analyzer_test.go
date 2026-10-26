package insight

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alfa-hack/alfa-hack/services/insight/internal/agentclient"
)

func TestStubAnalyzerEmitsStructuredInsight(t *testing.T) {
	request := agentclient.AnalysisRequest{
		RequestID: "job-1",
		Company:   agentclient.Company{ID: "magnit", Name: "Магнит"},
		Event:     agentclient.EventContext{ID: "news-1", Title: "Магнит расширяет логистику"},
		Evidence: []agentclient.Evidence{
			{SourceID: "news-1", Title: "Магнит расширяет логистику", URL: "https://example.com/news-1"},
			{SourceID: "rag:ALFA-Macro_Insights.pdf:1:0", Title: "Alfa Macro", URL: "rag://macro"},
		},
	}

	var raw []byte
	err := NewStubAnalyzer(0).Analyze(context.Background(), request, func(event agentclient.Event) error {
		if event.Type == "result" {
			raw = append([]byte(nil), event.Data...)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if raw == nil {
		t.Fatal("no result event")
	}

	var payload struct {
		Insight Insight `json:"insight"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode result: %v", err)
	}

	insight := payload.Insight
	if len(insight.Facts) == 0 {
		t.Fatal("facts are dropped")
	}
	if len(insight.Facts[0].SourceIDs) == 0 {
		t.Fatal("fact citations are dropped")
	}
	if insight.BankView == nil {
		t.Fatal("bank_view is dropped")
	}
	if insight.BankView.Used {
		t.Fatal("stub must not claim bank analytics without an alfa source")
	}
	if insight.BankPosition != "" {
		t.Fatal("legacy bank_position must stay empty when bank_view.used is false")
	}
	if insight.AIView == nil || insight.AIView.Text == "" {
		t.Fatal("ai_view is dropped")
	}
	if insight.Disclaimer == "" {
		t.Fatal("disclaimer is dropped")
	}
}
