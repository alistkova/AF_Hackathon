package insight

import (
	"reflect"
	"testing"
	"time"
)

func TestFilterFeedAppliesDimensions(t *testing.T) {
	t.Parallel()

	items := filterFixtures()
	from := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 10, 23, 59, 59, 0, time.UTC)

	tests := []struct {
		name   string
		filter FeedFilter
		want   []string
	}{
		{
			name:   "no filters",
			filter: FeedFilter{},
			want:   []string{"news-a", "news-b", "news-c"},
		},
		{
			name:   "single source",
			filter: FeedFilter{Sources: []string{"Ведомости"}},
			want:   []string{"news-a", "news-c"},
		},
		{
			name:   "multiple sources are OR",
			filter: FeedFilter{Sources: []string{"Ведомости", "РБК"}},
			want:   []string{"news-a", "news-b", "news-c"},
		},
		{
			name:   "tag",
			filter: FeedFilter{Tags: []string{"ритейл"}},
			want:   []string{"news-a"},
		},
		{
			name:   "sector",
			filter: FeedFilter{Sectors: []string{"Финансы"}},
			want:   []string{"news-b"},
		},
		{
			name:   "dimensions are AND",
			filter: FeedFilter{Sources: []string{"Ведомости"}, Tags: []string{"металлургия"}},
			want:   []string{"news-c"},
		},
		{
			name:   "query matches summary case insensitive",
			filter: FeedFilter{Query: "ЛОГИСТИКА"},
			want:   []string{"news-a"},
		},
		{
			name:   "date range includes the end date",
			filter: FeedFilter{From: &from, To: &to},
			want:   []string{"news-a", "news-b"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			filtered := FilterFeed(items, testCase.filter)
			got := make([]string, 0, len(filtered))
			for _, item := range filtered {
				got = append(got, item.EventID)
			}
			if !reflect.DeepEqual(got, testCase.want) {
				t.Fatalf("got %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestSortFeedByDate(t *testing.T) {
	t.Parallel()

	sorted := SortFeed(filterFixtures(), feedSortDate)
	got := make([]string, 0, len(sorted))
	for _, item := range sorted {
		got = append(got, item.EventID)
	}

	want := []string{"news-b", "news-a", "news-c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestFilterFeedByDateOnlyUpperBoundIncludesWholeDay(t *testing.T) {
	t.Parallel()

	to, ok := parseTestDate("2026-09-10")
	if !ok {
		t.Fatal("could not parse test date")
	}
	to = to.Add(24*time.Hour - time.Nanosecond)

	item := filterFixture(
		"news-late",
		"Ведомости",
		[]string{"ритейл"},
		[]string{"Торговля"},
		"2026-09-10T23:30:00Z",
		"Поздняя новость",
		"",
	)

	filtered := FilterFeed([]FeedItem{item}, FeedFilter{To: &to})
	if len(filtered) != 1 {
		t.Fatalf("got %d items, want 1", len(filtered))
	}
}

func TestBuildFeedFacetsCountsUnfilteredItems(t *testing.T) {
	t.Parallel()

	facets := BuildFeedFacets(filterFixtures())

	wantSources := []FeedFacet{
		{Value: "Ведомости", Count: 2},
		{Value: "РБК", Count: 1},
	}
	if !reflect.DeepEqual(facets.Sources, wantSources) {
		t.Fatalf("sources are %+v, want %+v", facets.Sources, wantSources)
	}

	wantTags := []FeedFacet{
		{Value: "Магнит", Count: 1},
		{Value: "макроэкономика", Count: 1},
		{Value: "металлургия", Count: 1},
		{Value: "ритейл", Count: 1},
	}
	if !reflect.DeepEqual(facets.Tags, wantTags) {
		t.Fatalf("tags are %+v, want %+v", facets.Tags, wantTags)
	}

	wantSectors := []FeedFacet{
		{Value: "Металлургия", Count: 1},
		{Value: "Торговля", Count: 1},
		{Value: "Финансы", Count: 1},
	}
	if !reflect.DeepEqual(facets.Sectors, wantSectors) {
		t.Fatalf("sectors are %+v, want %+v", facets.Sectors, wantSectors)
	}
}

func filterFixtures() []FeedItem {
	return []FeedItem{
		filterFixture(
			"news-a",
			"Ведомости",
			[]string{"ритейл", "Магнит"},
			[]string{"Торговля"},
			"2026-09-09T11:40:00Z",
			"Магнит расширяет логистику",
			"Ритейлер инвестирует в распределительные центры.",
		),
		filterFixture(
			"news-b",
			"РБК",
			[]string{"макроэкономика"},
			[]string{"Финансы"},
			"2026-09-10T13:30:00Z",
			"Банк России сохранил ставку",
			"Регулятор сохранил ключевую ставку.",
		),
		filterFixture(
			"news-c",
			"Ведомости",
			[]string{"металлургия"},
			[]string{"Металлургия"},
			"2026-09-08T15:05:00Z",
			"Северсталь ждет роста спроса",
			"Компания ожидает восстановления спроса на прокат.",
		),
	}
}

func filterFixture(
	eventID, source string,
	tags, sectors []string,
	publishedAt, title, summary string,
) FeedItem {
	return FeedItem{
		EventID:       eventID,
		CompanyID:     "magnit",
		Title:         title,
		Summary:       summary,
		Source:        source,
		URL:           "https://example.com/" + eventID,
		PublishedAt:   publishedAt,
		Relevance:     0.5,
		Reasons:       []string{"Упоминание компании"},
		Tags:          tags,
		TargetSectors: sectors,
	}
}

func parseTestDate(value string) (time.Time, bool) {
	parsed, err := time.Parse("2006-01-02", value)
	return parsed, err == nil
}
