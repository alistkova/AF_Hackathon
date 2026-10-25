package insight

import (
	"strings"
	"testing"
	"time"
)

func TestProfileForMapsIndustriesToNewsSectors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		industry string
		sector   string
	}{
		{industry: "Ритейл", sector: "Торговля"},
		{industry: "Банки", sector: "Финансы"},
		{industry: "Металлургия", sector: "Металлургия"},
		{industry: "Нефтегаз", sector: "Энергетика"},
		{industry: "Технологии", sector: "IT"},
	}

	for _, testCase := range cases {
		t.Run(testCase.industry, func(t *testing.T) {
			t.Parallel()

			profile := profileFor(testCase.industry)
			reason, ok := matchedSector([]string{testCase.sector}, profile.sectors)
			if !ok {
				t.Fatalf("profile for %q does not match sector %q", testCase.industry, testCase.sector)
			}
			if !strings.Contains(reason, testCase.sector) {
				t.Fatalf("reason %q does not name sector %q", reason, testCase.sector)
			}
		})
	}
}

func TestRankFeedPutsCompanyNewsFirstAndDropsNoise(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	company := Company{ID: "magnit", Name: "Магнит", Industry: "Ритейл", Region: "Краснодар"}
	news := []NewsItem{
		{
			NewsID:        "news-macro",
			Title:         "Банк России сохранил ключевую ставку",
			FullText:      "Совет директоров Банка России сохранил ставку на прежнем уровне.",
			PublishedAt:   now.Add(-24 * time.Hour),
			Tags:          []string{"макроэкономика", "ключевая ставка"},
			TargetSectors: []string{"Финансы"},
		},
		{
			NewsID:        "news-magnit",
			Title:         "Магнит увеличивает инвестиции в логистику",
			FullText:      "Ритейлер расширяет распределительные центры в Краснодарском крае.",
			PublishedAt:   now.Add(-48 * time.Hour),
			Tags:          []string{"ритейл", "Магнит"},
			TargetSectors: []string{"Торговля", "Логистика"},
		},
		{
			NewsID:        "news-china",
			Title:         "Китай ужесточил таможенные процедуры",
			FullText:      "Сроки оформления грузов выросли из-за дополнительных проверок.",
			PublishedAt:   now.Add(-72 * time.Hour),
			Tags:          []string{"ВЭД", "таможня"},
			TargetSectors: []string{"Машиностроение"},
		},
	}

	items := RankFeed(company, news, now)
	if len(items) != 1 {
		t.Fatalf("got %d feed items, want 1: %+v", len(items), items)
	}

	item := items[0]
	if item.EventID != "news-magnit" {
		t.Fatalf("first event is %q, want news-magnit", item.EventID)
	}
	if item.CompanyID != "magnit" {
		t.Fatalf("company_id is %q, want magnit", item.CompanyID)
	}
	if item.Relevance <= 0 || item.Relevance > 1 {
		t.Fatalf("relevance %v is out of range", item.Relevance)
	}
	if !containsString(item.Reasons, "Упоминание компании") {
		t.Fatalf("reasons %v do not mention the company match", item.Reasons)
	}
	if !containsString(item.Reasons, "Отрасль: Торговля") {
		t.Fatalf("reasons %v do not mention the sector match", item.Reasons)
	}
}

func TestRankFeedIncludesSectorOnlyNews(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	company := Company{ID: "severstal", Name: "Северсталь", Industry: "Металлургия"}
	news := []NewsItem{{
		NewsID:        "news-steel",
		Title:         "Отрасль ждет роста спроса на прокат",
		FullText:      "Менеджмент ожидает улучшения загрузки мощностей.",
		PublishedAt:   now.Add(-5 * 24 * time.Hour),
		TargetSectors: []string{"Металлургия"},
	}}

	items := RankFeed(company, news, now)
	if len(items) != 1 {
		t.Fatalf("got %d feed items, want 1", len(items))
	}
	if items[0].Relevance <= 0 {
		t.Fatalf("sector-only item has relevance %v", items[0].Relevance)
	}
}

func TestRankFeedFallsBackToLatestGeneralNews(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	company := Company{ID: "yandex", Name: "Яндекс", Industry: "Технологии"}
	news := []NewsItem{
		{
			NewsID:        "news-old",
			Title:         "Металлурги обсуждают экспорт",
			FullText:      "Компании ждут восстановления спроса.",
			PublishedAt:   now.Add(-10 * 24 * time.Hour),
			TargetSectors: []string{"Металлургия"},
		},
		{
			NewsID:        "news-new",
			Title:         "Ритейлеры открыли новые магазины",
			FullText:      "Сеть расширяет присутствие в регионах.",
			PublishedAt:   now.Add(-2 * 24 * time.Hour),
			TargetSectors: []string{"Торговля"},
		},
	}

	items := RankFeed(company, news, now)
	if len(items) != 2 {
		t.Fatalf("got %d fallback items, want 2", len(items))
	}
	if items[0].EventID != "news-new" {
		t.Fatalf("fallback is not ordered by date: first is %q", items[0].EventID)
	}
	if items[0].Relevance != 0.1 {
		t.Fatalf("fallback relevance is %v, want 0.1", items[0].Relevance)
	}
	if !containsString(items[0].Reasons, "Общая повестка") {
		t.Fatalf("fallback reasons are %v", items[0].Reasons)
	}
}

func TestRankFeedMatchesRegionWithInflection(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	company := Company{ID: "local", Name: "Местная компания", Region: "Краснодар"}
	news := []NewsItem{{
		NewsID:      "news-region",
		Title:       "В Краснодаре открыли новый распределительный центр",
		FullText:    "Проект запущен при поддержке региональных властей.",
		PublishedAt: now.Add(-24 * time.Hour),
	}}

	items := RankFeed(company, news, now)
	if len(items) != 1 {
		t.Fatalf("got %d feed items, want 1", len(items))
	}
	if !containsString(items[0].Reasons, "Регион: Краснодар") {
		t.Fatalf("reasons %v do not mention the region", items[0].Reasons)
	}
}

func TestSummarizeTruncatesOnWords(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("слово ", 100)
	summary := summarize(long, 40)

	if !strings.HasSuffix(summary, "…") {
		t.Fatalf("summary %q has no ellipsis", summary)
	}
	if strings.Contains(summary, "\n") {
		t.Fatalf("summary %q contains a newline", summary)
	}
	if len([]rune(summary)) > 41 {
		t.Fatalf("summary has %d runes, want at most 41", len([]rune(summary)))
	}
	if strings.HasSuffix(strings.TrimSuffix(summary, "…"), " ") {
		t.Fatalf("summary %q ends with a space", summary)
	}
}

func TestRankFeedSkipsItemsWithoutIdentity(t *testing.T) {
	t.Parallel()

	items := RankFeed(Company{ID: "magnit"}, []NewsItem{{Title: "Без идентификатора"}}, time.Now())
	if len(items) != 0 {
		t.Fatalf("got %d items for news without id, want 0", len(items))
	}
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
