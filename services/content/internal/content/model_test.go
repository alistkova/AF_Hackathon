package content

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewsItemJSONShape(t *testing.T) {
	item := NewsItem{
		NewsID:        "news_2026_09_11_001",
		Title:         "Банк России сохранил ключевую ставку",
		PublishedAt:   time.Date(2026, 9, 11, 13, 30, 0, 0, time.UTC),
		SourceName:    "РБК",
		SourceURL:     "https://www.rbc.ru/finances/example",
		FullText:      "Полный текст новости",
		Tags:          []string{"макроэкономика", "ключевая ставка"},
		TargetSectors: []string{"Финансы", "Строительство"},
	}

	body, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	required := []string{
		"news_id",
		"title",
		"published_at",
		"source_name",
		"source_url",
		"full_text",
		"tags",
		"target_sectors",
	}
	for _, key := range required {
		if _, ok := payload[key]; !ok {
			t.Fatalf("missing field %q in %s", key, body)
		}
	}
}
