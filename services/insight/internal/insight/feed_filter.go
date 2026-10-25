package insight

import (
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/alfa-hack/alfa-hack/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

const maxFeedQueryLength = 200

const (
	feedSortRelevance = "relevance"
	feedSortDate      = "date"
)

type FeedFilter struct {
	Sources []string
	Tags    []string
	Sectors []string
	Query   string
	From    *time.Time
	To      *time.Time
}

type FeedFacet struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

type FeedFacets struct {
	Sources []FeedFacet `json:"sources"`
	Tags    []FeedFacet `json:"tags"`
	Sectors []FeedFacet `json:"sectors"`
}

func feedFilter(c *gin.Context) (FeedFilter, bool) {
	filter := FeedFilter{
		Sources: queryValues(c, "source"),
		Tags:    queryValues(c, "tag"),
		Sectors: queryValues(c, "sector"),
		Query:   strings.TrimSpace(c.Query("q")),
	}
	if utf8.RuneCountInString(filter.Query) > maxFeedQueryLength {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Поисковый запрос слишком длинный")
		return FeedFilter{}, false
	}

	from, ok := feedDate(c, "from", false)
	if !ok {
		return FeedFilter{}, false
	}
	to, ok := feedDate(c, "to", true)
	if !ok {
		return FeedFilter{}, false
	}
	if from != nil && to != nil && from.After(*to) {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Начало периода позже его окончания")
		return FeedFilter{}, false
	}

	filter.From = from
	filter.To = to
	return filter, true
}

func feedDate(c *gin.Context, name string, endOfDay bool) (*time.Time, bool) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return nil, true
	}

	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		parsed, err := time.Parse(layout, raw)
		if err != nil {
			continue
		}
		if layout == "2006-01-02" && endOfDay {
			parsed = parsed.Add(24*time.Hour - time.Nanosecond)
		}
		return &parsed, true
	}

	httpx.Error(c, http.StatusBadRequest, "invalid_request", "Параметр "+name+" должен быть датой в формате 2006-01-02 или RFC3339")
	return nil, false
}

func feedSortParam(c *gin.Context) (string, bool) {
	raw := strings.ToLower(strings.TrimSpace(c.Query("sort")))
	switch raw {
	case "", feedSortRelevance:
		return feedSortRelevance, true
	case feedSortDate:
		return feedSortDate, true
	default:
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Параметр sort должен быть relevance или date")
		return "", false
	}
}

func SortFeed(items []FeedItem, sortBy string) []FeedItem {
	if sortBy != feedSortDate {
		return items
	}

	sorted := make([]FeedItem, len(items))
	copy(sorted, items)
	sort.SliceStable(sorted, func(i, j int) bool {
		left := publishedTime(sorted[i].PublishedAt)
		right := publishedTime(sorted[j].PublishedAt)
		if !left.Equal(right) {
			if left.IsZero() {
				return false
			}
			if right.IsZero() {
				return true
			}
			return left.After(right)
		}
		if sorted[i].Relevance != sorted[j].Relevance {
			return sorted[i].Relevance > sorted[j].Relevance
		}
		return sorted[i].EventID < sorted[j].EventID
	})
	return sorted
}

func publishedTime(raw string) time.Time {
	published, _ := time.Parse(time.RFC3339, raw)
	return published
}

func queryValues(c *gin.Context, name string) []string {
	raw := c.QueryArray(name)
	values := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, item := range raw {
		for _, value := range strings.Split(item, ",") {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			key := normalizeText(value)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			values = append(values, value)
		}
	}
	return values
}

func FilterFeed(items []FeedItem, filter FeedFilter) []FeedItem {
	if filter.empty() {
		return items
	}

	filtered := make([]FeedItem, 0, len(items))
	for _, item := range items {
		if !matchesSelected(item.Source, filter.Sources) {
			continue
		}
		if !matchesSelectedSet(item.Tags, filter.Tags) {
			continue
		}
		if !matchesSelectedSet(item.TargetSectors, filter.Sectors) {
			continue
		}
		if filter.Query != "" && !matchesQuery(item, filter.Query) {
			continue
		}
		if !matchesDate(item.PublishedAt, filter.From, filter.To) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func BuildFeedFacets(items []FeedItem) FeedFacets {
	sourceCounts := make(map[string]*FeedFacet)
	tagCounts := make(map[string]*FeedFacet)
	sectorCounts := make(map[string]*FeedFacet)

	for _, item := range items {
		countFacet(sourceCounts, item.Source)
		for _, tag := range item.Tags {
			countFacet(tagCounts, tag)
		}
		for _, sector := range item.TargetSectors {
			countFacet(sectorCounts, sector)
		}
	}

	return FeedFacets{
		Sources: sortedFacets(sourceCounts),
		Tags:    sortedFacets(tagCounts),
		Sectors: sortedFacets(sectorCounts),
	}
}

func (filter FeedFilter) empty() bool {
	return len(filter.Sources) == 0 &&
		len(filter.Tags) == 0 &&
		len(filter.Sectors) == 0 &&
		filter.Query == "" &&
		filter.From == nil &&
		filter.To == nil
}

func matchesSelected(value string, selected []string) bool {
	if len(selected) == 0 {
		return true
	}
	normalized := normalizeText(value)
	for _, candidate := range selected {
		if normalized == normalizeText(candidate) {
			return true
		}
	}
	return false
}

func matchesSelectedSet(values, selected []string) bool {
	if len(selected) == 0 {
		return true
	}
	for _, value := range values {
		if matchesSelected(value, selected) {
			return true
		}
	}
	return false
}

func matchesQuery(item FeedItem, query string) bool {
	haystack := normalizeText(strings.Join([]string{
		item.Title,
		item.Summary,
		item.Source,
		strings.Join(item.Tags, " "),
		strings.Join(item.TargetSectors, " "),
	}, " "))

	words := strings.FieldsFunc(normalizeText(query), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	if len(words) == 0 {
		return true
	}
	for _, word := range words {
		if !containsTerm(haystack, stem(word), true) {
			return false
		}
	}
	return true
}

func matchesDate(raw string, from, to *time.Time) bool {
	if from == nil && to == nil {
		return true
	}
	published, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return false
	}
	if from != nil && published.Before(*from) {
		return false
	}
	if to != nil && published.After(*to) {
		return false
	}
	return true
}

func countFacet(counts map[string]*FeedFacet, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	key := normalizeText(value)
	if facet, exists := counts[key]; exists {
		facet.Count++
		return
	}
	counts[key] = &FeedFacet{Value: value, Count: 1}
}

func sortedFacets(counts map[string]*FeedFacet) []FeedFacet {
	facets := make([]FeedFacet, 0, len(counts))
	for _, facet := range counts {
		facets = append(facets, *facet)
	}
	sort.SliceStable(facets, func(i, j int) bool {
		if facets[i].Count != facets[j].Count {
			return facets[i].Count > facets[j].Count
		}
		return normalizeText(facets[i].Value) < normalizeText(facets[j].Value)
	})
	return facets
}
